// Package resilience provides the retry/backoff and circuit-breaker primitives
// that wrap the Anthropic Claude call. Claude is slow, expensive, and a third
// party; its latency, rate limits, and outages must not leak into the rest of
// the system. These primitives are transport-agnostic and fully unit-tested.
package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned by a Breaker whose circuit is open (fast-fail).
var ErrOpen = errors.New("resilience: circuit breaker open")

// State is the breaker state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// Breaker is a simple failure-count circuit breaker. After maxFailures
// consecutive failures the circuit opens; after openDuration it transitions to
// half-open and allows a single trial call. A success closes it; a failure
// re-opens it.
type Breaker struct {
	maxFailures  int
	openDuration time.Duration
	now          func() time.Time

	mu       sync.Mutex
	failures int
	state    State
	openedAt time.Time
}

// NewBreaker builds a breaker. maxFailures<1 is treated as 1.
func NewBreaker(maxFailures int, openDuration time.Duration) *Breaker {
	if maxFailures < 1 {
		maxFailures = 1
	}
	return &Breaker{
		maxFailures:  maxFailures,
		openDuration: openDuration,
		now:          time.Now,
	}
}

// State returns the current state, transitioning open->half-open if the open
// window has elapsed.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refresh()
	return b.state
}

func (b *Breaker) refresh() {
	if b.state == StateOpen && b.now().Sub(b.openedAt) >= b.openDuration {
		b.state = StateHalfOpen
	}
}

// Do runs fn if the circuit permits, recording success/failure. When the circuit
// is open it returns ErrOpen without calling fn.
func (b *Breaker) Do(fn func() error) error {
	b.mu.Lock()
	b.refresh()
	if b.state == StateOpen {
		b.mu.Unlock()
		return ErrOpen
	}
	b.mu.Unlock()

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil {
		b.failures++
		if b.failures >= b.maxFailures || b.state == StateHalfOpen {
			b.state = StateOpen
			b.openedAt = b.now()
		}
		return err
	}
	b.failures = 0
	b.state = StateClosed
	return nil
}

// RetryConfig configures Retry.
type RetryConfig struct {
	MaxAttempts int           // total attempts, minimum 1
	BaseBackoff time.Duration // backoff = BaseBackoff * 2^(attempt-1)
}

// Retry runs fn up to cfg.MaxAttempts times with exponential backoff, aborting
// early if ctx is cancelled or fn returns a non-retryable error (via Permanent).
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	attempts := cfg.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err = fn()
		if err == nil {
			return nil
		}
		var perm *permanentError
		if errors.As(err, &perm) {
			return perm.err
		}
		if i == attempts-1 {
			break
		}
		backoff := cfg.BaseBackoff << i
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return err
}

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps an error so Retry will not retry it.
func Permanent(err error) error { return &permanentError{err: err} }
