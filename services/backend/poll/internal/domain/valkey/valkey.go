// Package valkey owns the Poll service's ephemeral anti-abuse state: anonymous
// rate-limit counters. It implements the ports.RateLimiter port. Counters are
// fixed-window and TTL'd, so link-spam bursts are absorbed here without ever
// touching Postgres.
package valkey

import (
	"context"
	"time"

	"github.com/valkey-io/valkey-go"
)

type Store struct {
	client valkey.Client
}

func NewStore(address string) (*Store, error) {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{address}})
	if err != nil {
		return nil, err
	}
	return &Store{client: client}, nil
}

func (s *Store) Close() { s.client.Close() }

// Allow increments the counter for key and returns whether it is still within
// budget for the current window. The first increment in a window sets the TTL, so
// the window slides forward once it elapses.
func (s *Store) Allow(ctx context.Context, key string, budget int, window time.Duration) (bool, error) {
	n, err := s.client.Do(ctx, s.client.B().Incr().Key(key).Build()).AsInt64()
	if err != nil {
		return false, err
	}
	if n == 1 {
		secs := int64(window / time.Second)
		if secs < 1 {
			secs = 1
		}
		if err := s.client.Do(ctx, s.client.B().Expire().Key(key).Seconds(secs).Build()).Error(); err != nil {
			return false, err
		}
	}
	return n <= int64(budget), nil
}
