// Package valkey is the concrete domain.Cache backed by Valkey: job status
// (cheap poll target), idempotency keys (short-circuit duplicate submits), and
// the LLM response cache (the main cost lever). The test suite uses the in-memory
// store instead, so this package needs no live Valkey to build.
package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
)

// Store implements domain.Cache over a Valkey client.
type Store struct {
	client valkey.Client
}

// New connects to Valkey. Call Close on shutdown.
func New(address string) (*Store, error) {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{address}})
	if err != nil {
		return nil, err
	}
	return &Store{client: client}, nil
}

// Close releases the client.
func (s *Store) Close() { s.client.Close() }

func statusKey(id string) string { return "surprise:status:" + id }
func idemKey(k string) string    { return "surprise:idem:" + k }
func llmKey(h string) string     { return "surprise:llmcache:" + h }

func (s *Store) SetStatus(ctx context.Context, requestID string, info domain.StatusInfo, ttl time.Duration) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return s.client.Do(ctx, s.client.B().Set().Key(statusKey(requestID)).Value(string(data)).ExSeconds(secs(ttl)).Build()).Error()
}

func (s *Store) GetStatus(ctx context.Context, requestID string) (domain.StatusInfo, error) {
	raw, err := s.client.Do(ctx, s.client.B().Get().Key(statusKey(requestID)).Build()).ToString()
	if err != nil {
		if isNil(err) {
			return domain.StatusInfo{}, domain.ErrNotFound
		}
		return domain.StatusInfo{}, err
	}
	var info domain.StatusInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return domain.StatusInfo{}, err
	}
	return info, nil
}

func (s *Store) SetIdempotencyIfAbsent(ctx context.Context, key, requestID string, ttl time.Duration) (bool, string, error) {
	// SET key requestID NX EX ttl -> "OK" if stored, nil if it already existed.
	err := s.client.Do(ctx, s.client.B().Set().Key(idemKey(key)).Value(requestID).Nx().ExSeconds(secs(ttl)).Build()).Error()
	if err == nil {
		return true, requestID, nil
	}
	if !isNil(err) {
		return false, "", err
	}
	existing, gerr := s.client.Do(ctx, s.client.B().Get().Key(idemKey(key)).Build()).ToString()
	if gerr != nil {
		return false, "", gerr
	}
	return false, existing, nil
}

func (s *Store) GetLLMCache(ctx context.Context, hash string) ([]domain.Idea, error) {
	raw, err := s.client.Do(ctx, s.client.B().Get().Key(llmKey(hash)).Build()).ToString()
	if err != nil {
		if isNil(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	var ideas []domain.Idea
	if err := json.Unmarshal([]byte(raw), &ideas); err != nil {
		return nil, err
	}
	return ideas, nil
}

func (s *Store) SetLLMCache(ctx context.Context, hash string, ideas []domain.Idea, ttl time.Duration) error {
	data, err := json.Marshal(ideas)
	if err != nil {
		return err
	}
	return s.client.Do(ctx, s.client.B().Set().Key(llmKey(hash)).Value(string(data)).ExSeconds(secs(ttl)).Build()).Error()
}

func secs(d time.Duration) int64 {
	s := int64(d.Seconds())
	if s < 1 {
		s = 1
	}
	return s
}

func isNil(err error) bool { return errors.Is(err, valkey.Nil) }
