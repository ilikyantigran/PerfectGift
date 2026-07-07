# Android App — Client Specification

> **Source of truth for building this client from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this client is for

The Android app is the **User's (planner's) primary client** on Android — functionally
identical to the iOS app. The User signs in, describes an occasion (holiday, budget,
free-form preferences), optionally creates a poll for their partner, and receives **ranked
surprise ideas**. It can also host the **Subject** flow when the phone is **handed over**
(native poll rendering instead of the web page).

Generation is slow (~3–15 s), so the defining pattern is **submit-then-observe**: fire the
request, show progress, then poll status *or* receive a push, and render ideas when ready.

**In scope**
- Sign in with **Google / Apple** (+ email fallback); token stored in **DataStore**.
- Occasion input → generation → ideas screen.
- Poll creation + share link; native poll rendering for the handed-over-phone case.
- Save/favorite; refine/regenerate.
- Register for push (FCM) and handle deep links.

**Not in scope**
- Business logic or persistence beyond the auth token + in-memory state (**no local DB**).
- Talking to any backend service directly — **only** the API Gateway over REST/JSON.

---

## 2. Architecture & stack

- **UI:** Kotlin + **Jetpack Compose**, **MVVM**.
- **Networking:** **Retrofit/OkHttp** + **Coroutines/Flow** against the REST gateway.
- **Persistence:** **DataStore** for the auth (refresh) token **only**; all other state is
  in-memory. No Room / no offline store.
- **State management:** local/per-screen via ViewModels + Flow. No shared global store.
- **Push:** **FCM**; register the device token with the backend on launch/sign-in.
- **Deep links:** **App Links** so a shared poll link opens the app when installed, else
  falls back to the Poll Web Page.

---

## 3. Contracts — consumes the Gateway REST API

A **pure client of the API Gateway**; no gRPC, no knowledge of internal services. Key
endpoints (full list in the gateway spec):

| Flow | Calls |
|---|---|
| Auth | `POST /v1/auth/signin`, `POST /v1/auth/refresh`, `POST /v1/auth/revoke`, `GET /v1/me` |
| Generate | `POST /v1/generations` → `202 {requestId}`; poll `GET /v1/generations/{id}` until `ready`; `POST /v1/generations/{id}/refine` |
| Ideas | render ranked ideas from `GET /v1/generations/{id}`; `POST /v1/ideas/{id}/save` |
| Poll (owner) | `POST /v1/polls`, `GET /v1/polls/{id}/responses` |
| Poll (handed-over phone / Subject) | `GET /v1/polls/token/{t}`, `POST /v1/polls/token/{t}/responses` |
| Reference | `GET /v1/holidays`, `GET /v1/categories` |
| Push | `POST /v1/devices` (register FCM token) |

**Conventions the client must honor**
- Send `Authorization: Bearer <access_token>` on authenticated routes; refresh via
  `/v1/auth/refresh` on `401` and retry once (OkHttp authenticator/interceptor).
- Generation is **async**: never block the UI; advance on push or status polling.
- JSON is snake_case; errors follow `{ error: { code, message, details } }`.
- Send an `Idempotency-Key` on `POST /v1/generations` so retries are safe.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose |
|---|---|---|---|
| API Gateway | out | HTTPS/JSON | The only backend contact |
| Google Sign-In | out | Native SDK | Obtain Google ID token → `/v1/auth/signin` |
| Sign in with Apple | out | Web/SDK flow | Obtain Apple ID token → `/v1/auth/signin` |
| FCM | in | Push | "Poll done" / "Ideas ready" notifications |
| DataStore | local | — | Store the auth token |
| App Links | in | — | Deep-link a shared poll into the app |

---

## 5. Key UX flows

- **Generation (submit-then-observe):** input → `POST /v1/generations` → progress → push or
  poll → ideas screen. Handle `failed` with a graceful "try again".
- **Two-sided poll:** create poll → share link (or hand over phone) → notified when the
  Subject completes → responses sharpen the next generation.

---

## 6. Build notes

- **Build order:** the mobile client is the MVP's **step 3** (pick iOS *or* Android first).
  The second client is **step 8**. Keep parity with iOS on flows and the gateway contract.
- **Config:** gateway base URL per build variant; Google/Apple client IDs; FCM setup.
- **Testing:** mock the gateway against the OpenAPI/Swagger contract; do not couple to
  internal service shapes.

## 7. Non-functional targets

- UI interactions **< 300 ms**; generation shows progress, never a frozen UI.
- No PII persisted beyond the DataStore token.
- Graceful degradation on network/LLM failure, never a dead-end.
