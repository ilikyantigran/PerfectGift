# Catalog Service

Read-mostly **reference data** (holidays, categories, budget bands) and the curated
**inspiration corpus** used to *ground* PerfectGift's generator. The corpus is
embedded with pgvector so the Surprise service can pull the most relevant grounding
per request — the main quality lever of the product. Rare admin writes, massive
cache-hit reads. No NATS events.

Part of the PerfectGift backend. Built in the house style (gRPC internal + HTTP/
Swagger edge, Postgres own schema + migrations, Valkey cache, OpenTelemetry).

## RPCs (`catalog.v1`)

| RPC | Exposure | Purpose |
|---|---|---|
| `ListHolidays{region?, active?, on_or_after?}` → `{holidays[]}` | Public REST `GET /v1/holidays` | Cached client reference read |
| `GetCategories{kind?}` → `{categories[], budget_bands[]}` | Public REST `GET /v1/categories` | Cached client reference read |
| `SearchInspiration{query_text \| query_embedding, filters{category,budget}, top_k}` → `{snippets[]}` | **gRPC internal only** | pgvector similarity grounding for Surprise |
| `UpsertInspiration{inspiration}` → `{id}` | gRPC (admin/editorial) | (Re)computes embedding, upserts corpus row, invalidates cache |

`SearchInspiration` / `UpsertInspiration` are **not** on the public REST edge (no
HTTP annotation, absent from Swagger) — exactly as the contract requires.

## Layout

```
api/catalog/v1/catalog.proto        proto contract (source of truth for the API)
cmd/catalog/main.go                 thin entrypoint (reads CONFIG_PATH)
configs/values_{local,docker}.yaml  config (same keys, different values)
internal/
  app/app.go                        App object: assembles + runs the service
  app/catalog_server.go             the 4 RPC handlers (business logic)
  app/mapping.go                    proto <-> domain mapping
  domain/model/                     neutral domain types
  domain/postgres/                  pgvector-backed store (reference + corpus)
  domain/valkey/                    hot reference-read cache
  embedding/                        Embedder interface + HTTP client + deterministic fake
  infra/{config,telemetry,docs}/    config, observability, Swagger UI
migrations/0001_init.sql            catalog schema, pgvector, tables, HNSW index
pkg/api/catalog/v1/                 generated gRPC / gateway / swagger
```

## Configuration (env)

Config is selected by `CONFIG_PATH` (defaults to `./configs/values_local.yaml`;
the Docker image sets `/app/configs/values_docker.yaml`). Files are data-only; the
embedding API key comes from the environment.

| Key | Meaning |
|---|---|
| `catalog_service.{host,grpc_port,http_port}` | gRPC `9096`, HTTP `8096` |
| `postgres.dsn` | Postgres+pgvector DSN (owns the `catalog` schema) |
| `valkey.address` | Valkey cache address |
| `embedding.model` | **Pinned** embedding model id |
| `embedding.dimension` | **Pinned** vector dimension (default `1536`) |
| `embedding.endpoint` | OpenAI-compatible embeddings URL; **empty → deterministic fake embedder** (service boots with no external API) |
| `embedding.api_key_env` | Name of the env var holding the API key (e.g. `EMBEDDING_API_KEY`) — never the key itself |
| `catalog.reference_cache_ttl_seconds` | TTL for cached reference reads |
| `catalog.{default_top_k,max_top_k}` | SearchInspiration default / hard cap |

> **Embedding contract (critical):** `embedding.model` + `embedding.dimension`
> MUST match what the **Surprise** service uses to embed queries, and must match
> the `vector(N)` dimension in `migrations/0001_init.sql`. Otherwise similarity
> search is meaningless. Changing the model/dimension requires editing the
> migration's `vector(...)` and **re-embedding the whole corpus**.

The embedding computation is behind the `embedding.Embedder` interface. When
`endpoint` is empty, a **deterministic fake** (stable unit vectors) is used — so
the service, its tests, and local runs never require a live embedding API. Set
`endpoint` (+ the key env var) to use the real OpenAI-compatible client.

## Run locally

```bash
# 1. Postgres + pgvector and Valkey (example)
docker run -d --name catalog-pg -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=catalog \
    -p 5432:5432 pgvector/pgvector:pg16
docker run -d --name catalog-valkey -p 6379:6379 valkey/valkey:latest

# 2. Apply the migration (creates the catalog schema, pgvector ext, HNSW index)
psql 'postgres://postgres:postgres@localhost:5432/catalog?sslmode=disable' \
    -f migrations/0001_init.sql

# 3. Run (uses the deterministic fake embedder since endpoint is empty)
go run ./cmd/catalog
#   gRPC  :9096
#   HTTP  :8096  ->  /v1/holidays, /v1/categories, /swagger/, /metrics

# To use a real embedding API instead of the fake:
#   set embedding.endpoint in the config and export the key:
#   export EMBEDDING_API_KEY=sk-...
```

Quick checks:

```bash
curl 'http://localhost:8096/v1/holidays?region=US&active=true'
curl 'http://localhost:8096/v1/categories?kind=CATEGORY_KIND_GIFT'
open http://localhost:8096/swagger/        # Swagger UI
curl http://localhost:8096/metrics         # Prometheus
```

`SearchInspiration` / `UpsertInspiration` are gRPC-only (e.g. via `grpcurl` against
`:9096`); they are intentionally not reachable over REST.

## Test

```bash
go test ./...        # fully hermetic: no DB / network / embedding API required
```

Server logic is unit-tested against fakes (validation, cache hit/miss,
embed-vs-supplied-vector, dimension mismatch, top_k defaulting/capping, error
mapping). The embedder is tested for determinism / dimension / unit-norm.

The Postgres integration tests (real SQL + the pgvector `<=>` search path +
in-place upsert) are **skipped by default** and run only when a database is
provided:

```bash
# DB must already have migrations/0001_init.sql applied
CATALOG_TEST_DB_DSN='postgres://postgres:postgres@localhost:5432/catalog?sslmode=disable' \
    go test ./internal/domain/postgres/...
```

## Build / codegen / docker

```bash
make vendor-proto      # one-time: vendor google/api, validate, openapiv2 protos
make generate          # regenerate pkg/api/** + embedded swagger from the proto
go build ./...
docker build -t perfectgift/catalog .    # multi-stage → distroless, CONFIG_PATH=values_docker.yaml
```

## Notes / deviations

- **Integration tests are env-guarded** (`CATALOG_TEST_DB_DSN`) rather than
  testcontainers-by-default, so `go test ./...` is hermetic with zero Docker
  dependency while still exercising real SQL when a DB is supplied.
- **`on_or_after`** on `ListHolidays` is accepted and plumbed through the filter
  but not yet applied (holiday dates are rule-based: `fixed`/`relative`); it is
  reserved for date-materialization logic. Documented so callers don't rely on it.
- `UpsertInspiration` invalidates the whole reference cache namespace per the
  contract. Editorial writes are rare, so the occasional cold reference cache is
  acceptable and keeps invalidation trivially correct.
- Embedding vectors are rendered to pgvector's text literal form and cast with
  `::vector`, so the store needs only the `pgx` driver (no extra vector codec dep).
