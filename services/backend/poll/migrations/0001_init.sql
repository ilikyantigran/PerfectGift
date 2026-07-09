-- Poll service schema. Owns the `poll` schema; no cross-service foreign keys.
-- Run against the service's own Postgres database.

CREATE SCHEMA IF NOT EXISTS poll;

SET search_path TO poll;

-- polls: the poll definition, owned by a User (owner_user_id = JWT subject).
CREATE TABLE IF NOT EXISTS polls (
    id                  uuid PRIMARY KEY,
    owner_user_id       text        NOT NULL,
    surprise_request_id text,
    title               text        NOT NULL,
    questions           jsonb       NOT NULL,
    status              text        NOT NULL DEFAULT 'active'
        CHECK (status IN ('draft', 'active', 'completed', 'expired')),
    expires_at          timestamptz NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS polls_owner_idx ON polls (owner_user_id);

-- poll_links: opaque, expiring link tokens. Only the SHA-256 HASH is stored;
-- the raw token exists solely in the shared URL.
CREATE TABLE IF NOT EXISTS poll_links (
    id          uuid PRIMARY KEY,
    poll_id     uuid        NOT NULL REFERENCES polls (id) ON DELETE CASCADE,
    token_hash  text        NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    revoked     boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- poll_responses: the Subject's answers. client_fingerprint is coarse anti-abuse
-- data (hash of IP + user-agent), never raw PII.
CREATE TABLE IF NOT EXISTS poll_responses (
    id                 uuid PRIMARY KEY,
    poll_id            uuid        NOT NULL REFERENCES polls (id) ON DELETE CASCADE,
    answers            jsonb       NOT NULL,
    submitted_at       timestamptz NOT NULL DEFAULT now(),
    client_fingerprint text
);

CREATE INDEX IF NOT EXISTS poll_responses_poll_idx ON poll_responses (poll_id);
