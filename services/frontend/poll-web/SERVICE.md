# Poll Web Page — Client Specification

> **Source of truth for building this client from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this client is for

The Poll Web Page is a **tiny, static SPA** that exists for one reason: a shared **poll
link** is opened in a **mobile browser** by the **Subject** (the partner), who probably does
**not** have the app. It lets that anonymous Subject fetch a poll by token, answer the
questions, and submit — **no accounts, no sign-in, nothing else.**

It is intentionally minimal and deliberately **not** the native app: the native apps handle
the handed-over-phone case; this page handles the link-in-a-browser case. Native apps use
**universal/app links** so an installed app intercepts the link first; this page is the
fallback.

**In scope**
- Resolve a poll from the **opaque link token** in the URL.
- Render the poll's questions.
- Submit the Subject's answers.
- A thank-you screen. That's the entire surface.

**Not in scope**
- Accounts, auth, routing beyond the single poll, any User-side features.
- Reading responses, generation, notifications, or any authenticated data.
- Server-side rendering (there is **no SEO need** — it's a private, token-gated form).

---

## 2. Architecture & stack

- **Framework:** a tiny SPA — **Svelte** or **plain React**, static build.
- **Rendering:** **static, client-rendered SPA** (no SSR). Single route: the poll.
- **Hosting:** static assets on **S3 + CDN** (a CDN bucket — no server to run).
- **State:** local/in-component only. No global store, no persistence, no cookies beyond
  what the anonymous poll session requires.

---

## 3. Contracts — consumes two public Gateway routes only

The page talks **only** to the API Gateway, and only to the **anonymous, token-scoped**
routes. It never sends a JWT.

| Step | Call | Notes |
|---|---|---|
| Load poll | `GET /v1/polls/token/{t}` | `{t}` = opaque token from the shared URL. Returns `{ poll_id, title, questions }`. 404 uniformly if expired/invalid/revoked. |
| Submit | `POST /v1/polls/token/{t}/responses` | Body `{ answers }`. **Rate-limited** (per token/IP by the Poll Service). Returns `{ ok }` → thank-you screen. |

**Conventions**
- **No `Authorization` header** — the Poll Service validates the opaque token itself.
- Requires **CORS** to be enabled for this page's origin on exactly these two routes.
- JSON snake_case; errors follow `{ error: { code, message, details } }`. Treat a `404`/
  expired token and a `429` (rate-limited) with clear, friendly messaging.
- Expect the token to be **expiring** — handle the "this link is no longer available" case
  gracefully.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose |
|---|---|---|---|
| API Gateway (anonymous poll routes) | out | HTTPS/JSON | Fetch + submit the poll |
| S3 + CDN | host | — | Serve the static SPA |
| Universal/App Links (native apps) | related | — | Installed app intercepts the link first; this page is the fallback |

Downstream (not called directly, for context): the gateway routes to the **Poll Service**,
which validates the token, stores the response, and emits **`PollCompleted`** — which the
Notification Service turns into a "your partner finished the poll" push to the User.

---

## 5. Privacy & abuse notes

- The Subject's answers are **personal and sensitive**; the page collects only what the poll
  asks and transmits it over HTTPS.
- The page must not leak any **owner/User** information — `GetPollByToken` deliberately
  returns only the questions, never who created the poll or their data.
- Because this rides the only unauthenticated public surface, the **Poll Service** enforces
  aggressive rate limiting; the page should surface `429`s calmly and not auto-retry hard.

---

## 6. Build notes

- **Build order:** ships **with the Poll Service at step 5** as the two-sided differentiating
  feature.
- **Config:** gateway base URL per environment; CDN/bucket target.
- **Keep it small:** no router library beyond token handling, no state manager, no auth
  stack. Its smallness *is* the design — it keeps hosting to a CDN bucket and the attack
  surface tiny.

## 7. Non-functional targets

- First load fast on mobile networks (static, CDN-served, minimal JS).
- Works on a plain mobile browser with no app installed.
- Clear terminal states: submitted (thank-you), expired link, rate-limited.
