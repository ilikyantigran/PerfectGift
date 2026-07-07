# Poll Service — Service Specification

> **Source of truth for building this service from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this service is for

Poll owns the **anonymous, link-scoped, two-sided** flow that is PerfectGift's
differentiator. A **User** (authenticated) creates a poll tied to a specific surprise. The
service mints a **signed, expiring link token**. The **Subject** (the partner, *no
account, ever*) opens that link — in the Poll Web Page or on a handed-over phone — fetches
the poll by token, and submits answers **without authenticating**.

This is **the only service exposed to unauthenticated public internet traffic**, so it
owns its own aggressive rate limiting and abuse controls. Its blast radius is deliberately
contained by being a separate service.

**In scope**
- Poll creation (owner-scoped, from poll templates/questions).
- Mint **opaque, expiring, single-poll-scoped link tokens** (stored **hashed**).
- Anonymous token-scoped fetch + submit for the Subject.
- Capture and store Subject responses; owner-only read of responses.
- Emit **`PollCompleted`** when the Subject finishes.
- Aggressive anti-abuse: anonymous rate limiting, token expiry, one-response guarding.

**Not in scope**
- Generating ideas (Surprise reads poll responses via gRPC).
- Notifying the User (Notification consumes `PollCompleted`).
- User accounts / auth (Identity). The Subject is never authenticated.

---

## 2. Ownership & data

Owns its own **PostgreSQL** schema + **Valkey** cache. Surprise reads poll data **only via
gRPC** — never the DB.

### PostgreSQL (`poll` schema)
- **`polls`** — `id (uuid pk)`, `owner_user_id`, `surprise_request_id (nullable)`,
  `title`, `questions (jsonb)`, `status (draft|active|completed|expired)`, `expires_at`,
  `created_at`.
- **`poll_links`** — `id`, `poll_id (fk)`, `token_hash (unique)`, `expires_at`,
  `revoked (bool)`, `created_at`. **Only the hash is stored**; the raw token exists only in
  the shared URL.
- **`poll_responses`** — `id`, `poll_id (fk)`, `answers (jsonb)`, `submitted_at`,
  `client_fingerprint (coarse, anti-abuse)`.

### Valkey
- **`token → poll` resolution cache** — hot path for the public fetch.
- **Anonymous rate-limit counters** — per token / per IP, TTL'd. First line of defense
  against link-spam.
- **Short-lived poll session** — a narrowly-scoped session issued to the Subject after a
  valid token fetch, so submit is tied to a fetched poll.

---

## 3. Contracts

### 3.1 gRPC API (`poll.v1`) — internal

| RPC | Auth | Request | Response | Notes |
|---|---|---|---|---|
| `CreatePoll` | User (JWT subject) | `{ title, questions, surprise_request_id?, ttl }` | `{ poll_id, link_token, link_url, expires_at }` | Raw token returned **once** |
| `GetPollByToken` | Anonymous (token) | `{ token }` | `{ poll_id, title, questions }` | Validates token + expiry; issues Subject session; **no owner data leaked** |
| `SubmitResponse` | Anonymous (token) | `{ token, answers }` | `{ ok }` | Rate-limited; validates against poll questions; emits `PollCompleted` |
| `GetResponses` | User (owner only) | `{ poll_id }` | `{ responses[] }` | Owner-scoped authz: caller's JWT subject must equal `owner_user_id` |

### 3.2 Events — NATS JetStream

**Publishes**
- **`PollCompleted`** — payload `{ poll_id, surprise_request_id?, owner_user_id, completed_at }`.
  Consumed by Notification (tell the User) and, later, analytics. Read (via gRPC, not this
  event) by Surprise when it pulls responses for grounding.

**Consumes:** none.

### 3.3 Public REST surface (via gateway)

`GET /v1/polls/token/{t}` → `GetPollByToken`; `POST /v1/polls/token/{t}/responses` →
`SubmitResponse`. These two routes are the entire public/anonymous attack surface. CORS is
allowed for the Poll Web Page origin on these routes only.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose |
|---|---|---|---|
| API Gateway | in | gRPC | Owner routes (JWT) + anonymous token routes |
| Poll Web Page / native app | in (via gateway) | HTTPS | Subject fetch + submit |
| Surprise Service | in | gRPC (`GetResponses`) | Pulls poll answers as generation grounding |
| Notification Service | out (event) | NATS | `PollCompleted` → push "poll done" |
| Identity | — (indirect) | JWT | Owner routes validated locally via JWKS |
| PostgreSQL | out | SQL | polls, links (hashed), responses |
| Valkey | out | RESP | token cache, anonymous rate limits, Subject session |
| NATS JetStream | out | — | publish `PollCompleted` |

---

## 5. Security & abuse model (owned here)

Because this is the public surface, these are **first-class requirements**, not extras:
- **Opaque tokens, stored hashed** — a DB leak does not expose live links.
- **Expiry** — every link has `expires_at`; expired/revoked tokens 404 uniformly (no
  distinguishing "expired" vs "never existed" to avoid enumeration).
- **No cross-poll leakage** — a token resolves to exactly one poll; responses are scoped.
- **Anonymous rate limiting** — per token + per IP in Valkey; aggressive by default.
- **Owner-scoped authz** — `GetResponses`/`CreatePoll` check the JWT subject owns the poll.
- **Light content handling** — free-text answers are personal; treat as sensitive, and
  they later pass Surprise's moderation before influencing output.

---

## 6. Tech stack & build notes

- **Language:** Go. gRPC internal. Postgres + Valkey. NATS producer.
- **Migrations:** own the `poll` schema.
- **Config (env):** DB DSN, Valkey URL, NATS URL, token signing secret, default link TTL,
  anonymous rate-limit budgets, allowed web origin.
- **Build order:** **step 5** — after the core generation loop works (Identity → Surprise →
  one app → async). Poll + the web poll page ship together as the two-sided feature.

## 7. Non-functional targets

- Public fetch/submit p95 < 300 ms.
- Must withstand link-spam bursts without touching Postgres (Valkey rate limit + token cache).
- Strongly consistent poll/response writes (single-service Postgres transactions).
