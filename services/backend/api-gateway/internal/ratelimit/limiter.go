// Package ratelimit provides the gateway's global / per-user / per-IP rate limiting
// (SERVICE.md §5). The default implementation is an in-process fixed-window counter,
// which needs no external state and is what the unit tests exercise. In production
// the same Limiter interface is intended to be backed by Valkey counters (the only
// stateful bit of an otherwise stateless gateway); a Valkey-backed implementation can
// be dropped in behind this interface without touching the middleware. See README.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter decides whether a request keyed by `key` may proceed. Implementations must
// be safe for concurrent use.
type Limiter interface {
	Allow(key string) bool
}

// Noop never limits — used when a particular budget is unconfigured/disabled.
type Noop struct{}

func (Noop) Allow(string) bool { return true }

// Window is a fixed-window counter: at most `limit` calls per `window` per key.
// A limit of 0 disables limiting (always allow), so an unset budget is a no-op.
type Window struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count      int
	windowEnds time.Time
}

// NewWindow builds a fixed-window limiter allowing `limit` calls per `window` per key.
func NewWindow(limit int, window time.Duration) *Window {
	return &Window{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

func (w *Window) Allow(key string) bool {
	if w.limit <= 0 {
		return true
	}
	now := time.Now()

	w.mu.Lock()
	defer w.mu.Unlock()

	b, ok := w.buckets[key]
	if !ok || now.After(b.windowEnds) {
		w.buckets[key] = &bucket{count: 1, windowEnds: now.Add(w.window)}
		return true
	}
	if b.count >= w.limit {
		return false
	}
	b.count++
	return true
}
