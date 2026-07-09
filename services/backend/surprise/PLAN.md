# Surprise Service — Implementation Plan

_Contract: `SERVICE.md` (authoritative) + root `architecture.md`. TDD workflow; the
human-approval gate is pre-authorized by Tigran — this file records the plan before
implementation._

## 1. Goal

Build the **Surprise Service** from scratch, test-driven, in the Go house-style layout
(`knp-service` shape): gRPC internal + HTTP/Swagger edge, Postgres (own `surprise`
schema + pgvector) + Valkey + NATS JetStream, Anthropic Claude behind an interface.
`go build ./...` and `go test ./...` must be GREEN and **hermetic** (no live DB, NATS,
network, or API key).

Self-contained Go module:
`github.com/ilikyantigran/PerfectGift/services/backend/surprise`.

## 2. Contract surface (from SERVICE.md §3)

### gRPC RPCs (`surprise.v1`)
| RPC | Request | Response | HTTP edge |
|---|---|---|---|
| `RequestGeneration` | user_id, holiday_id, budget_band, preferences_text, poll_id?, tier?, idempotency_key | request_id, status=QUEUED | `POST /v1/generations` (202) |
| `GetGenerationStatus` | request_id | status, progress? | `GET /v1/generations/{request_id}/status` |
| `GetIdeas` | request_id | ideas[] (ranked) | `GET /v1/generations/{request_id}/ideas` |
| `SaveIdea` | user_id, idea_id | ok | `POST /v1/ideas/save` |
| `Refine` | request_id, refinement | request_id, status=QUEUED | `POST /v1/generations/{request_id}/refine` |

### Events (NATS JetStream)
- **Publishes** `GenerationRequested` (durable work-queue job) + `IdeasReady` (pub/sub).
- **Consumes** `GenerationRequested` (own worker pool).

### External — Anthropic Claude
- Structured output via **tool use** (typed JSON idea objects).
- Model tiering: `claude-sonnet-5` default, `claude-opus-4-8` premium, `claude-haiku-4-5`
  moderation. Wrapped in retry + timeout + **circuit breaker**. Worker-only.

## 3. Data model

### Postgres (`surprise` schema + pgvector)
- `surprise_requests(id uuid pk, user_id, holiday_id, budget_band, preferences_text,
  poll_id null, idempotency_key unique, status, model_tier, refinement null, created_at)`
- `generated_ideas(id uuid pk, request_id fk, title, why_it_fits, rough_cost, how_to,
  rank, moderation_status, embedding vector(1536), created_at)`
- `saved_ideas(id uuid pk, user_id, idea_id fk, saved_at, unique(user_id, idea_id))`

### Valkey
- `surprise:status:{request_id}` → job status JSON (cheap poll target)
- `surprise:idem:{idempotency_key}` → request_id (dedupe submits)
- `surprise:llmcache:{hash}` → cached ideas JSON (main cost lever)

## 4. Package layout (house style)

```
surprise/
  api/surprise/v1/surprise.proto        # contract source (grpc-gateway annotations)
  cmd/surprise/main.go                  # thin entrypoint
  configs/values_local.yaml | values_docker.yaml
  internal/
    app/app.go                          # App object (NewApp/Run): wires gRPC+HTTP+worker
    app/surprise_server.go              # RPC handlers (request-path only; never LLM)
    clients/poll.go  clients/catalog.go # OUTbound stubs + interfaces + fakes (local proto shapes)
    domain/postgres/                    # store + migrations (embedded)
    domain/valkey/                      # status, idempotency, LLM cache (in-memory-testable iface)
    llm/                                # Client interface + Anthropic raw-HTTP impl + deterministic fake
    events/                             # NATS publisher/consumer interface + in-memory fake
    pipeline/                           # THE generation pipeline (§5) — pure, fully unit-tested
    resilience/                         # circuit breaker + retry/backoff
    infra/config | telemetry | docs
  pkg/api/surprise/v1/                  # generated pb/grpc/gateway/swagger
  vendor-proto/                         # vendored google/api etc. for codegen
  Dockerfile  Makefile  README.md
```

Everything external (Claude, Poll, Catalog, DB, Valkey, NATS) behind an interface with a
fake so the pipeline + server are unit-tested with zero I/O.

## 5. Generation pipeline (SERVICE.md §5 — the core algorithm)

1. **Accept** RequestGeneration → validate → dedupe on `idempotency_key` (Valkey + DB
   unique) → persist `surprise_requests` (status `queued`) → publish `GenerationRequested`
   → return fast (202).
2. **Worker pulls** the durable job → set status `running`.
3. **LLM cache check** — hash normalized inputs; hit → skip Claude call.
4. **Gather grounding** — `Poll.GetResponses(poll_id)` (best-effort; missing→skip) +
   `Catalog.SearchInspiration` (best-effort; degrade if unavailable).
5. **Build structured prompt** → call Claude via tool-use (typed idea objects), tier-selected,
   wrapped in retry + circuit breaker.
6. **Moderate** (Haiku pass) + validate + **rank** N candidates.
7. **Persist** `generated_ideas` (+ embeddings), cache the response, set status `ready`.
8. **Publish** `IdeasReady`.
Circuit-breaker open / hard failure → status `failed`.

## 6. Test strategy (test-first)

- `resilience`: breaker opens after N failures, half-open recovery; retry honors max attempts.
- `llm`: fake returns deterministic ideas; anthropic request builder shape (tool schema).
- `valkey` fake: idempotency set-if-absent, status round-trip, cache hit/miss.
- `pipeline`: cache-hit path, grounding-degrade path, moderation-reject filtering, ranking,
  failure→failed status, IdeasReady published with correct idea_count.
- `server`: validation errors, idempotent RequestGeneration (same key → same request_id, one
  publish), owner-scoping on GetIdeas/SaveIdea/Refine, status maps to gRPC codes.
- `clients`: fakes used by pipeline tests; real stubs compile against local proto shapes.

No test requires DB/NATS/network/API key. `go test ./...` hermetic.

## 7. Build order

proto → generate into pkg/api → config + telemetry/docs → resilience → llm (iface+fake+real)
→ events (iface+fake+nats) → clients (iface+fake+stub) → valkey + postgres → pipeline
→ server → app wiring → Dockerfile/Makefile → README → verify build+test.

## 8. Deviations / assumptions

- **Anthropic client** implemented as a self-contained raw-HTTP client (`net/http` +
  `encoding/json`) using the Messages API with `tool_choice`-forced tool use for structured
  JSON — avoids adding the Go SDK as a dependency and keeps the module light; the fake is
  used in all tests. Model IDs per architecture.md: `claude-sonnet-5`/`claude-opus-4-8`/
  `claude-haiku-4-5`, adaptive thinking omitted (fast structured extraction).
- **Embeddings**: pgvector column dimension pinned at 1536; embeddings produced by the LLM
  layer behind the same interface (deterministic fake in tests). Real embedding endpoint is a
  config knob; a zero/hash-based vector is used if unconfigured (grounding still works via
  Catalog). Documented as a v1 simplification — must match Catalog's embedding space in prod.
- **JWT/owner-scope**: the acting user id is taken from request context (gateway injects it);
  in tests it is passed explicitly. Owner checks compare request/idea owner to the caller.
- Worker pool size, cache TTL, breaker thresholds, retry budget: all env/config knobs.
