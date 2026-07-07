# PerfectGift — Services

This directory holds the **per-service specifications**. Each folder contains one
`SERVICE.md` that is the **self-contained source of truth for building that service from
scratch**: what it's for, its contracts (gRPC / REST / events / schema), and its required
integrations. Future agents should read the relevant `SERVICE.md` before building or
changing a service, alongside the root [`architecture.md`](../architecture.md).

## Backend (Go · gRPC internal · HTTP/Swagger edge · Postgres · Valkey · NATS)

| Service | Folder | One-liner |
|---|---|---|
| API Gateway / BFF | [`backend/api-gateway`](backend/api-gateway/SERVICE.md) | Public REST edge; JWT, rate limits, REST⇄gRPC, Swagger. Stateless. |
| Identity | [`backend/identity`](backend/identity/SERVICE.md) | Who the User is: sign-in, JWT issue/validate, sessions. |
| Poll | [`backend/poll`](backend/poll/SERVICE.md) | Anonymous, link-scoped two-sided poll flow. The only public/anonymous surface. |
| Surprise | [`backend/surprise`](backend/surprise/SERVICE.md) | The heart: async LLM idea generation with grounding. |
| Catalog | [`backend/catalog`](backend/catalog/SERVICE.md) | Read-mostly reference data + curated pgvector grounding corpus. |
| Notification | [`backend/notification`](backend/notification/SERVICE.md) | Async APNs/FCM push fan-out with outbox + retries. |

## Frontend (three clients, one API — all speak only to the gateway)

| Client | Folder | One-liner |
|---|---|---|
| iOS App | [`frontend/ios-app`](frontend/ios-app/SERVICE.md) | SwiftUI/MVVM planner client; submit-then-observe. |
| Android App | [`frontend/android-app`](frontend/android-app/SERVICE.md) | Kotlin/Compose planner client; parity with iOS. |
| Poll Web Page | [`frontend/poll-web`](frontend/poll-web/SERVICE.md) | Tiny static SPA (S3+CDN) for the Subject's link-in-browser poll. |

## How the pieces connect (see `architecture.md` §4 for the diagram)

- Clients → **API Gateway** (REST/JSON) → domain services (**gRPC**).
- Async over **NATS JetStream**: `GenerationRequested` (durable job for Surprise workers),
  `PollCompleted` + `IdeasReady` (events → Notification).
- External: **Surprise → Anthropic Claude**; **Notification → APNs/FCM**.
- Every service owns its **own** Postgres schema — **no cross-service DB access**; services
  read each other's data only via gRPC/events.

## Build order (from `architecture.md` §12)

1. Identity (+ gateway auth + Swagger) → 2. Surprise (sync-ish, get quality right) →
3. one mobile client → 4. make generation async (NATS) → 5. Poll + poll web page →
6. Catalog + pgvector grounding → 7. Notification → 8. second client, save/refine →
9. moderation, observability, cost metrics.
