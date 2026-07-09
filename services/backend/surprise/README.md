# Surprise Service

The **heart of PerfectGift**: async LLM idea generation. It turns sparse inputs
(a holiday, a budget band, free-form preferences, and an optional poll) into
several genuinely good, budget-appropriate, non-generic surprise ideas —
reliably, in a few seconds, at controlled cost.

Generation is slow, expensive, and depends on a third party (Anthropic Claude),
so it is **never** done on the request path. `RequestGeneration` persists +
enqueues and returns fast (HTTP 202); a worker pool consumes the durable
`GenerationRequested` job, pulls grounding, calls Claude with tool-use, then
moderates/validates/ranks/persists the ideas and emits `IdeasReady`.

> Contract: [`SERVICE.md`](./SERVICE.md) (authoritative) + the root
> [`architecture.md`](../../../architecture.md). Build plan: [`PLAN.md`](./PLAN.md).

## Architecture at a glance

```
Gateway ──gRPC──▶ RequestGeneration ─┐   (fast path: persist + enqueue, 202)
                                     ├─▶ Postgres (surprise schema + pgvector)
                                     ├─▶ Valkey   (status · idempotency · LLM cache)
                                     └─▶ NATS JetStream: publish GenerationRequested (durable job)
                                                           │
   worker pool ◀── consume GenerationRequested ◀───────────┘
        │  1. status=running
        │  2. LLM cache check (hash of normalized inputs)
        │  3. grounding: Poll.GetResponses + Catalog.SearchInspiration  (best-effort, degrade)
        │  4. Claude tool-use → typed idea objects (tier-selected; retry + circuit breaker)
        │  5. moderate (Haiku) + validate + rank
        │  6. persist generated_ideas (+ embeddings), cache response, status=ready
        └─ 7. publish IdeasReady  ──▶ Notification (external)
```

Every external dependency (Claude, Poll, Catalog, Postgres, Valkey, NATS) sits
behind an interface with a fake, so `go test ./...` runs with **no** live DB,
NATS, network, or API key.

## RPCs (`surprise.v1`) and REST edge

| RPC | REST | Notes |
|---|---|---|
| `RequestGeneration` | `POST /v1/generations` | Persist + idempotency + enqueue; returns `202 {request_id, QUEUED}` |
| `GetGenerationStatus` | `GET /v1/generations/{request_id}/status` | Cheap Valkey-backed poll |
| `GetIdeas` | `GET /v1/generations/{request_id}/ideas` | Ranked; empty until READY |
| `SaveIdea` | `POST /v1/ideas/save` | Favorite/save |
| `Refine` | `POST /v1/generations/{request_id}/refine` | Re-queue with adjustments |

All RPCs are owner-scoped: the acting user (JWT subject, injected by the gateway
as the `x-user-id` metadata header) must own the request/idea.

## Events (NATS JetStream)

- **Publishes** `GenerationRequested` — a durable **work-queue** job consumed by
  this service's own worker pool.
- **Publishes** `IdeasReady` — a pub/sub event (`{request_id, user_id, idea_count}`)
  consumed by Notification.
- **Consumes** `GenerationRequested` — the worker pool pulls jobs off the request
  path.

## Model tiering (Anthropic Claude)

| Tier | Model | Use |
|---|---|---|
| default | `claude-sonnet-5` | standard generation |
| premium / "deep" | `claude-opus-4-8` | requested via `tier=MODEL_TIER_OPUS` |
| moderation | `claude-haiku-4-5` | cheap safety/classification pass |

Structured output is obtained via **forced tool use** (`emit_ideas`), so ideas
come back as typed JSON objects. The Claude call is wrapped in **retry + timeout
+ circuit breaker** and only ever runs inside the worker.

## Configuration

Config is selected by `CONFIG_PATH` (default `./configs/values_local.yaml`; the
Docker image sets `values_docker.yaml`). The two files share keys and differ only
in addresses. **Secrets are never in the file** — the Anthropic API key comes from
the environment.

| Env var | Purpose |
|---|---|
| `CONFIG_PATH` | Which YAML to load (`values_local.yaml` / `values_docker.yaml`) |
| `ANTHROPIC_API_KEY` | **Required at runtime** for real generation (unused by tests) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Optional; enables OTLP tracing when set |

Config knobs (see `configs/*.yaml`): Postgres DSN, Valkey address, NATS
URL/stream/subjects/durable-name, model IDs per tier, embedding dimension,
worker pool size & ideas-wanted, retry/backoff & circuit-breaker thresholds, and
cache TTLs (status, idempotency, LLM response cache).

## Data stores

- **PostgreSQL + pgvector** (`surprise` schema): `surprise_requests`,
  `generated_ideas` (+ `embedding vector(1536)`), `saved_ideas`. Migrations are
  embedded (`internal/domain/postgres/migrations`) and applied automatically on
  startup by `App.Run` via `Store.Migrate`.
- **Valkey**: job status (poll target), idempotency keys (short-circuit duplicate
  submits), and the LLM response cache (hash of normalized inputs → cached ideas —
  the main cost lever).

## Run it

Prerequisites for a full local run: PostgreSQL (with the `vector` extension
available), Valkey/Redis, NATS with JetStream, and `ANTHROPIC_API_KEY`.

```bash
# from services/backend/surprise/
export ANTHROPIC_API_KEY=sk-ant-...           # required for real generation
export CONFIG_PATH=./configs/values_local.yaml
go run ./cmd/surprise
```

On start the service applies migrations, connects to Valkey + NATS (creating the
`SURPRISE` stream), binds the durable worker consumer, and serves:

- gRPC on `:9090`
- HTTP on `:8080` — REST gateway, `/metrics` (Prometheus), `/swagger/` (Swagger UI)

The worker pool starts with the process; queue depth is the scaling signal.

### Regenerate the API (proto → Go)

```bash
make vendor-proto   # once: vendor google/api, validate, openapiv2 protos
make generate       # protoc → pkg/api (+ swagger copied into internal/infra/docs)
```

### Docker

```bash
docker build -t perfectgift/surprise .
docker run --rm -p 9090:9090 -p 8080:8080 \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  perfectgift/surprise            # CONFIG_PATH defaults to values_docker.yaml
```

## Test it

Fully hermetic — no DB, NATS, network, or API key required:

```bash
go build ./...
go test ./...
```

The suite covers the resilience primitives (breaker open/half-open, retry
exhaust/permanent), the LLM resilient decorator, the whole generation pipeline
(cache-hit skips the LLM, grounding degradation, moderation/validation filtering,
contiguous re-ranking, failure → `failed`, `IdeasReady` emission), the request-path
handlers (validation, idempotency, owner-scoping, refine, save), and an
end-to-end loop wiring the server → job → worker → pipeline with in-memory fakes.

## Layout

```
api/surprise/v1/surprise.proto     contract source (gRPC + grpc-gateway annotations)
api/{poll,catalog}/v1/*.proto      LOCAL copies of downstream shapes (Surprise calls OUT)
cmd/surprise/main.go               thin entrypoint
configs/                           values_local.yaml · values_docker.yaml
internal/
  app/                             App object (Run) + gRPC server + tests
  clients/                         Poll & Catalog: interfaces, gRPC stubs, fakes
  domain/                          entities + Repository/Cache interfaces
    memory/                        in-memory store (test/dev)
    postgres/                      pgx store + embedded migrations (+ pgvector)
    valkey/                        valkey-go cache
  events/                          NATS JetStream producer/consumer + in-memory Bus fake
  llm/                             Client interface + Anthropic raw-HTTP impl + fake + resilient decorator
  pipeline/                        the generation algorithm (§5) + worker
  resilience/                      circuit breaker + retry/backoff
  infra/{config,telemetry,docs}
pkg/api/                           generated code (importable by other services)
Dockerfile · Makefile
```

## Deviations & assumptions

- **Anthropic client** is a self-contained raw-HTTP client (`net/http` +
  `encoding/json`) against the Messages API with forced tool use — no third-party
  SDK dependency. The deterministic fake backs every test.
- **Embeddings**: Anthropic exposes no first-party embeddings endpoint on the
  Messages API, so the LLM layer derives a stable pseudo-embedding from text and
  stores it in the `vector(1536)` column. This is a v1 simplification — in
  production, swap in a real embedding provider whose model/dimension **matches
  Catalog's** embedding space (see catalog/SERVICE.md).
- **Owner identity** is read from the `x-user-id` request metadata (gateway
  injects it from the JWT subject); when absent (trusted internal calls) the
  owner check is skipped. Tests pass it explicitly.
- **Validation** is done in the handler (the proto carries `validate` rules for
  documentation; no validator code is generated), matching the house style.
