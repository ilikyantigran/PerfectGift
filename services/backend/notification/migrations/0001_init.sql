-- Notification service schema: device registry + transactional outbox.
-- Owned solely by this service; no cross-schema foreign keys (house rule).

CREATE SCHEMA IF NOT EXISTS notification;

-- gen_random_uuid() is in core Postgres >= 13; pgcrypto covers older versions.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- devices: push targets, one row per (platform, push_token).
CREATE TABLE IF NOT EXISTS notification.devices (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       text        NOT NULL,
    platform      text        NOT NULL CHECK (platform IN ('ios', 'android')),
    push_token    text        NOT NULL,
    app_version   text        NOT NULL DEFAULT '',
    registered_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    active        boolean     NOT NULL DEFAULT true,
    UNIQUE (platform, push_token)
);

-- Fan-out reads resolve a user's active devices.
CREATE INDEX IF NOT EXISTS devices_user_active_idx
    ON notification.devices (user_id) WHERE active;

-- notifications: the transactional outbox. One row per logical notification
-- (user + event), deduplicated by dedupe_key so a redelivered bus event never
-- creates a second row.
CREATE TABLE IF NOT EXISTS notification.notifications (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         text        NOT NULL,
    type            text        NOT NULL CHECK (type IN ('poll_completed', 'ideas_ready')),
    payload         jsonb       NOT NULL,
    dedupe_key      text        NOT NULL UNIQUE,
    status          text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts        int         NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz
);

-- The dispatcher claims due pending rows; this partial index keeps that cheap.
CREATE INDEX IF NOT EXISTS notifications_due_idx
    ON notification.notifications (next_attempt_at) WHERE status = 'pending';
