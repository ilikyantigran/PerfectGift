// Package valkey is the hot reference-read cache. Holidays / categories / budget
// bands change rarely, so reads are cached aggressively and invalidated on the
// rare admin write. Values are stored as JSON under a "catalog:ref:" key prefix.
// The cache is best-effort: callers treat any error as a miss and fall through to
// Postgres, so a Valkey outage degrades latency, never correctness.
package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

const refPrefix = "catalog:ref:"

// Store wraps a Valkey client for JSON reference caching.
type Store struct {
	client valkey.Client
}

// NewStore connects to Valkey at address (host:port).
func NewStore(address string) (*Store, error) {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{address}})
	if err != nil {
		return nil, err
	}
	return &Store{client: client}, nil
}

// Close releases the client.
func (s *Store) Close() { s.client.Close() }

// GetJSON loads key into dst. Returns (true,nil) on hit, (false,nil) on miss.
func (s *Store) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	cmd := s.client.B().Get().Key(refPrefix + key).Build()
	res, err := s.client.Do(ctx, cmd).AsBytes()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(res, dst); err != nil {
		return false, fmt.Errorf("unmarshal cached %s: %w", key, err)
	}
	return true, nil
}

// SetJSON stores v under key with a TTL.
func (s *Store) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal %s for cache: %w", key, err)
	}
	cmd := s.client.B().Set().Key(refPrefix + key).Value(string(payload)).
		ExSeconds(int64(ttl.Seconds())).Build()
	return s.client.Do(ctx, cmd).Error()
}

// Invalidate deletes all reference cache keys (called on the rare admin write).
// Reference data is small, so a full flush of the "catalog:ref:*" namespace is
// the simplest correct invalidation.
func (s *Store) Invalidate(ctx context.Context) error {
	var cursor uint64
	for {
		cmd := s.client.B().Scan().Cursor(cursor).Match(refPrefix + "*").Count(256).Build()
		entry, err := s.client.Do(ctx, cmd).AsScanEntry()
		if err != nil {
			return err
		}
		if len(entry.Elements) > 0 {
			del := s.client.B().Del().Key(entry.Elements...).Build()
			if err := s.client.Do(ctx, del).Error(); err != nil {
				return err
			}
		}
		if entry.Cursor == 0 {
			break
		}
		cursor = entry.Cursor
	}
	return nil
}
