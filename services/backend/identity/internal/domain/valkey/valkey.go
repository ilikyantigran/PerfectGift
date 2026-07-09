// Package valkey owns Identity's ephemeral state in Valkey: login sessions /
// refresh tokens (with instant revocation via key deletion) and per-key login
// rate-limit counters.
package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/model"

	"github.com/valkey-io/valkey-go"
)

// Store is the Valkey-backed implementation of the app's Sessions and
// RateLimiter interfaces.
type Store struct {
	client        valkey.Client
	rlMax         int
	rlWindow      time.Duration
}

// NewStore connects to Valkey. rlMax/rlWindow configure the login rate limiter.
func NewStore(address string, rlMax int, rlWindow time.Duration) (*Store, error) {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{address}})
	if err != nil {
		return nil, err
	}
	return &Store{client: client, rlMax: rlMax, rlWindow: rlWindow}, nil
}

func (s *Store) Close() { s.client.Close() }

func sessionKey(id string) string { return "identity:session:" + id }
func rateKey(key string) string   { return "identity:ratelimit:" + key }

// storedSession is the JSON shape persisted per session.
type storedSession struct {
	UserID      string    `json:"user_id"`
	RefreshHash string    `json:"refresh_hash"`
	Device      string    `json:"device"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Create stores a new session with a TTL matching its expiry.
func (s *Store) Create(ctx context.Context, sess model.Session) error {
	return s.put(ctx, sess)
}

// Update overwrites an existing session (used to rotate the refresh hash).
func (s *Store) Update(ctx context.Context, sess model.Session) error {
	return s.put(ctx, sess)
}

func (s *Store) put(ctx context.Context, sess model.Session) error {
	body, err := json.Marshal(storedSession{
		UserID:      sess.UserID,
		RefreshHash: sess.RefreshHash,
		Device:      sess.Device,
		IssuedAt:    sess.IssuedAt,
		ExpiresAt:   sess.ExpiresAt,
	})
	if err != nil {
		return err
	}
	ttl := time.Until(sess.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}
	cmd := s.client.B().Set().Key(sessionKey(sess.ID)).Value(string(body)).
		ExSeconds(int64(ttl.Seconds())).Build()
	return s.client.Do(ctx, cmd).Error()
}

// Get returns a session by id; ok=false when it is absent (or has expired).
func (s *Store) Get(ctx context.Context, id string) (model.Session, bool, error) {
	res, err := s.client.Do(ctx, s.client.B().Get().Key(sessionKey(id)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return model.Session{}, false, nil
		}
		return model.Session{}, false, err
	}
	var st storedSession
	if err := json.Unmarshal([]byte(res), &st); err != nil {
		return model.Session{}, false, err
	}
	return model.Session{
		ID:          id,
		UserID:      st.UserID,
		RefreshHash: st.RefreshHash,
		Device:      st.Device,
		IssuedAt:    st.IssuedAt,
		ExpiresAt:   st.ExpiresAt,
	}, true, nil
}

// Delete removes a session, instantly revoking its refresh token.
func (s *Store) Delete(ctx context.Context, id string) error {
	return s.client.Do(ctx, s.client.B().Del().Key(sessionKey(id)).Build()).Error()
}

// Allow increments the counter for key within its window and reports whether the
// attempt is within the configured limit. The first attempt sets the TTL.
func (s *Store) Allow(ctx context.Context, key string) (bool, error) {
	k := rateKey(key)
	n, err := s.client.Do(ctx, s.client.B().Incr().Key(k).Build()).AsInt64()
	if err != nil {
		return false, err
	}
	if n == 1 {
		exp := s.client.B().Expire().Key(k).Seconds(int64(s.rlWindow.Seconds())).Build()
		if err := s.client.Do(ctx, exp).Error(); err != nil {
			return false, err
		}
	}
	return n <= int64(s.rlMax), nil
}

// Ping verifies connectivity (used at startup).
func (s *Store) Ping(ctx context.Context) error {
	if err := s.client.Do(ctx, s.client.B().Ping().Build()).Error(); err != nil {
		return fmt.Errorf("valkey ping: %w", err)
	}
	return nil
}
