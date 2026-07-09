-- Surprise service schema. Owns its own `surprise` schema; no cross-service FKs.
-- pgvector powers dedup/similarity across generated ideas.
CREATE EXTENSION IF NOT EXISTS vector;
CREATE SCHEMA IF NOT EXISTS surprise;

CREATE TABLE IF NOT EXISTS surprise.surprise_requests (
    id               uuid PRIMARY KEY,
    user_id          text        NOT NULL,
    holiday_id       text        NOT NULL,
    budget_band      text        NOT NULL,
    preferences_text text        NOT NULL DEFAULT '',
    poll_id          text,
    idempotency_key  text        NOT NULL UNIQUE,
    status           text        NOT NULL DEFAULT 'queued',
    model_tier       text        NOT NULL DEFAULT 'sonnet',
    refinement       text        NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_requests_user ON surprise.surprise_requests (user_id);

CREATE TABLE IF NOT EXISTS surprise.generated_ideas (
    id                uuid PRIMARY KEY,
    request_id        uuid        NOT NULL REFERENCES surprise.surprise_requests (id) ON DELETE CASCADE,
    title             text        NOT NULL,
    why_it_fits       text        NOT NULL DEFAULT '',
    rough_cost        text        NOT NULL DEFAULT '',
    how_to            text        NOT NULL DEFAULT '',
    rank              int         NOT NULL DEFAULT 0,
    moderation_status text        NOT NULL DEFAULT 'approved',
    embedding         vector(1536),
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ideas_request ON surprise.generated_ideas (request_id);

CREATE TABLE IF NOT EXISTS surprise.saved_ideas (
    id       uuid PRIMARY KEY,
    user_id  text        NOT NULL,
    idea_id  uuid        NOT NULL REFERENCES surprise.generated_ideas (id) ON DELETE CASCADE,
    saved_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, idea_id)
);
