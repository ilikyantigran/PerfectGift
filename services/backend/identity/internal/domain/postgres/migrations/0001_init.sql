-- Identity service schema. Owns users, their password credentials, and the
-- mapping from external (Apple/Google) identities to local users.
-- No cross-service foreign keys: this schema is self-contained.

CREATE SCHEMA IF NOT EXISTS identity;

-- citext gives us case-insensitive unique emails.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS identity.users (
    id           uuid PRIMARY KEY,
    email        citext UNIQUE,               -- nullable for social-only accounts
    display_name text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'active',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- One password credential per user (only for email/password accounts).
CREATE TABLE IF NOT EXISTS identity.credentials (
    user_id       uuid PRIMARY KEY REFERENCES identity.users(id) ON DELETE CASCADE,
    type          text NOT NULL DEFAULT 'password',
    password_hash text NOT NULL,
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- External identity links. provider_subject is unique per provider.
CREATE TABLE IF NOT EXISTS identity.oauth_links (
    user_id          uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    provider         text NOT NULL,           -- 'apple' | 'google'
    provider_subject text NOT NULL,
    linked_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_subject)
);

CREATE INDEX IF NOT EXISTS oauth_links_user_idx ON identity.oauth_links (user_id);
