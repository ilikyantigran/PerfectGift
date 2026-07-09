package notify

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// DispatcherConfig tunes the outbox sweep and retry policy.
type DispatcherConfig struct {
	Interval    time.Duration // how often to sweep the outbox
	Lease       time.Duration // claim lease = also the crash-recovery window
	Batch       int           // max rows claimed per sweep
	MaxAttempts int           // give up (status=failed) after this many attempts
	BaseBackoff time.Duration // first retry delay; doubles each attempt
}

func (c DispatcherConfig) withDefaults() DispatcherConfig {
	if c.Interval <= 0 {
		c.Interval = time.Second
	}
	if c.Lease <= 0 {
		c.Lease = 30 * time.Second
	}
	if c.Batch <= 0 {
		c.Batch = 100
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 6
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = 2 * time.Second
	}
	return c
}

// Dispatcher drains the outbox: it claims pending rows, fans each out to the
// target user's active devices via the Pusher, and finalizes the row. It is the
// component that turns the durable outbox into actual pushes with retry/backoff.
//
// Guarantees it upholds:
//   - never lost   — a row stays pending (leased, then reclaimable) until it is
//     either delivered (MarkSent) or exhausted (MarkFailed); a crash mid-send
//     just lets the lease expire and another sweep retries it.
//   - never double-sent — the atomic lease in Store.ClaimPending means a single
//     row is worked by one sweep at a time; the unique dedupe_key upstream means
//     one row per logical event. Per-device delivery is at-least-once (a retry
//     may re-push to a device that already got it), exactly as the contract
//     specifies ("at-least-once with dedupe").
type Dispatcher struct {
	store  Store
	pusher Pusher
	cfg    DispatcherConfig
	now    func() time.Time
}

// NewDispatcher builds a Dispatcher. now defaults to time.Now.
func NewDispatcher(store Store, pusher Pusher, cfg DispatcherConfig, now func() time.Time) *Dispatcher {
	if now == nil {
		now = time.Now
	}
	return &Dispatcher{store: store, pusher: pusher, cfg: cfg.withDefaults(), now: now}
}

// Run sweeps the outbox every Interval until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := d.RunOnce(ctx); err != nil {
				slog.ErrorContext(ctx, "dispatcher sweep failed", "error", err)
			}
		}
	}
}

// RunOnce claims one batch of due rows and dispatches each. Returns how many
// rows it processed.
func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	rows, err := d.store.ClaimPending(ctx, d.now(), d.cfg.Lease, d.cfg.Batch)
	if err != nil {
		return 0, err
	}
	for _, o := range rows {
		if err := d.dispatch(ctx, o); err != nil {
			slog.ErrorContext(ctx, "dispatch failed", "id", o.ID, "error", err)
		}
	}
	return len(rows), nil
}

// dispatch fans one outbox row out to the user's active devices and finalizes
// it. A transient error on any device retries the whole row (with backoff); a
// dead token deactivates that device without failing the row.
func (d *Dispatcher) dispatch(ctx context.Context, o Outbox) error {
	devices, err := d.store.ActiveDevicesForUser(ctx, o.UserID)
	if err != nil {
		return d.retry(ctx, o) // treat resolution failure as transient
	}

	msg := PushMessage{Type: o.Type, Payload: o.Payload}
	transient := false
	for _, dev := range devices {
		switch pushErr := d.pusher.Push(ctx, dev, msg); {
		case pushErr == nil:
			// delivered
		case errors.Is(pushErr, ErrInvalidToken):
			// Terminal for this device: prune the dead token, don't fail the row.
			if derr := d.store.DeactivateDeviceByToken(ctx, dev.PushToken); derr != nil {
				slog.ErrorContext(ctx, "deactivate dead token failed", "error", derr)
			}
		default:
			transient = true
			slog.WarnContext(ctx, "transient push failure", "device", dev.ID, "error", pushErr)
		}
	}

	if transient {
		return d.retry(ctx, o)
	}
	// All devices terminal (delivered or pruned), including the zero-device case.
	return d.store.MarkSent(ctx, o.ID, d.now())
}

// retry reschedules a row with exponential backoff, or gives up once attempts
// are exhausted.
func (d *Dispatcher) retry(ctx context.Context, o Outbox) error {
	attempts := o.Attempts + 1
	if attempts >= d.cfg.MaxAttempts {
		return d.store.MarkFailed(ctx, o.ID)
	}
	return d.store.Reschedule(ctx, o.ID, attempts, d.now().Add(d.backoff(attempts)))
}

// backoff returns BaseBackoff * 2^(attempts-1), capped to avoid overflow.
func (d *Dispatcher) backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := attempts - 1
	if shift > 16 { // cap the exponent; 2^16 * base is already very long
		shift = 16
	}
	return d.cfg.BaseBackoff * time.Duration(1<<uint(shift))
}
