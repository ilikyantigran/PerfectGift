# Notification Service

Async **push fan-out** worker for PerfectGift. It consumes domain events from
NATS JetStream, resolves the target user's devices from its **own** `devices`
table, and pushes via **APNs (iOS) / FCM (Android)** with retry/backoff. A
**transactional outbox** guarantees a push is never lost and never double-sent.
The only synchronous surface is device registration.

See [`SERVICE.md`](./SERVICE.md) for the contract and the root
[`architecture.md`](../../../architecture.md) for the system context.

## What it does

- **Consumes** `PollCompleted` and `IdeasReady` (durable JetStream consumers).
- **Enqueues** one outbox row per logical notification, idempotent on
  `dedupe_key` → a redelivered event never creates a second notification.
- **Dispatches** the outbox on an interval: claims due rows with an atomic
  lease, fans each out to the user's active devices, and finalizes the row with
  retry/backoff; dead tokens are pruned (device deactivated).
- **Owns** device registration over gRPC (+ one REST route).

### Delivery guarantees

| Guarantee | Mechanism |
|---|---|
| **Never lost** | Durable outbox row stays `pending` until delivered or exhausted; a crash mid-send lets the claim **lease** expire so a later sweep retries it. |
| **Never double-sent** | Unique `dedupe_key` (one row per logical event) + **atomic lease** in `ClaimPending` (one dispatcher works a row at a time). |
| **At-least-once delivery** | A retry may re-push to a device that already received it — exactly the contract's "at-least-once with dedupe" stance. |

## Contracts

### gRPC (`notification.v1`)

| RPC | Request | Response | REST |
|---|---|---|---|
| `RegisterDevice` | `{user_id, platform, push_token, app_version}` | `{device_id}` | `POST /v1/devices` |
| `UnregisterDevice` | `{push_token}` | `{ok}` | `DELETE /v1/devices/{push_token}` |

`RegisterDevice` upserts on `(platform, push_token)` and reactivates an existing
row. `UnregisterDevice` deactivates and is idempotent.

### Events consumed (NATS JetStream)

| Event | JSON shape | Outbox `dedupe_key` |
|---|---|---|
| `PollCompleted` | `{poll_id, surprise_request_id?, owner_user_id, completed_at}` | `poll_completed:<poll_id>:<owner_user_id>` |
| `IdeasReady` | `{request_id, user_id, idea_count}` | `ideas_ready:<request_id>:<user_id>` |

## Data model (own `notification` schema)

- **`devices`** — `id, user_id, platform (ios\|android), push_token,
  app_version, registered_at, last_seen_at, active`; unique `(platform,
  push_token)`.
- **`notifications`** (outbox) — `id, user_id, type, payload (jsonb),
  dedupe_key (unique), status (pending\|sent\|failed), attempts,
  next_attempt_at, created_at, sent_at`.

Migration: [`migrations/0001_init.sql`](./migrations/0001_init.sql).

## Configuration

Config is loaded from the YAML file named by `CONFIG_PATH`
(default `./configs/values_local.yaml`; the Docker image sets
`values_docker.yaml`). Keys are data-only — **secrets come from the
environment / mounted files**, not the YAML.

| Section | Keys |
|---|---|
| `notification_service` | `host`, `grpc_port` (8090), `http_port` (8091) |
| `postgres` | `dsn` |
| `nats` | `url`, `stream`, `durable`, `poll_completed_subject`, `ideas_ready_subject` |
| `dispatcher` | `interval`, `batch`, `max_attempts`, `base_backoff`, `lease` |
| `apns` | `enabled`, `key_path`, `key_id`, `team_id`, `topic`, `sandbox` |
| `fcm` | `enabled`, `credentials_path`, `project_id` |

**Push credentials** (not in config):
- **APNs** — a token-auth `.p8` key at `apns.key_path`, with `key_id`,
  `team_id`, and `topic` (app bundle id). Set `apns.enabled: true`.
- **FCM** — a service-account JSON at `fcm.credentials_path` (its
  `project_id` is used unless overridden). Set `fcm.enabled: true`.

A disabled provider is simply not registered; a push to that platform then
fails loudly (and is retried) rather than being silently dropped.

## Run

```bash
# 1. Apply the schema (psql, or your migration tool of choice)
psql "$DSN" -f migrations/0001_init.sql

# 2. Run locally (needs Postgres + NATS JetStream reachable per values_local.yaml)
go run ./cmd/notification            # CONFIG_PATH defaults to ./configs/values_local.yaml

# or in Docker (uses values_docker.yaml)
docker build -t perfectgift-notification .
docker run --rm -p 8090:8090 -p 8091:8091 perfectgift-notification
```

The HTTP port serves the REST gateway, `GET /metrics` (Prometheus), and
`GET /swagger/` (Swagger UI). The gRPC port serves `notification.v1` with
reflection enabled.

Register a device through the REST edge:

```bash
curl -X POST localhost:8091/v1/devices \
  -H 'content-type: application/json' \
  -d '{"user_id":"u-1","platform":"PLATFORM_IOS","push_token":"abc","app_version":"1.0"}'
```

## Test

```bash
go test ./...            # fully hermetic — no DB / NATS / push provider needed
go test -race ./...      # the concurrency + crash guarantees under the race detector
```

All external dependencies (Postgres, NATS, APNs, FCM) sit behind interfaces
(`notify.Store`, `notify.Pusher`, `notify.Subscription`); unit tests drive the
consumer, dispatcher, and gRPC server against in-memory fakes. The outbox
guarantees (dedupe, retry/backoff, dead-token pruning, no-double-send under
concurrency, crash recovery) are proven in
[`internal/notify/dispatcher_test.go`](./internal/notify/dispatcher_test.go)
and [`consumer_test.go`](./internal/notify/consumer_test.go).

## Regenerate the API

```bash
make generate            # protoc → pkg/api + copies swagger into internal/infra/docs
```

`make vendor-proto` re-vendors the third-party protos (needs network/git);
they are already committed under `vendor-proto/`.

## Layout

```
api/notification/v1/notification.proto   contract (gRPC + gateway HTTP annotations)
cmd/notification/main.go                 entrypoint (CONFIG_PATH → App.Run)
configs/values_{local,docker}.yaml       config
migrations/0001_init.sql                 notification schema
internal/app/                            App wiring + gRPC server impl
internal/notify/                         domain: types, Store/Pusher/Subscription ifaces, consumer, dispatcher (+ tests)
internal/domain/postgres/                real Store (pgx)
internal/events/                         real NATS JetStream Subscription
internal/push/                           real APNs + FCM Pushers
internal/infra/{config,telemetry,docs}   house infra
pkg/api/notification/v1/                 generated code (importable by other services)
```
