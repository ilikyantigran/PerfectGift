package notify

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// clock is a mutable time source for deterministic retry/lease/crash tests.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)} }
func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func seedPending(t *testing.T, s *fakeStore, userID string, now time.Time) Outbox {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"title": "x"})
	inserted, err := s.EnqueueOutbox(context.Background(), Outbox{
		UserID: userID, Type: TypeIdeasReady, Payload: payload,
		DedupeKey: "k-" + userID, Status: StatusPending, NextAttemptAt: now, CreatedAt: now,
	})
	if err != nil || !inserted {
		t.Fatalf("seed: inserted=%v err=%v", inserted, err)
	}
	return s.getByKey("k-" + userID)
}

func addDevice(s *fakeStore, userID, token string, plat Platform) {
	_, _ = s.UpsertDevice(context.Background(), Device{
		UserID: userID, Platform: plat, PushToken: token,
	}, time.Now())
}

func testCfg() DispatcherConfig {
	return DispatcherConfig{
		Interval: time.Second, Lease: 30 * time.Second,
		Batch: 100, MaxAttempts: 3, BaseBackoff: 2 * time.Second,
	}
}

func TestDispatch_HappyPath_PushesAndMarksSent(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	addDevice(store, "user-1", "tok-ios", PlatformIOS)
	addDevice(store, "user-1", "tok-and", PlatformAndroid)
	seedPending(t, store, "user-1", clk.now())

	d := NewDispatcher(store, pusher, testCfg(), clk.now)
	n, err := d.RunOnce(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("RunOnce n=%d err=%v", n, err)
	}
	if pusher.count() != 2 {
		t.Errorf("pushes = %d, want 2 (both devices)", pusher.count())
	}
	o := store.getByKey("k-user-1")
	if o.Status != StatusSent {
		t.Errorf("status = %q, want sent", o.Status)
	}
	if o.SentAt == nil {
		t.Error("sent_at not set")
	}
}

func TestDispatch_NoDevices_MarksSent(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	seedPending(t, store, "ghost", clk.now())

	d := NewDispatcher(store, pusher, testCfg(), clk.now)
	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pusher.count() != 0 {
		t.Errorf("pushes = %d, want 0", pusher.count())
	}
	if o := store.getByKey("k-ghost"); o.Status != StatusSent {
		t.Errorf("status = %q, want sent (nothing to deliver is terminal)", o.Status)
	}
}

func TestDispatch_TransientFailure_ReschedulesWithBackoff(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	pusher.failFor["tok"] = errors.New("provider 503")
	addDevice(store, "user-1", "tok", PlatformIOS)
	seedPending(t, store, "user-1", clk.now())

	d := NewDispatcher(store, pusher, testCfg(), clk.now)
	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	o := store.getByKey("k-user-1")
	if o.Status != StatusPending {
		t.Errorf("status = %q, want pending (retry)", o.Status)
	}
	if o.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", o.Attempts)
	}
	wantNext := clk.now().Add(2 * time.Second) // base backoff at attempt 1
	if !o.NextAttemptAt.Equal(wantNext) {
		t.Errorf("next_attempt_at = %v, want %v", o.NextAttemptAt, wantNext)
	}
}

func TestDispatch_ExhaustsAttempts_MarksFailed(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	pusher.failFor["tok"] = errors.New("provider down")
	addDevice(store, "user-1", "tok", PlatformIOS)
	seedPending(t, store, "user-1", clk.now())

	d := NewDispatcher(store, pusher, testCfg(), clk.now) // MaxAttempts=3
	// Each sweep: claim (needs due row), fail, reschedule. Advance past the lease
	// and the backoff each round so the row is due again.
	for i := 0; i < 3; i++ {
		if _, err := d.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		clk.advance(time.Hour)
	}
	if o := store.getByKey("k-user-1"); o.Status != StatusFailed {
		t.Errorf("status = %q, want failed after exhausting attempts", o.Status)
	}
}

func TestDispatch_DeadToken_DeactivatesDevice(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	pusher.failFor["dead"] = ErrInvalidToken
	addDevice(store, "user-1", "dead", PlatformIOS)
	addDevice(store, "user-1", "live", PlatformAndroid)
	seedPending(t, store, "user-1", clk.now())

	d := NewDispatcher(store, pusher, testCfg(), clk.now)
	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.device("dead").Active {
		t.Error("dead-token device should be deactivated")
	}
	if !store.device("live").Active {
		t.Error("live device should stay active")
	}
	// A dead token is terminal, not a transient failure — the row is done.
	if o := store.getByKey("k-user-1"); o.Status != StatusSent {
		t.Errorf("status = %q, want sent", o.Status)
	}
}

// no double-send: two dispatchers sweeping the same single pending row must
// deliver it exactly once, thanks to the atomic lease in ClaimPending.
func TestDispatch_ConcurrentSweeps_NoDoubleSend(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	addDevice(store, "user-1", "tok", PlatformIOS)
	seedPending(t, store, "user-1", clk.now())

	d := NewDispatcher(store, pusher, testCfg(), clk.now)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.RunOnce(context.Background())
		}()
	}
	wg.Wait()

	if got := pusher.countFor("tok"); got != 1 {
		t.Errorf("pushes to tok = %d, want exactly 1 (atomic claim)", got)
	}
	if o := store.getByKey("k-user-1"); o.Status != StatusSent {
		t.Errorf("status = %q, want sent", o.Status)
	}
}

// never lost: if a worker claims a row and crashes before finalizing it, the
// lease expires and a later sweep re-claims and delivers it.
func TestDispatch_CrashBeforeFinalize_RowRecoveredAfterLease(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	addDevice(store, "user-1", "tok", PlatformIOS)
	seedPending(t, store, "user-1", clk.now())

	// Simulate a crash: claim the row (leasing it) but never finalize.
	claimed, err := store.ClaimPending(context.Background(), clk.now(), 30*time.Second, 10)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: n=%d err=%v", len(claimed), err)
	}

	d := NewDispatcher(store, pusher, testCfg(), clk.now)

	// Before the lease expires, the row is invisible → not re-sent.
	if n, _ := d.RunOnce(context.Background()); n != 0 {
		t.Fatalf("expected 0 claimable during lease, got %d", n)
	}
	if pusher.count() != 0 {
		t.Fatalf("no push expected while leased, got %d", pusher.count())
	}

	// After the lease elapses, recovery: the row is delivered.
	clk.advance(31 * time.Second)
	if n, _ := d.RunOnce(context.Background()); n != 1 {
		t.Fatalf("expected 1 recovered row, got %d", n)
	}
	if pusher.count() != 1 {
		t.Errorf("pushes = %d, want 1 after recovery", pusher.count())
	}
	if o := store.getByKey("k-user-1"); o.Status != StatusSent {
		t.Errorf("status = %q, want sent", o.Status)
	}
}

func TestDispatch_ResolveError_Retries(t *testing.T) {
	clk := newClock()
	store := newFakeStore()
	pusher := newFakePusher()
	seedPending(t, store, "user-1", clk.now())
	store.resolveErr = errors.New("db down")

	d := NewDispatcher(store, pusher, testCfg(), clk.now)
	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if o := store.getByKey("k-user-1"); o.Status != StatusPending || o.Attempts != 1 {
		t.Errorf("status=%q attempts=%d, want pending/1", o.Status, o.Attempts)
	}
}
