# API Gateway / BFF — Service Specification

> **Source of truth for building this service from scratch.** Read this together with the
> root [`architecture.md`](../../../architecture.md). Where this file and the architecture
> disagree, the architecture wins — open an issue.

---

## 1. What this service is for

The **API Gateway / Backend-for-Frontend (BFF)** is the single public HTTP edge of
PerfectGift. Every client — iOS, Android, and the Poll Web Page — talks **only** to the
gateway. The gateway translates the outside world's REST/JSON into internal **gRPC**
calls to the backing services, and it owns all the concerns that must happen *once, at
the edge*: TLS termination, authentication checks, rate limiting, request shaping for
mobile, CORS for the web poll page, and the public **OpenAPI/Swagger** contract.

It is **stateless**. It owns no database and no schema. It can be scaled horizontally
behind a load balancer and restarted freely.

**Explicitly in scope**
- TLS termination and HTTP/JSON ⇄ gRPC translation.
- JWT validation on authenticated routes (local verification against Identity's public key).
- Route-level authorization gates (authenticated vs. anonymous/public routes).
- Global + per-user + per-IP rate limiting.
- Request/response shaping and aggregation for mobile (BFF role): collapse chatty
  internal calls into one client-friendly payload where useful.
- The public **OpenAPI 3 / Swagger** document — this is the client contract.
- CORS policy for the Poll Web Page origin.
- Edge observability: request tracing entry point, access logs, metrics.

**Explicitly NOT in scope**
- Business logic of any kind (lives in the domain services).
- Persistence (stateless).
- Token *issuance* or credential storage (that is Identity).
- The anonymous poll link-token validation itself (Poll Service validates the token; the
  gateway just routes the request without a JWT).

---

## 2. Ownership & data

| Concern | Owns? | Notes |
|---|---|---|
| Database | ❌ | Stateless. No schema. |
| Cache | ➖ optional | A small Valkey/local cache for JWKS (Identity's public key set) and rate-limit counters. |
| Public contract | ✅ | The OpenAPI/Swagger doc is generated and owned here. |

---

## 3. Contracts

### 3.1 Inbound — public REST/JSON API (the client contract)

All routes are versioned under `/v1`. Auth column: **JWT** = requires a valid Identity
access token in `Authorization: Bearer <jwt>`; **Token** = anonymous, carries an opaque
poll link token in the path; **Public** = no auth.

| Method & Path | Auth | Routes to (gRPC) | Purpose |
|---|---|---|---|
| `POST /v1/auth/signin` | Public | Identity `SignIn` | Sign in with Apple/Google/email → tokens |
| `POST /v1/auth/refresh` | Public* | Identity `RefreshToken` | Rotate refresh → new access token |
| `POST /v1/auth/revoke` | JWT | Identity `Revoke` | Sign out / revoke session |
| `GET  /v1/me` | JWT | Identity `GetMe` | Current user profile |
| `POST /v1/polls` | JWT | Poll `CreatePoll` | Create a poll, get share link |
| `GET  /v1/polls/{id}/responses` | JWT | Poll `GetResponses` | Owner reads Subject answers |
| `GET  /v1/polls/token/{t}` | Token | Poll `GetPollByToken` | Subject fetches poll by link token |
| `POST /v1/polls/token/{t}/responses` | Token | Poll `SubmitResponse` | Subject submits answers (rate-limited) |
| `POST /v1/generations` | JWT | Surprise `RequestGeneration` | Start a generation → `202 {requestId}` |
| `GET  /v1/generations/{id}` | JWT | Surprise `GetGenerationStatus` + `GetIdeas` | Poll status; return ideas when ready |
| `POST /v1/generations/{id}/refine` | JWT | Surprise `Refine` | Regenerate/refine ideas |
| `POST /v1/ideas/{id}/save` | JWT | Surprise `SaveIdea` | Save/favorite an idea |
| `GET  /v1/holidays` | JWT | Catalog `ListHolidays` | Reference: holidays |
| `GET  /v1/categories` | JWT | Catalog `GetCategories` | Reference: categories/budget bands |
| `POST /v1/devices` | JWT | Notification `RegisterDevice` | Register APNs/FCM device token |

`*` refresh carries the refresh token in the body; it is not a JWT-protected route but
is treated as sensitive (strict rate limit).

**Conventions**
- Content type `application/json`; snake_case field names in JSON.
- Errors use a uniform envelope: `{ "error": { "code": "string", "message": "string", "details": {...} } }`
  with standard HTTP status codes (400/401/403/404/409/422/429/500).
- Idempotency: `POST /v1/generations` accepts an `Idempotency-Key` header, forwarded to
  Surprise.
- Async: generation returns **`202 Accepted`** with `{ requestId }`; clients poll
  `GET /v1/generations/{id}` (or wait for push).

### 3.2 Outbound — gRPC to internal services

The gateway is a **gRPC client** to all six domain services. It holds the generated
client stubs for: `identity.v1`, `poll.v1`, `surprise.v1`, `catalog.v1`,
`notification.v1`. It does not talk to any database or the bus directly.

### 3.3 Events

None. The gateway neither publishes nor consumes NATS events.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose | Failure behavior |
|---|---|---|---|---|
| Identity Service | out | gRPC | Auth routes; **JWKS public key** for local JWT verify | If JWKS fetch fails, serve last-known key; fail closed on expiry |
| Poll Service | out | gRPC | Poll CRUD + anonymous token routes | 5xx → surface `502`; respect Poll's own rate limits |
| Surprise Service | out | gRPC | Generation lifecycle | `202` accept even if worker is busy (async) |
| Catalog Service | out | gRPC | Reference data reads | Cache-friendly; degrade to cached response |
| Notification Service | out | gRPC | Device registration | Non-critical; best-effort |
| Clients (iOS/Android/Web) | in | HTTPS/JSON | The public API | — |

**Auth flow the gateway enforces:** it validates JWTs **locally** using Identity's
rotating public key (fetched from a JWKS endpoint, cached). No per-request call to
Identity on the hot path. Anonymous Subject routes (`/polls/token/...`) bypass JWT
validation entirely — the Poll Service validates the opaque link token itself.

---

## 5. Cross-cutting responsibilities owned here

- **Rate limiting:** global, per-user (by JWT subject), and per-IP (critical for the
  anonymous poll routes). Counters in Valkey.
- **CORS:** allow the Poll Web Page origin on `/v1/polls/token/*` routes only.
- **Observability:** the gateway is the **root span** for OpenTelemetry traces — it
  injects/propagates trace context into every downstream gRPC call so a generation can be
  traced gateway → Surprise → Claude. Emits Prometheus metrics (RPS, latency, status
  codes per route).
- **Request shaping (BFF):** may aggregate e.g. `GET /v1/generations/{id}` from
  `GetGenerationStatus` + `GetIdeas` into one mobile-friendly response.

---

## 6. Tech stack & build notes

- **Language:** Go (house style).
- **Edge:** HTTP/JSON server (e.g. `net/http` + chi/echo) with generated **OpenAPI/Swagger**.
- **Internal:** gRPC client stubs generated from the `.proto` files owned by each service.
- **Config:** per-service env vars — downstream service addresses, JWKS URL, rate-limit
  budgets, CORS origins, TLS certs (or delegate TLS to the platform ingress).
- **Deploy:** stateless container; horizontally scalable; behind managed ingress/LB.
  `docker-compose` in dev, Cloud Run / k8s in prod.

## 7. Non-functional targets

- Edge latency overhead **< 20 ms** p95 (excluding downstream work).
- Must never block on generation — always return `202` fast.
- Fail closed on auth (reject on unverifiable JWT), fail soft on reference data
  (serve cached Catalog reads when Catalog is degraded).
