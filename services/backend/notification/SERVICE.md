# Notification Service — Service Specification

> **Source of truth for building this service from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this service is for

Notification is the **async push fan-out** worker. It consumes domain events, resolves the
target User's devices, and pushes via **APNs (iOS)** / **FCM (Android)** with retry and
backoff. It exists as its own service because it is classic async fan-out with retries that
**should never sit in a request path**.

It has **no synchronous callers** other than device registration — it is driven by the bus.

**In scope**
- Consume **`PollCompleted`** ("your partner finished the poll") and **`IdeasReady`**
  ("your surprise ideas are ready").
- Resolve the target User → their registered devices.
- Push via APNs/FCM with **retry/backoff**.
- **Transactional outbox pattern** so a push is never lost or double-sent.
- Own device-token registration (register/refresh/unregister).

**Not in scope**
- Deciding *what* happened (that's the emitting service). It only delivers.
- Auth/accounts (Identity); it trusts the `user_id` in events.
- In-app inbox / notification history UI (could be added later on the same store).

---

## 2. Ownership & data

Owns its own **PostgreSQL** schema. (No Valkey needed for the MVP; the outbox lives in
Postgres.)

### PostgreSQL (`notification` schema)
- **`devices`** — `id`, `user_id`, `platform (ios|android)`, `push_token`, `app_version`,
  `registered_at`, `last_seen_at`, `active`. Unique on `(platform, push_token)`.
- **`notifications`** — the **outbox**: `id`, `user_id`, `type (poll_completed|ideas_ready)`,
  `payload (jsonb)`, `dedupe_key (unique)`, `status (pending|sent|failed)`, `attempts`,
  `next_attempt_at`, `created_at`, `sent_at`. Enables at-least-once with dedupe.

---

## 3. Contracts

### 3.1 gRPC API (`notification.v1`) — internal

| RPC | Request | Response | Notes |
|---|---|---|---|
| `RegisterDevice` | `{ user_id, platform, push_token, app_version }` | `{ device_id }` | Upsert on push_token; owner-scoped |
| `UnregisterDevice` | `{ push_token }` | `{ ok }` | On sign-out / uninstall signal |

The only public REST route: `POST /v1/devices` → `RegisterDevice`.

### 3.2 Events — NATS JetStream

**Consumes**
- **`PollCompleted`** `{ poll_id, owner_user_id, ... }` → push "poll done" to the owner.
- **`IdeasReady`** `{ request_id, user_id, idea_count }` → push "ideas ready" to the user.

**Publishes:** none (delivery receipts/analytics could be added later).

### 3.3 External — push providers

- **APNs** for iOS devices, **FCM** for Android devices. Retry with backoff; on hard
  failure (invalid/expired token) mark the device inactive.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose | Failure behavior |
|---|---|---|---|---|
| NATS JetStream | in | — | Consume `PollCompleted`, `IdeasReady` | Durable consumer; redelivery on crash |
| API Gateway | in | gRPC | `RegisterDevice` / `UnregisterDevice` | Best-effort, non-critical |
| APNs | out | HTTPS (token/cert) | iOS push | Retry/backoff; deactivate dead tokens |
| FCM | out | HTTPS | Android push | Retry/backoff; deactivate dead tokens |
| PostgreSQL | out | SQL | devices + outbox | Outbox = durable at-least-once |
| Identity | — (indirect) | JWT | Device registration validated at gateway |

**Note on user→device resolution:** events carry `user_id`; Notification maps it to devices
from its **own** `devices` table (populated by `RegisterDevice`). It does **not** call
Identity per push.

---

## 5. Delivery guarantees (owned here)

- **Transactional outbox:** the consumer writes an outbox row in the same transaction as
  marking the event handled; a separate dispatcher sends pending rows and marks them sent.
  → a push is **never lost** even if the process crashes mid-send.
- **Dedupe:** `dedupe_key` (e.g. `type + request_id/poll_id + user_id`) makes redelivery of
  the same event idempotent → **never double-sent**.
- **Eventual consistency is acceptable:** a one-second delay on a push is invisible to users
  (per the architecture's consistency stance).
- **Backoff + dead-token pruning** keep provider error rates healthy.

---

## 6. Tech stack & build notes

- **Language:** Go. gRPC (small) + NATS durable consumer + Postgres. APNs/FCM clients.
- **Migrations:** own the `notification` schema (devices, outbox).
- **Config (env):** DB DSN, NATS URL, APNs key/cert + topic, FCM credentials, retry/backoff
  params, dispatcher interval.
- **Build order:** **step 7** — after Poll and Surprise emit their events. Push for
  "poll done" / "ideas ready".

## 7. Non-functional targets

- Event → push delivered within ~1–2 s p95 (network + provider bound).
- Zero lost / zero duplicate notifications under crashes (outbox + dedupe).
- Device registration p95 < 200 ms.
