package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreakerOpensAfterMaxFailures(t *testing.T) {
	b := NewBreaker(3, time.Minute)
	failing := func() error { return errors.New("boom") }

	for i := 0; i < 3; i++ {
		if err := b.Do(failing); err == nil {
			t.Fatalf("attempt %d: expected failure", i)
		}
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("expected StateOpen after 3 failures, got %v", got)
	}
	// While open, Do fast-fails without calling fn.
	called := false
	err := b.Do(func() error { called = true; return nil })
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
	if called {
		t.Fatal("fn should not be called while circuit is open")
	}
}

func TestBreakerHalfOpenRecovers(t *testing.T) {
	b := NewBreaker(1, 10*time.Millisecond)
	_ = b.Do(func() error { return errors.New("boom") }) // opens
	if b.State() != StateOpen {
		t.Fatal("expected open")
	}
	time.Sleep(15 * time.Millisecond)
	if b.State() != StateHalfOpen {
		t.Fatal("expected half-open after open window")
	}
	// A success in half-open closes the circuit.
	if err := b.Do(func() error { return nil }); err != nil {
		t.Fatalf("half-open success: %v", err)
	}
	if b.State() != StateClosed {
		t.Fatal("expected closed after successful trial")
	}
}

func TestRetryStopsOnSuccess(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 5, BaseBackoff: time.Millisecond}, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryExhausts(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, BaseBackoff: time.Millisecond}, func() error {
		calls++
		return errors.New("always")
	})
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryPermanentNotRetried(t *testing.T) {
	calls := 0
	sentinel := errors.New("nope")
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 5, BaseBackoff: time.Millisecond}, func() error {
		calls++
		return Permanent(sentinel)
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("permanent error should not retry, got %d calls", calls)
	}
}
