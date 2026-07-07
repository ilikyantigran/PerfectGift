# iOS App — Client Specification

> **Source of truth for building this client from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this client is for

The iOS app is the **User's (planner's) primary client**. The User signs in, describes an
occasion (holiday, budget, free-form preferences), optionally creates a poll for their
partner, and receives **ranked surprise ideas**. It can also host the **Subject** flow when
the User simply **hands over the phone** (the same poll rendered natively instead of the web
page).

Because generation is inherently slow (~3–15 s), the app's defining pattern is
**submit-then-observe**: fire the request, show a friendly progress state, then poll for
status *or* receive a push, and render ideas when ready.

**In scope**
- Sign in with **Apple / Google** (+ email fallback), token storage in **Keychain**.
- Occasion input screen → generation → ideas screen.
- Poll creation + share link; native poll rendering for the handed-over-phone case.
- Save/favorite ideas; refine/regenerate.
- Register for push (APNs) and deep-link handling.

**Not in scope**
- Any business logic or persistence beyond the auth token + in-memory view state (per the
  brief — **no local database**).
- Talking to any backend service directly — **only** the API Gateway over REST/JSON.

---

## 2. Architecture & stack

- **UI:** SwiftUI, **MVVM**.
- **Concurrency/networking:** `async/await` + `URLSession` against the REST gateway.
- **Persistence:** **Keychain** for the auth (refresh) token **only**; everything else is
  in-memory view state. No Core Data / no offline store.
- **State management:** local/per-screen. No shared global store (not worth the complexity).
- **Push:** APNs; register the device token with the backend on launch/sign-in.
- **Deep links:** **Universal Links** so a shared poll link opens the app when installed,
  else falls back to the Poll Web Page.

---

## 3. Contracts — consumes the Gateway REST API

The app is a **pure client of the API Gateway**. It holds no gRPC stubs and knows nothing of
internal services. Key endpoints (see the gateway spec for the full list):

| Flow | Calls |
|---|---|
| Auth | `POST /v1/auth/signin`, `POST /v1/auth/refresh`, `POST /v1/auth/revoke`, `GET /v1/me` |
| Generate | `POST /v1/generations` → `202 {requestId}`; poll `GET /v1/generations/{id}` until `ready`; `POST /v1/generations/{id}/refine` |
| Ideas | render ranked ideas from `GET /v1/generations/{id}`; `POST /v1/ideas/{id}/save` |
| Poll (owner) | `POST /v1/polls`, `GET /v1/polls/{id}/responses` |
| Poll (handed-over phone / Subject) | `GET /v1/polls/token/{t}`, `POST /v1/polls/token/{t}/responses` |
| Reference | `GET /v1/holidays`, `GET /v1/categories` |
| Push | `POST /v1/devices` (register APNs token) |

**Conventions the client must honor**
- Send `Authorization: Bearer <access_token>` on authenticated routes; transparently
  refresh via `/v1/auth/refresh` on `401` and retry once.
- Generation is **async**: never block the UI waiting on one request. Show progress; use
  push arrival or status polling to advance.
- JSON is snake_case; errors follow the `{ error: { code, message, details } }` envelope.
- Send an `Idempotency-Key` on `POST /v1/generations` (e.g. per user submit) to make
  retries safe.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose |
|---|---|---|---|
| API Gateway | out | HTTPS/JSON | The only backend contact |
| Sign in with Apple | out | Native SDK | Obtain Apple ID token → `/v1/auth/signin` |
| Google Sign-In | out | Native SDK | Obtain Google ID token → `/v1/auth/signin` |
| APNs | in | Push | "Poll done" / "Ideas ready" notifications |
| Keychain | local | — | Store the auth token securely |
| Universal Links | in | — | Deep-link a shared poll into the app |

---

## 5. Key UX flows

- **Generation (submit-then-observe):** input → `POST /v1/generations` → progress state →
  push or poll → ideas screen. Handle the `failed` status with a graceful "try again".
- **Two-sided poll:** create poll → share link (or hand over phone) → later, notified when
  the Subject completes → responses sharpen the next generation.

---

## 6. Build notes

- **Build order:** the app is the MVP's **step 3** — pick iOS *or* Android first; input
  screen → ideas screen. The second client comes later (step 8).
- **Config:** gateway base URL per environment; Apple/Google client IDs; APNs entitlement.
- **Testing:** mock the gateway with the OpenAPI/Swagger contract; the app must not depend
  on internal service shapes.

## 7. Non-functional targets

- UI interactions **< 300 ms**; generation shows progress, never a frozen UI.
- No PII persisted on device beyond the Keychain token.
- Graceful degradation on network/LLM failure ("try again"), never a dead-end.
