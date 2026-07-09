// Package postgres is the production implementation of notify.Store, backed by
// the service's own `notification` schema (devices + outbox). The unit tests
// use an in-memory fake instead, so this package is compiled but not exercised
// by `go test ./...`; run it against a real database via the migrations in
// ../../../migrations.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is a Postgres-backed notify.Store.
type Store struct {
	pool *pgxpool.Pool
}

var _ notify.Store = (*Store)(nil)

// New opens a connection pool to the given DSN and verifies connectivity.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// --- devices ---------------------------------------------------------------

func (s *Store) UpsertDevice(ctx context.Context, d notify.Device, now time.Time) (notify.Device, error) {
	const q = `
INSERT INTO notification.devices (user_id, platform, push_token, app_version, registered_at, last_seen_at, active)
VALUES ($1, $2, $3, $4, $5, $5, true)
ON CONFLICT (platform, push_token) DO UPDATE
    SET user_id = EXCLUDED.user_id,
        app_version = EXCLUDED.app_version,
        last_seen_at = EXCLUDED.last_seen_at,
        active = true
RETURNING id, registered_at, last_seen_at, active`
	row := s.pool.QueryRow(ctx, q, d.UserID, string(d.Platform), d.PushToken, d.AppVersion, now)
	if err := row.Scan(&d.ID, &d.RegisteredAt, &d.LastSeenAt, &d.Active); err != nil {
		return notify.Device{}, err
	}
	return d, nil
}

func (s *Store) DeactivateDeviceByToken(ctx context.Context, pushToken string) error {
	const q = `UPDATE notification.devices SET active = false WHERE push_token = $1`
	_, err := s.pool.Exec(ctx, q, pushToken)
	return err
}

func (s *Store) ActiveDevicesForUser(ctx context.Context, userID string) ([]notify.Device, error) {
	const q = `
SELECT id, user_id, platform, push_token, app_version, registered_at, last_seen_at, active
FROM notification.devices
WHERE user_id = $1 AND active = true`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []notify.Device
	for rows.Next() {
		var d notify.Device
		var platform string
		if err := rows.Scan(&d.ID, &d.UserID, &platform, &d.PushToken, &d.AppVersion,
			&d.RegisteredAt, &d.LastSeenAt, &d.Active); err != nil {
			return nil, err
		}
		d.Platform = notify.Platform(platform)
		out = append(out, d)
	}
	return out, rows.Err()
}

// --- outbox ----------------------------------------------------------------

func (s *Store) EnqueueOutbox(ctx context.Context, o notify.Outbox) (bool, error) {
	const q = `
INSERT INTO notification.notifications (user_id, type, payload, dedupe_key, status, attempts, next_attempt_at, created_at)
VALUES ($1, $2, $3, $4, 'pending', 0, $5, $6)
ON CONFLICT (dedupe_key) DO NOTHING`
	tag, err := s.pool.Exec(ctx, q, o.UserID, string(o.Type), []byte(o.Payload), o.DedupeKey, o.NextAttemptAt, o.CreatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ClaimPending atomically leases due rows using FOR UPDATE SKIP LOCKED, pushing
// their next_attempt_at to now+lease. SKIP LOCKED + the lease is what makes
// concurrent dispatchers safe (one worker per row) and gives crash recovery
// (a leased row un-finalized becomes due again once the lease elapses).
func (s *Store) ClaimPending(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]notify.Outbox, error) {
	const q = `
UPDATE notification.notifications
SET next_attempt_at = $2
WHERE id IN (
    SELECT id FROM notification.notifications
    WHERE status = 'pending' AND next_attempt_at <= $1
    ORDER BY next_attempt_at
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING id, user_id, type, payload, dedupe_key, status, attempts, next_attempt_at, created_at, sent_at`
	rows, err := s.pool.Query(ctx, q, now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []notify.Outbox
	for rows.Next() {
		var o notify.Outbox
		var typ, stat string
		var payload []byte
		if err := rows.Scan(&o.ID, &o.UserID, &typ, &payload, &o.DedupeKey, &stat,
			&o.Attempts, &o.NextAttemptAt, &o.CreatedAt, &o.SentAt); err != nil {
			return nil, err
		}
		o.Type = notify.Type(typ)
		o.Status = notify.Status(stat)
		o.Payload = payload
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) MarkSent(ctx context.Context, id string, sentAt time.Time) error {
	const q = `UPDATE notification.notifications SET status = 'sent', sent_at = $2 WHERE id = $1`
	return s.execOne(ctx, q, id, sentAt)
}

func (s *Store) Reschedule(ctx context.Context, id string, attempts int, nextAttemptAt time.Time) error {
	const q = `UPDATE notification.notifications SET status = 'pending', attempts = $2, next_attempt_at = $3 WHERE id = $1`
	return s.execOne(ctx, q, id, attempts, nextAttemptAt)
}

func (s *Store) MarkFailed(ctx context.Context, id string) error {
	const q = `UPDATE notification.notifications SET status = 'failed' WHERE id = $1`
	return s.execOne(ctx, q, id)
}

func (s *Store) execOne(ctx context.Context, q string, args ...any) error {
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no row updated")
	}
	return nil
}
