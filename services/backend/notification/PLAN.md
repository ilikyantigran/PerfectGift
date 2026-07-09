# Notification Service — Implementation Plan

_Contract: `SERVICE.md` (+ root `architecture.md`, plus `poll/SERVICE.md` &
`surprise/SERVICE.md` for consumed event shapes). Built test-first per
`tdd-workflow`, in the Go house-style layout per `backend-service`.
Approval gate: **pre-authorized by Tigran** (no interactive human in-session)._

## 1. What this service is / what changes

Build the **Notification Service** from scratch: an async push fan-out worker.
It:
- consumes `PollCompleted` and `IdeasReady` from **NATS JetStream** (durable
  consumer),
- writes a **transactional-outbox** row (idempotent on `dedupe_key`),
- a separate **dispatcher** resolves the target user's devices from its **own**
  `devices` table and pushes via **APNs (iOS) / FCM (Android)** with
  retry/backoff, deactivating dead tokens,
- owns **device registration** over a small gRPC surface
  (`RegisterDevice`, `UnregisterDevice`) plus one REST route
  `POST /v1/devices`.

Delivery guarantees owned here: **never lost** (durable outbox + retry + crash
recovery via lease) and **never double-sent** (unique `dedupe_key` makes event
redelivery idempotent; atomic claim/lease means one dispatcher sends a row at a
time). Delivery itself is **at-least-once with dedupe**, exactly as the spec
states.

## 2. Layout (all under `services/backend/notification/`)

```
api/notification/v1/notification.proto     proto contract (gRPC + gateway HTTP)
pkg/api/notification/v1/*                   generated (pb, grpc, gateway, swagger)
cmd/notification/main.go                    thin entrypoint
configs/values_{local,docker}.yaml          CONFIG_PATH-selected config
migrations/0001_init.sql                    notification schema (devices, outbox)
internal/app/app.go                         App: wires stores/pushers/consumers/dispatcher + gRPC/HTTP
internal/app/notification_server.go         gRPC impl: RegisterDevice / UnregisterDevice
internal/notify/types.go                    domain types + event decode + dedupe/payload builders
internal/notify/store.go                    Store interface (devices + outbox)
internal/notify/push.go                     Pusher interface + platform router + sentinel errors
internal/notify/consumer.go                 event handlers + JetStream Subscription interface + runner
internal/notify/dispatcher.go               outbox dispatcher (claim → push → mark)
internal/notify/*_test.go                   hermetic unit tests + in-memory fakes
internal/domain/postgres/postgres.go        real Store (pgx) — compiled, not in hermetic tests
internal/push/{apns,fcm}.go                 real Pushers (thin HTTP) — compiled, not in hermetic tests
internal/events/nats.go                     real JetStream Subscription adapter
internal/infra/{config,telemetry,docs}      house infra (config extended; telemetry verbatim)
Dockerfile · Makefile · go.mod · README.md · PROGRESS.md
```

Module: `github.com/ilikyantigran/PerfectGift/services/backend/notification`.

## 3. Data model (own `notification` Postgres schema)

- **devices**: `id uuid pk`, `user_id text`, `platform text (ios|android)`,
  `push_token text`, `app_version text`, `registered_at`, `last_seen_at`,
  `active bool`. **Unique (platform, push_token)**. Upsert reactivates.
- **notifications** (outbox): `id uuid pk`, `user_id text`,
  `type text (poll_completed|ideas_ready)`, `payload jsonb`,
  `dedupe_key text UNIQUE`, `status text (pending|sent|failed)`,
  `attempts int`, `next_attempt_at timestamptz`, `created_at`, `sent_at`.

## 4. Contracts

**gRPC `notification.v1`**
- `RegisterDevice{user_id, platform, push_token, app_version} -> {device_id}` —
  upsert on push_token. REST: `POST /v1/devices`.
- `UnregisterDevice{push_token} -> {ok}` — deactivate; idempotent.

**Events consumed (NATS JetStream, durable consumer)** — JSON matching producers:
- `PollCompleted{poll_id, surprise_request_id?, owner_user_id, completed_at}`
  → outbox `type=poll_completed`, user=`owner_user_id`,
  `dedupe_key="poll_completed:"+poll_id+":"+owner_user_id`.
- `IdeasReady{request_id, user_id, idea_count}`
  → outbox `type=ideas_ready`, user=`user_id`,
  `dedupe_key="ideas_ready:"+request_id+":"+user_id`.

Handler = decode → `EnqueueOutbox` (idempotent) → **ack**. Ack only after the
DB commit; if ack is lost, redelivery hits the unique `dedupe_key` and no second
row is created.

## 5. Dispatcher algorithm

Loop every `dispatch_interval`:
1. `ClaimPending(now, lease, batch)` — atomically select `pending` rows with
   `next_attempt_at <= now`, bump `next_attempt_at = now + lease` (a **lease**;
   this is also crash recovery — an un-acked crashed row becomes claimable again
   after the lease).
2. For each row: resolve `ActiveDevicesForUser`; push to each by platform.
   - transient error → `Reschedule(attempts+1, now+backoff(attempts))`; if
     `attempts+1 >= max_attempts` → `MarkFailed`.
   - dead/invalid token (`ErrInvalidToken`) → `DeactivateDeviceByToken`; not a
     transient failure of the row.
   - all device attempts terminal (delivered or token deactivated), incl. zero
     devices → `MarkSent`.
Backoff: exponential `base * 2^(attempts-1)`.

## 6. Tests I will write first (all hermetic — no live DB/NATS/provider)

Against an in-memory fake `Store` (with atomic lease/claim) and fake `Pusher`:
1. **Enqueue dedupe** — same event twice → exactly one outbox row (never
   double-sent at the source).
2. **Consumer decode** — `PollCompleted`/`IdeasReady` JSON → correct
   type/user/dedupe_key/payload; bad JSON → error (Nak, not ack).
3. **Dispatch happy path** — pending → pushes to all active devices → `sent`,
   `sent_at` set.
4. **Zero devices** → marked `sent` (nothing lost, nothing to send).
5. **Transient failure** → attempts++ and future `next_attempt_at`, stays
   `pending`, not sent (proves not-lost / will retry).
6. **Max attempts** → `failed`.
7. **Dead token** → device deactivated; row still completes.
8. **Concurrency / no double-send** — two dispatch passes race one row → pushed
   once, `sent` once (atomic claim).
9. **Crash recovery** — claim a row, don't mark, advance clock past lease →
   re-claimed and sent (not lost).
10. **gRPC server** — RegisterDevice upsert returns device_id; platform
    validation; UnregisterDevice deactivates + is idempotent.

## 7. Key decisions / trade-offs

- **Outbox row = per (user, event)**, fan-out to N devices at dispatch time.
  Retry re-pushes to the user's active devices → delivery is at-least-once (the
  spec's exact stance); dedupe_key + atomic claim prevent duplicate rows /
  concurrent double-send. Alternative (per-device rows) rejected: heavier, and
  the contract explicitly says "at-least-once with dedupe."
- **Lease-based claim** unifies retry scheduling and crash recovery in one
  mechanism (no separate reaper).
- **Everything external behind interfaces** (`Store`, `Pusher`, JetStream
  `Subscription`) with fakes → `go test ./...` is fully hermetic. Real pgx /
  NATS / APNs / FCM adapters are compiled but never touched by tests.
- **Ids**: string UUIDs (user-facing device entity), per house convention.
- Injectable **clock** (`func() time.Time`) so retry/lease/crash tests are
  deterministic.

## 8. Risks

- No git / offline-ish: `make vendor-proto` clones from the internet → instead
  **copy vendored protos from the reference `knp-service`** (cp, not git).
- Codegen must succeed with local protoc plugins (present) → verify early.
- Confine every change to `services/backend/notification/` (hard constraint).
