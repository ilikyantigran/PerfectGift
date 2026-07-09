# Catalog Service — Implementation Plan

_Contract: `services/backend/catalog/SERVICE.md` + root `architecture.md`. House
style: `knp-service`. Status: **APPROVED** (Tigran pre-authorized; no interactive
gate in this session)._

## 1. What we're building & why

The **Catalog Service** — the read-mostly reference + grounding service for
PerfectGift. It serves holidays / categories / budget bands to clients (cache-hot
reads) and provides **semantic grounding** to the Surprise service via pgvector
similarity search over a curated inspiration corpus. Rare admin writes. No NATS.

RPCs (exactly as the contract lists them):
- `ListHolidays{region?, active?, on_or_after?}` → `{holidays[]}` — cached, client read (REST `GET /v1/holidays`).
- `GetCategories{kind?}` → `{categories[], budget_bands[]}` — cached, client read (REST `GET /v1/categories`).
- `SearchInspiration{query_text | query_embedding, filters{category_id, budget_band_id}, top_k}` → `{snippets[]}` — **internal only** (Surprise), pgvector similarity. No REST annotation.
- `UpsertInspiration{inspiration}` → `{id}` — admin/editorial write; (re)computes embedding, invalidates reference cache. No REST annotation.

## 2. Layout (house style, self-contained Go module)

Module: `github.com/ilikyantigran/PerfectGift/services/backend/catalog`

```
catalog/
  api/catalog/v1/catalog.proto        # proto source (contract)
  cmd/catalog/main.go                 # thin entrypoint (template)
  configs/values_local.yaml           # host addresses
  configs/values_docker.yaml          # compose addresses
  internal/
    app/app.go                        # App object: NewApp/Run wiring
    app/catalog_server.go             # the 4 RPC handlers (business logic)
    app/catalog_server_test.go        # unit tests vs fakes (hermetic)
    domain/postgres/postgres.go       # pgvector-backed store (reference + corpus)
    domain/postgres/postgres_test.go  # integration test, env-guarded (skips by default)
    domain/valkey/valkey.go           # hot reference read cache
    embedding/embedding.go            # Embedder interface + HTTP client + config
    embedding/fake.go                 # deterministic fake embedder (tests + local)
    embedding/embedding_test.go       # determinism + dimension tests
    infra/config/config.go            # Config struct + InitConfig
    infra/telemetry/telemetry.go      # portable (copied as-is)
    infra/docs/docs.go                # Swagger UI (parameterized)
    infra/docs/catalog.swagger.json   # generated, embedded
  migrations/0001_init.sql            # catalog schema, pgvector, tables, HNSW index
  pkg/api/catalog/v1/                  # generated pb / grpc / gateway / swagger
  vendor-proto/                        # google/api, validate, openapiv2 (copied from knp)
  Dockerfile                          # multi-stage → distroless
  Makefile                            # vendor-proto + generate
  README.md / PLAN.md / PROGRESS.md
```

## 3. Data model (Postgres `catalog` schema + pgvector)

- `holidays(id uuid, name text, date_rule text[fixed|relative], region text, tags jsonb, active bool)`
- `categories(id uuid, name text, kind text[gift|date], parent_id uuid null)`
- `budget_bands(id uuid, label text, min_cents int, max_cents int, currency text)`
- `inspiration(id uuid, title text, body text, category_id uuid null, budget_band_id uuid null, tags jsonb, embedding vector(D), curated_by text, curated_at timestamptz, active bool)`
- Extension `vector`; **HNSW index** on `inspiration.embedding vector_cosine_ops`.
- No cross-schema FKs (house rule). `D` = embedding dimension, pinned in config &
  migration (default **1536**); changing it re-embeds the corpus + edits migration.

## 4. Embedding contract (the pinned lever)

`Embedder` interface: `Embed(ctx, []string) ([][]float32, error)`, `Model() string`,
`Dimension() int`. Two impls:
- **HTTP client** — OpenAI-compatible `/embeddings` endpoint; model + dimension +
  endpoint + API-key-env from config. Real path.
- **Fake** — deterministic hash-seeded unit vectors of the configured dimension.
  Used by tests and by local runs when no endpoint is configured (so the service
  boots without an embedding API).

Model + dimension are pinned in config and **must match what Surprise queries
with** — documented loudly in README + config comments.

## 5. Interfaces the server depends on (enables hermetic tests)

Defined in `internal/app`:
```go
type ReferenceStore interface {
    ListHolidays(ctx, HolidayFilter) ([]Holiday, error)
    GetCategories(ctx, kind string) ([]Category, []BudgetBand, error)
}
type InspirationStore interface {
    SearchInspiration(ctx, embedding []float32, f SearchFilter, topK int) ([]Snippet, error)
    UpsertInspiration(ctx, Inspiration, embedding []float32) (string, error)
}
type Cache interface {
    GetJSON(ctx, key string, dst any) (bool, error)
    SetJSON(ctx, key string, v any, ttl time.Duration) error
    Invalidate(ctx, prefix string) error
}
type Embedder interface { Embed(...); Model() string; Dimension() int }
```
`postgres.Store` satisfies both stores; `valkey.Store` satisfies Cache. Fakes in
tests satisfy all four. Cache is best-effort (miss/err → fall through to DB).

## 6. Tests we write first (TDD, all hermetic — no live DB/network/API)

`internal/app/catalog_server_test.go` (fakes):
1. ListHolidays returns store rows; passes region/active/on_or_after filter through.
2. ListHolidays cache hit skips the store; cache miss populates cache.
3. GetCategories returns categories + budget bands; kind filter passed through.
4. SearchInspiration with `query_text` calls Embedder then vector search with top_k+filters.
5. SearchInspiration with `query_embedding` skips Embedder; validates dimension mismatch → InvalidArgument.
6. SearchInspiration validation: neither text nor embedding → InvalidArgument; top_k defaulted/capped.
7. UpsertInspiration computes embedding, stores, returns id, invalidates cache.
8. UpsertInspiration validation: empty title/body → InvalidArgument.

`internal/embedding/embedding_test.go`: fake determinism (same text→same vector),
correct dimension, unit-norm.

`internal/domain/postgres/postgres_test.go`: full round-trip against a real pgvector
DB — **skipped unless `CATALOG_TEST_DB_DSN` is set** (keeps `go test ./...` green
hermetically; runs in CI/local when a DB is provided).

## 7. Key decisions / trade-offs

- **pgx/v5 + pgvector-go** for the vector column (house uses valkey-go already;
  pgx is the standard Go pg driver). Alternative `database/sql` rejected — pgx
  native is cleaner for `vector`.
- **tags as jsonb**, surfaced in proto as `repeated string` (simple, sufficient).
- **budget min/max as integer cents** to avoid float money.
- **Integration test env-guarded** rather than testcontainers-by-default: keeps the
  default `go test ./...` fully hermetic with zero docker dependency, still lets a
  real DB prove the SQL. (Constraint explicitly allows guarded testcontainers;
  env-guard is the lighter, equally-hermetic choice.)
- **Fake embedder as the local/default** embedder so the service runs without an
  external embedding API; real HTTP client used when endpoint configured.
- Ports: gRPC `9096`, HTTP `8096` (avoid clashes with sibling services).

## 8. Risks

- Embedding dimension mismatch with Surprise → silently bad grounding. Mitigation:
  pin in config, validate `query_embedding` length in SearchInspiration, document.
- pgvector index (HNSW) requires `CREATE EXTENSION vector` — migration ordered first.
- `make generate` needs vendored protos; copied from knp (no network/git at gen time).

## 9. Definition of done

`go build ./...` + `go test ./...` GREEN inside `catalog/` (report exact output).
README with run (env incl. embedding model/dimension, migrations, docker) + test
instructions. Final file-tree + RPC + deviation report.
