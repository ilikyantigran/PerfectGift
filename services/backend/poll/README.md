# Poll Service

The anonymous, link-scoped, two-sided poll flow for PerfectGift — and the **only**
service exposed to unauthenticated public internet traffic. A **User** (authenticated)
creates a poll; the service mints an **opaque, expiring, single-poll link token**
(stored **hashed**). A **Subject** (no account, ever) opens the link, fetches the poll
by token, and submits answers **without authenticating**. On completion the service
emits **`PollCompleted`** on NATS JetStream (consumed by Notification; poll answers are
later read by Surprise via `GetResponses`).

Built in the house style: gRPC internal + HTTP/Swagger edge, its own Postgres `poll`
schema, Valkey for anonymous rate limiting, NATS producer, `CONFIG_PATH`-selected YAML,
telemetry (slog JSON + OTel tracing + Prometheus).

## RPCs (`poll.v1`)

| RPC | Auth | REST edge | Notes |
|---|---|---|---|
| `CreatePoll` | User (JWT subject) | `POST /v1/polls` | Returns `{poll_id, link_token, link_url, expires_at}`. Raw token returned **once**. |
| `GetPollByToken` | Anonymous | `GET /v1/polls/token/{token}` | Validates token + expiry; **no owner data leaked**. |
| `SubmitResponse` | Anonymous, rate-limited | `POST /v1/polls/token/{token}/responses` | Validates answers vs questions; one response per poll; emits `PollCompleted`. |
| `GetResponses` | User (owner only) | `GET /v1/polls/{poll_id}/responses` | Owner-scoped: JWT subject must equal `owner_user_id`. |

The two anonymous `token` routes are the entire public attack surface. CORS is granted
only to the configured `web.allowed_origin`.

### Event published

`PollCompleted` → subject `poll.completed` (stream `POLL`), payload
`{ poll_id, surprise_request_id?, owner_user_id, completed_at }`. Consumes nothing.

## Security & abuse model

- **Opaque tokens, stored hashed** — 256-bit random token (base64url) in the URL; only
  its SHA-256 hash is persisted. A DB leak exposes no live links.
- **Uniform 404** — expired, revoked, unknown, and already-consumed tokens all return
  `NotFound`, so links can't be enumerated ("expired" vs "never existed" are
  indistinguishable). A non-owner on `GetResponses` gets the same `NotFound` as a missing
  poll.
- **Anonymous rate limiting** — per-token and per-IP fixed-window counters in Valkey,
  applied **before** any Postgres work, so link-spam bursts never reach the DB. Over
  budget → `ResourceExhausted` (HTTP 429).
- **One-response guard** — a poll accepts exactly one response, then flips `active →
  completed` atomically; further fetch/submit read as gone.
- **Owner-scoped authz** — the acting user id is taken from the verified JWT subject,
  never from a request body field.

## Data model (Postgres `poll` schema)

- `polls(id, owner_user_id, surprise_request_id?, title, questions jsonb, status, expires_at, created_at)`
- `poll_links(id, poll_id, token_hash unique, expires_at, revoked, created_at)` — hash only
- `poll_responses(id, poll_id, answers jsonb, submitted_at, client_fingerprint)`

Migrations are embedded (`migrations/*.sql`) and applied on startup (idempotent).

## Configuration

Selected by `CONFIG_PATH` (defaults to `./configs/values_local.yaml`; the Docker image
sets `/app/configs/values_docker.yaml`). Keys:

| Section | Keys |
|---|---|
| `poll_service` | `host`, `grpc_port` (8080), `http_port` (8081) |
| `postgres` | `dsn` |
| `valkey` | `address` |
| `nats` | `url`, `stream`, `subject` |
| `security` | `jwt_secret` (HS256; inject via secrets manager in prod) |
| `tokens` | `default_ttl_seconds` |
| `ratelimit` | `per_token_budget`, `per_token_window`, `per_ip_budget`, `per_ip_window` |
| `web` | `allowed_origin`, `link_path` (`/p/{token}`) |

## Run locally

```bash
# dependencies (examples)
docker run --rm -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=poll -p 5432:5432 -d postgres:16
docker run --rm -p 6379:6379 -d valkey/valkey:8
docker run --rm -p 4222:4222 -d nats:2 -js        # JetStream enabled

# generate API code (only needed after editing the .proto)
make generate

# run — applies migrations on startup
go run ./cmd/poll
```

Ports: gRPC on `:8080`, HTTP (REST gateway + `/metrics` + `/swagger/`) on `:8081`.
Swagger UI: <http://localhost:8081/swagger/>.

Mint a dev JWT for the owner routes with the configured `jwt_secret` (HS256, `sub` =
owner id) and send it as `Authorization: Bearer <jwt>`.

## Docker

```bash
docker build -t perfectgift-poll .
docker run --rm -p 8080:8080 -p 8081:8081 perfectgift-poll   # uses values_docker.yaml
```

## Test

```bash
go build ./...
go test ./...      # hermetic: no live DB / Valkey / NATS required
```

Unit tests cover token hashing, question/answer validation, the JWT auth interceptor,
and every RPC's rules (auth, uniform-404, rate limiting, one-response guard, owner
scoping, `PollCompleted` emission) against in-memory fakes. Optional integration tests
run only when their env var is set:

```bash
POLL_TEST_POSTGRES_DSN='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
    go test ./internal/domain/postgres/ -run Integration -v
POLL_TEST_VALKEY_ADDR=127.0.0.1:6379 go test ./internal/domain/valkey/ -run Integration -v
```

## Layout

```
api/poll/v1/poll.proto            proto contract (source of truth for shapes)
cmd/poll/main.go                  thin entrypoint
configs/values_{local,docker}.yaml
internal/app/                     App object + PollService server + proto<->model mapping
internal/ports/                   Repo / RateLimiter / Publisher interfaces + shared types
internal/domain/{postgres,valkey,events,token,model}
internal/infra/{config,telemetry,docs,auth}
migrations/*.sql                  poll schema (embedded)
pkg/api/poll/v1/                  generated gRPC + gateway + swagger (importable)
```

## Deviations / assumptions

- **JWT is HS256 with a shared secret.** Identity/JWKS isn't built yet; verification is
  behind the `auth` package so swapping to RS256/JWKS later needs no handler changes.
- **"Short-lived poll session"** from the spec is realized via the token→poll resolution
  path plus the one-response/consumed guard. The contract response shapes carry no
  session field, so none was added.
- `owner_user_id` / `surprise_request_id` are opaque strings (JWT subject / caller id).
- Event publish failures are logged, not surfaced: the Postgres write is the source of
  truth and delivery is eventually consistent. A transactional outbox is a future upgrade.
