# Poll Web Page

A tiny, static single-page app for the **PerfectGift** project. A Subject (the partner)
opens a shared **poll link** in a mobile browser, answers the questions, and submits.
No accounts, no sign-in, no routing beyond the single poll — its smallness is the design.

See [`SERVICE.md`](./SERVICE.md) for the full client spec.

## What it does

1. Reads the opaque poll **token** from the URL.
2. Calls the API Gateway to load the poll (`GET /v1/polls/token/{t}`).
3. Renders the questions (free text, single-choice, multi-choice).
4. Submits the answers (`POST /v1/polls/token/{t}/responses`).
5. Shows a thank-you screen.

It talks **only** to those two anonymous, token-scoped Gateway routes and never sends an
`Authorization` header — the Poll Service validates the token itself.

### Terminal states handled

| State | Trigger | What the Subject sees |
|---|---|---|
| Thank-you | Submit succeeds (`{ ok: true }`) | "Thank you! Your answers were sent." |
| Link unavailable | `404` (expired / invalid / revoked / already used) | "This link is no longer available." |
| Rate-limited | `429` | "Just a moment… please wait, then reopen." (no hard auto-retry) |
| No token | URL has no token | "This page needs a poll link." |
| Generic error | 5xx / network / malformed body | "Something went wrong." (submit errors keep the form so the Subject can retry) |

## Stack

Plain **React 18 + TypeScript**, built by **Vite** into static assets (CDN/S3-hostable).
No router library, no state manager, no auth. Tests run on **Vitest** with no live backend.

## Install

```bash
cd services/frontend/poll-web
npm install
```

## Develop

```bash
npm run dev            # Vite dev server, prints a local URL
```

Open the app with a token in the URL (see below).

## Build (production, static)

```bash
npm run build          # tsc --noEmit typecheck, then vite build -> dist/
npm run preview        # serve the built dist/ locally to sanity-check
```

The contents of `dist/` are what you upload to the S3 + CDN bucket. `base` is `./`
(relative), so the bundle works from any path on the CDN.

## Test

```bash
npm test               # vitest run — network module, token parser, answer logic
npm run typecheck      # tsc --noEmit only
```

The network layer lives in [`src/api.ts`](./src/api.ts) and accepts an injectable
`fetch`, so all four contract paths (load, submit, 404, 429) are unit-tested against a
fake with no backend running.

## Configuring the Gateway base URL

Set `VITE_GATEWAY_BASE_URL` at build/dev time. It defaults to `http://localhost:8080`
(the local Docker stack's gateway).

```bash
# one-off
VITE_GATEWAY_BASE_URL=https://api.perfectgift.example npm run build

# or via a .env file (see .env.example)
cp .env.example .env
```

**CORS:** the Gateway must allow this page's origin on the two `/v1/polls/token/*`
routes (it already scopes CORS to exactly those routes).

## What a token URL looks like

Any of these resolve the same token (query param wins if several are present):

```
https://poll.perfectgift.example/?t=OPAQUE_TOKEN
https://poll.perfectgift.example/p/OPAQUE_TOKEN
https://poll.perfectgift.example/#/OPAQUE_TOKEN
```

Locally during `npm run dev`, append `?t=YOUR_TOKEN` to the dev-server URL. Mint a token
by creating a poll through the Gateway (`POST /v1/polls`, JWT-authenticated) — the
response's `link_token` / `link_url` is what the Subject receives.

## Layout

```
poll-web/
├── index.html            # single HTML shell
├── package.json
├── tsconfig.json
├── vite.config.ts        # base "./", Vitest config
├── .env.example
├── SERVICE.md            # the client spec (source of truth)
├── README.md
└── src/
    ├── main.tsx          # React entry
    ├── App.tsx           # the state machine (load → form → thank-you / error states)
    ├── QuestionField.tsx # renders one question by type
    ├── api.ts            # network module — the only code that calls the Gateway
    ├── answers.ts        # pure: build wire payload + required-field validation
    ├── token.ts          # extract the opaque token from the URL
    ├── types.ts          # normalized domain types
    ├── config.ts         # Gateway base URL
    ├── styles.css
    ├── api.test.ts
    ├── token.test.ts
    └── answers.test.ts
```
