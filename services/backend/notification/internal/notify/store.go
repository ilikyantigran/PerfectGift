package notify

import (
	"context"
	"time"
)

// Store is the persistence boundary for the notification service. Postgres is
// the production implementation (internal/domain/postgres); tests use an
// in-memory fake. It owns two things: the devices table and the outbox.
//
// The outbox methods are written so the delivery guarantees hold:
//   - EnqueueOutbox is idempotent on dedupe_key  → an event redelivered by the
//     bus never creates a second notification (never double-sent at the source).
//   - ClaimPending atomically leases rows (bumping next_attempt_at forward)     → only one dispatcher works a row at a time (no concurrent double-send), and
//     a row whose worker crashed becomes claimable again once its lease expires
//     (never lost).
type Store interface {
	DeviceStore
	OutboxStore
}

// DeviceStore owns device-token registration.
type DeviceStore interface {
	// UpsertDevice registers or refreshes a device, keyed by (platform,
	// push_token). An existing row is updated (and reactivated). Returns the
	// stored device including its id.
	UpsertDevice(ctx context.Context, d Device, now time.Time) (Device, error)
	// DeactivateDeviceByToken marks a device inactive (sign-out, uninstall, or a
	// dead token reported by a push provider). Idempotent.
	DeactivateDeviceByToken(ctx context.Context, pushToken string) error
	// ActiveDevicesForUser returns the user's currently-active devices.
	ActiveDevicesForUser(ctx context.Context, userID string) ([]Device, error)
}

// OutboxStore owns the transactional outbox.
type OutboxStore interface {
	// EnqueueOutbox inserts a pending row, ignoring the insert if dedupe_key
	// already exists. Reports whether a new row was inserted.
	EnqueueOutbox(ctx context.Context, o Outbox) (inserted bool, err error)
	// ClaimPending atomically leases up to limit pending rows whose
	// next_attempt_at <= now, setting their next_attempt_at to now+lease so no
	// other worker (and no re-run before the lease elapses) picks them up.
	ClaimPending(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Outbox, error)
	// MarkSent finalizes a row as delivered.
	MarkSent(ctx context.Context, id string, sentAt time.Time) error
	// Reschedule records a transient failure: bump attempts and set the next
	// retry time. The row returns to pending.
	Reschedule(ctx context.Context, id string, attempts int, nextAttemptAt time.Time) error
	// MarkFailed finalizes a row as permanently failed (attempts exhausted).
	MarkFailed(ctx context.Context, id string) error
}
