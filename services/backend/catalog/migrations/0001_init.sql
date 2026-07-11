-- Catalog service schema: reference data + curated grounding corpus (pgvector).
-- One schema per service; no cross-service foreign keys (house rule).
--
-- IMPORTANT: the vector(1536) dimension below MUST match:
--   1. the `embedding.dimension` in configs/values_*.yaml, and
--   2. the dimension the Surprise service uses to embed queries.
-- Changing the embedding model/dimension requires editing this dimension AND
-- re-embedding the whole corpus.

BEGIN;

CREATE EXTENSION IF NOT EXISTS vector;

CREATE SCHEMA IF NOT EXISTS catalog;

-- Holidays: reference data served to clients.
CREATE TABLE IF NOT EXISTS catalog.holidays (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name      text        NOT NULL,
    date_rule text        NOT NULL DEFAULT 'fixed'
                          CHECK (date_rule IN ('fixed', 'relative')),
    region    text        NOT NULL DEFAULT '',
    tags      jsonb       NOT NULL DEFAULT '[]'::jsonb,
    active    boolean     NOT NULL DEFAULT true
);
CREATE INDEX IF NOT EXISTS holidays_region_active_idx
    ON catalog.holidays (region, active);

-- Categories: gift/date taxonomy served to clients. Self-referential parent
-- via parent_id, but no FK constraint kept intentionally simple/nullable.
CREATE TABLE IF NOT EXISTS catalog.categories (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name      text        NOT NULL,
    kind      text        NOT NULL
                          CHECK (kind IN ('gift', 'date')),
    parent_id uuid        NULL
);
CREATE INDEX IF NOT EXISTS categories_kind_idx
    ON catalog.categories (kind);

-- Budget bands: served alongside categories.
CREATE TABLE IF NOT EXISTS catalog.budget_bands (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    label     text        NOT NULL,
    min_cents integer     NOT NULL DEFAULT 0,
    max_cents integer     NOT NULL DEFAULT 0,
    currency  text        NOT NULL DEFAULT 'USD'
);

-- Inspiration: the curated grounding corpus. `embedding` is the pgvector column
-- queried by SearchInspiration (for Surprise). category_id / budget_band_id are
-- optional soft references (no cross-table FK enforced, so rows can be seeded
-- independently).
CREATE TABLE IF NOT EXISTS catalog.inspiration (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title          text          NOT NULL,
    body           text          NOT NULL,
    category_id    uuid          NULL,
    budget_band_id uuid          NULL,
    tags           jsonb         NOT NULL DEFAULT '[]'::jsonb,
    embedding      vector(1536)  NOT NULL,
    curated_by     text          NOT NULL DEFAULT '',
    curated_at     timestamptz   NOT NULL DEFAULT now(),
    active         boolean       NOT NULL DEFAULT true
);

-- Approximate-nearest-neighbour index for cosine similarity search. HNSW gives
-- fast, high-recall search for the modest corpus volume. cosine ops match the
-- `<=>` operator used by SearchInspiration.
CREATE INDEX IF NOT EXISTS inspiration_embedding_hnsw_idx
    ON catalog.inspiration
    USING hnsw (embedding vector_cosine_ops);

-- Filter helper for the WHERE clauses of SearchInspiration.
CREATE INDEX IF NOT EXISTS inspiration_active_cat_budget_idx
    ON catalog.inspiration (active, category_id, budget_band_id);

-- Seed reference data (idempotent: inserts only when the table is empty), so a
-- fresh stack gives clients real holidays / categories / budget bands to pick.
INSERT INTO catalog.holidays (name, date_rule)
SELECT v.name, v.date_rule
FROM (VALUES
    ('Valentine''s Day', 'fixed'),
    ('Anniversary', 'relative'),
    ('Birthday', 'relative'),
    ('Christmas', 'fixed'),
    ('Mother''s Day', 'relative'),
    ('Father''s Day', 'relative'),
    ('New Year''s', 'fixed'),
    ('Just Because', 'fixed')
) AS v(name, date_rule)
WHERE NOT EXISTS (SELECT 1 FROM catalog.holidays);

INSERT INTO catalog.categories (name, kind)
SELECT v.name, v.kind
FROM (VALUES
    ('Experiences', 'gift'),
    ('Jewelry', 'gift'),
    ('Tech & Gadgets', 'gift'),
    ('Books', 'gift'),
    ('Home & Cozy', 'gift'),
    ('Restaurant', 'date'),
    ('Outdoors', 'date'),
    ('Arts & Culture', 'date'),
    ('Night In', 'date')
) AS v(name, kind)
WHERE NOT EXISTS (SELECT 1 FROM catalog.categories);

INSERT INTO catalog.budget_bands (label, min_cents, max_cents, currency)
SELECT v.label, v.min_cents, v.max_cents, 'USD'
FROM (VALUES
    ('Under $50', 0, 5000),
    ('$50–$150', 5000, 15000),
    ('$150–$300', 15000, 30000),
    ('$300+', 30000, 100000)
) AS v(label, min_cents, max_cents)
WHERE NOT EXISTS (SELECT 1 FROM catalog.budget_bands);

COMMIT;
