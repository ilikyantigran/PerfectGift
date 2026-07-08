# Poll Service — Implementation Plan

_Contract: `services/backend/poll/SERVICE.md` + root `architecture.md`. TDD (Explore → Plan → Implement test-first → Verify). Approval gate pre-authorized by Tigran._

## What we're building
The anonymous, link-scoped two-sided poll flow — the only public/unauthenticated
service. Users create polls; Subjects fetch by opaque expiring token (stored HASHED)
and submit answers without auth. Aggressive anonymous rate limiting (Valkey). Emits
`PollCompleted` on NATS JetStream. Owns its `poll` Postgres schema.

## House style (from `backend-service` skill + `knp-service`)
Self-contained Go module, `cmd/poll/main.go` thin entrypoint, `internal/app` App
object (`NewApp`/`Run`), `internal/infra/{config,telemetry,docs}`, `internal/domain/*`
state owners, `pkg/api` generated code, `api/*.proto` source, gRPC + grpc-gateway REST
+ Swagger + /metrics, `CONFIG_PATH`-selected YAML.

- Module: `github.com/ilikyantigran/PerfectGift/services/backend/poll`
- Proto pkg `poll.v1`, service `PollService`, go_package `…/poll/pkg/api/poll/v1;pollv1`

## RPCs and REST edge (exact, from §3.1 + §3.3)
| RPC | Auth | REST | Response |
|---|---|---|---|
| `CreatePoll` | User JWT | `POST /v1/polls` | `{poll_id, link_token, link_url, expires_at}` — raw token once |
| `GetPollByToken` | Anonymous | `GET /v1/polls/token/{token}` | `{poll_id, title, questions}` — no owner data |
| `SubmitResponse` | Anonymous, rate-limited | `POST /v1/polls/token/{token}/responses` | `{ok}` — emits `PollCompleted` |
| `GetResponses` | User (owner only) | `GET /v1/polls/{poll_id}/responses` | `{responses[]}` |

## Data model (Postgres `poll` schema, migration 0001)
- `polls(id uuid pk, owner_user_id text, surprise_request_id text null, title, questions jsonb, status, expires_at timestamptz, created_at)`
- `poll_links(id uuid pk, poll_id fk, token_hash text unique, expires_at, revoked bool, created_at)` — **only the hash**
- `poll_responses(id uuid pk, poll_id fk, answers jsonb, submitted_at, client_fingerprint text)`
- No cross-schema FKs (schema-internal FKs poll_links/responses → polls are fine).

Questions JSON shape: `[{id, prompt, type: text|single_choice|multi_choice, options[], required}]`.
Answers JSON shape: `[{question_id, text, choice_ids[]}]`.

## Security model (§5) — first-class
- Opaque token = 32 random bytes base64url; store **SHA-256 hash**; raw returned once.
- Uniform **NotFound (404)** for expired / revoked / unknown / already-consumed tokens (no enumeration oracle).
- Owner authz: acting user id from **JWT subject** (metadata `authorization: Bearer …`, HS256), never request body. `GetResponses`/`CreatePoll` require it; owner mismatch → uniform NotFound.
- Anonymous rate limiting per token + per IP (Valkey counters, TTL) → `ResourceExhausted` (429).
- One-response guard: a poll accepts one response, then → `completed`; further fetch/submit → uniform NotFound (consumed).

## Ports (interfaces) — enables hermetic tests
Defined in `internal/app`:
- `Repo` (Postgres): CreatePoll, GetPollByTokenHash, GetPollByID, RevokeAndComplete/InsertResponse (tx), GetResponses.
- `RateLimiter` (Valkey): `Allow(ctx, key, limit, window) (bool, error)`, plus token→poll cache helpers.
- `Publisher` (NATS): `PublishPollCompleted(ctx, PollCompleted) error`.
Real impls: `internal/domain/postgres`, `internal/domain/valkey`, `internal/domain/events` (NATS).
Fakes live in tests. **Tests never touch a live DB/NATS/network.**

## Auth
`internal/infra/auth`: HS256 JWT parse (`golang-jwt/jwt/v5`), `SubjectFrom(ctx)` helper,
unary interceptor populating context. Anonymous RPCs skip; owner RPCs require subject → else `Unauthenticated`.

## Events (§3.2)
Publishes `PollCompleted{poll_id, surprise_request_id?, owner_user_id, completed_at}` to
JetStream subject `poll.completed` (stream `POLL`). Consumes none.

## Config sections
`poll_service`(host/grpc/http), `postgres.dsn`, `valkey.address`, `nats`(url,stream,subject),
`security.jwt_secret`, `tokens.default_ttl_seconds`, `ratelimit`(per_token/per_ip budget+window),
`web.allowed_origin` + `link_path` (builds `link_url`), CORS for the two public routes.

## Tests to write first (red → green)
1. `token` unit: hash deterministic, raw random & != hash.
2. `answers` validation unit: required missing → err; unknown qid → err; choice ∉ options → err; happy path ok.
3. `pollserver` (fakes) — the rules:
   - CreatePoll: no JWT → Unauthenticated; stores hashed (fake repo sees hash, never raw); returns raw once; link_url + expiry correct.
   - GetPollByToken: valid ok & no owner_user_id in response; expired/revoked/unknown/consumed → NotFound (uniform).
   - SubmitResponse: valid → ok + emits PollCompleted + marks completed; bad token → NotFound; over budget → ResourceExhausted; invalid answers → InvalidArgument; second submit → NotFound.
   - GetResponses: owner ok; non-owner → NotFound; no JWT → Unauthenticated.
4. `auth` interceptor unit: valid token → subject; bad/absent → none.
5. In-memory fake rate limiter unit (fixed-window). Guarded testcontainers/integration skipped without env.

## Build order
scaffold templates → proto → `make generate` → config → domain (postgres/valkey/events) → auth → server (test-first) → wire App → Dockerfile/Makefile → README → verify (`go build ./...`, `go test ./...`, boot check).

## Assumptions / deviations (documented in README + final report)
- JWT is HS256 with a shared secret (Identity/JWKS not built yet); interface lets us swap to JWKS later.
- "Short-lived poll session" realized as a Valkey token→poll cache + the one-response/consumed guard; the contract response shapes carry no session field, so none is added.
- `owner_user_id`/`surprise_request_id` are strings (opaque subject ids), per architecture's UUID/string id posture.
- Consumed/owner-mismatch return uniform NotFound to avoid enumeration (per §5).
