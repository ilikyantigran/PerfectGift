# Surprise Service — Service Specification

> **Source of truth for building this service from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.
> This is **the heart of the product and the trickiest service** — treat generation
> quality as the actual deliverable.

---

## 1. What this service is for

Surprise orchestrates **idea generation**: it turns sparse, messy inputs (a holiday, a
budget band, a paragraph of free-form preferences, and an optional poll) into **several
genuinely good, budget-appropriate, non-generic** surprise ideas — reliably, in a few
seconds, at controlled cost.

Generation is **slow (~3–15 s), expensive (a paid LLM call), and depends on a third
party**. It scales on a different axis (LLM concurrency + cost, not web RPS). That is why
it is isolated: LLM latency, rate limits, and outages must not touch the rest of the
system. Generation is therefore an **async job**, never a blocking request.

**In scope**
- Accept a generation request; persist it with an **idempotency key**.
- **Enqueue a job** on NATS; return `202 {requestId}` immediately.
- A **worker pool** pulls jobs → gathers grounding (poll answers + Catalog corpus) →
  builds a structured prompt → calls **Claude** with **tool-use / structured JSON output**.
- **Validate, moderate, rank, persist** the resulting ideas; emit `IdeasReady`.
- **LLM response caching** by a hash of normalized inputs to cut cost on repeats.
- Save/favorite; regenerate/refine.
- **Model tiering:** Sonnet default, Opus for premium/"deep", Haiku for classification/moderation.

**Not in scope**
- Owning poll data (reads it from Poll via gRPC) or the corpus (reads Catalog via gRPC).
- Sending notifications (Notification consumes `IdeasReady`).
- Auth/accounts (Identity).

---

## 2. Ownership & data

Owns its own **PostgreSQL + pgvector** schema and a **Valkey** cache.

### PostgreSQL (`surprise` schema, + pgvector)
- **`surprise_requests`** — `id (uuid pk)`, `user_id`, `holiday_id`, `budget_band`,
  `preferences_text`, `poll_id (nullable)`, `idempotency_key (unique)`,
  `status (queued|running|ready|failed)`, `model_tier`, `created_at`.
- **`generated_ideas`** — `id`, `request_id (fk)`, `title`, `why_it_fits`,
  `rough_cost`, `how_to`, `rank`, `moderation_status`, `embedding (vector)`, `created_at`.
  The `embedding` supports dedup/similarity across ideas.
- **`saved_ideas`** — `id`, `user_id`, `idea_id (fk)`, `saved_at`. Strongly consistent.

### Valkey
- **Job status** — cheap polling target for `GetGenerationStatus`.
- **Idempotency keys** — short-circuit duplicate submits.
- **LLM response cache** — key = hash of normalized inputs → cached ideas; the main cost lever.

---

## 3. Contracts

### 3.1 gRPC API (`surprise.v1`) — internal

| RPC | Request | Response | Notes |
|---|---|---|---|
| `RequestGeneration` | `{ user_id, holiday_id, budget_band, preferences_text, poll_id?, tier?, idempotency_key }` | `{ request_id, status: queued }` | Persist + enqueue; returns fast (maps to HTTP `202`) |
| `GetGenerationStatus` | `{ request_id }` | `{ status, progress? }` | Backed by Valkey; cheap to poll |
| `GetIdeas` | `{ request_id }` | `{ ideas[] }` (ranked) | Owner-scoped; empty until `ready` |
| `SaveIdea` | `{ user_id, idea_id }` | `{ ok }` | Favorite/save |
| `Refine` | `{ request_id, refinement }` | `{ request_id, status: queued }` | Regenerate with adjustments |

Owner-scoped authz on all: caller's JWT subject must own the request/idea.

### 3.2 Events — NATS JetStream

**Publishes**
- **`GenerationRequested`** — a **durable work-queue** job `{ request_id, tier, ... }`.
  The Surprise worker pool consumes it (see below). This is a *job*, not a broadcast.
- **`IdeasReady`** — pub/sub event `{ request_id, user_id, idea_count }`. Consumed by
  Notification ("your ideas are ready") and future analytics.

**Consumes**
- **`GenerationRequested`** — its own workers pull these to do the slow work off the
  request path. (Poll's `PollCompleted` is *not* consumed here; Surprise reads poll
  answers on demand via gRPC when a job runs.)

### 3.3 External — Anthropic Claude

- Structured output via **tool use** so ideas come back as **typed JSON objects** (not
  prose to parse).
- **Model tiering:** `claude-sonnet-5` default; `claude-opus-4-8` premium/deep;
  `claude-haiku-4-5` for cheap moderation/classification passes.
- Wrapped in **retry + timeout + circuit breaker**. Never called on the request path —
  only inside the worker.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose | Failure behavior |
|---|---|---|---|---|
| API Gateway | in | gRPC | Lifecycle RPCs | Always `202` fast; never block on LLM |
| Poll Service | out | gRPC (`GetResponses`) | Pull poll answers as grounding | Missing/expired poll → generate without it |
| Catalog Service | out | gRPC (`SearchInspiration`) | pgvector grounding snippets | Degrade to weaker grounding if unavailable |
| Anthropic Claude | out | HTTPS | The generation + moderation calls | Circuit breaker → mark `failed`, surface "try again" UX |
| Notification | out (event) | NATS | `IdeasReady` push | Eventual; a delay is invisible |
| NATS JetStream | both | — | `GenerationRequested` (queue), `IdeasReady` (event) | Durable queue survives worker restarts |
| PostgreSQL + pgvector | out | SQL | requests, ideas, saves, embeddings | Transactional idea ledger |
| Valkey | out | RESP | job status, idempotency, LLM cache | Cache miss = full LLM call |

---

## 5. Generation pipeline (the core algorithm)

1. **Accept** `RequestGeneration` → validate → dedupe on `idempotency_key` → persist
   `surprise_requests` (status `queued`) → publish `GenerationRequested` → return `202`.
2. **Worker pulls** the job (durable queue). Set status `running`.
3. **Check LLM cache** (hash of normalized inputs). Hit → skip the LLM call.
4. **Gather grounding:** `Poll.GetResponses(poll_id)` (if any) + `Catalog.SearchInspiration`
   (pgvector similarity on the request) → concrete, on-brand seeds.
5. **Build structured prompt**; call **Claude** with tool-use for typed idea objects.
6. **Moderate** (Haiku pass) + validate + **rank** the N candidates.
7. **Persist** `generated_ideas` (+ embeddings), cache the response, set status `ready`.
8. **Publish** `IdeasReady`. Client learns via push or by polling `GetGenerationStatus`.

**Cost & resilience are first-class:** response caching, model tiering, per-user rate
limits (enforced at gateway), idempotency, circuit breaker + backoff around Claude.

---

## 6. Tech stack & build notes

- **Language:** Go. gRPC internal. Postgres + pgvector. Valkey. NATS (producer + durable consumer).
- **Migrations:** own the `surprise` schema incl. pgvector extension + embedding columns.
- **Config (env):** DB DSN, Valkey URL, NATS URL, Anthropic API key + model IDs per tier,
  timeouts/retry/circuit-breaker thresholds, worker pool size, cache TTL, rate-limit budgets.
- **Build order:** **step 2 of the MVP, then step 4.** First build a *sync-ish*
  `RequestGeneration` calling Claude with a hand-written prompt + small hardcoded grounding
  to nail **quality**. Only after the flow works, turn it **async** (NATS job + status
  polling). Catalog/pgvector grounding replaces the hardcoded seeds at step 6.

## 7. Non-functional targets

- `RequestGeneration` returns in **< 300 ms** (just persist + enqueue).
- Generation completes typically **3–15 s** async.
- LLM **cost-per-generation is a tracked business metric** (tokens, cache hit rate, tier mix).
- Worker pool autoscales for holiday spikes; queue depth is a scaling signal.
