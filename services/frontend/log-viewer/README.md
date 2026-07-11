# Log Viewer

A small **React 18 + TypeScript + Vite** single-page app for the **PerfectGift**
project. It shows structured logs from every service in one browser page, with
filters, a live tail, expandable `fields`, and one-click **Jaeger** trace links.

The **log-server** serves this bundle at its own root and exposes the two JSON
routes the UI consumes.

## What it does

- **Log table** — columns: time · level · service · message · trace. Levels are
  color-coded (ERROR red, WARN amber, INFO neutral, DEBUG muted). Messages wrap
  in full; a row with structured `fields` expands to show them as pretty JSON.
- **Filter bar**
  - **Service** dropdown, populated from `GET /api/services`, plus **All**.
  - **Level** dropdown — All / ERROR / WARN / INFO / DEBUG.
  - **Time range** — presets **Last 15m / 1h / 24h / All** (computed client-side
    into `from`/`to`), plus an optional **Custom** range with two datetime
    inputs.
  - **Search** — message search that supports `*glob*` wildcards. The raw string
    is passed through as `q`; the server does the matching (e.g. `*auth problem:*`).
- **Trace cell** — `trace_id` shown truncated (first 8 chars…) with a
  **copy-to-clipboard** button and a link to
  `http://localhost:16686/trace/<trace_id>` (Jaeger, new tab). An empty
  `trace_id` renders as `—` with no link or copy button.
- **Live tail** — a toggle that polls `GET /api/logs?…&after=<maxIdSeen>` every
  ~2s and prepends new rows (respecting the current filters). When off, a manual
  **Refresh** button reloads.
- Sensible **loading / empty / error** states.

## API it consumes (the shared contract)

Same-origin JSON, served by the log-server:

- `GET /api/logs?service=&level=&q=&from=&to=&limit=&after=` → `{ "logs": [ <LogRow>, … ] }`
  (newest-first). `service`/`level` are exact filters; `q` is a `*`-glob message
  search; `from`/`to` are an RFC3339 range; `limit` defaults to 200 server-side;
  `after` returns only rows with `id > after` (the live-tail cursor).
- `GET /api/services` → `{ "services": [ … ] }` (the dropdown).

A `LogRow` is:

```json
{
  "id": 1234,
  "ts": "2026-07-10T18:34:53.123456Z",
  "level": "INFO",
  "service": "identity",
  "message": "gRPC listening",
  "trace_id": "0af7651916cd43dd8448eb211c80319c",
  "span_id": "b7ad6b7169203331",
  "fields": { "addr": ":9090" }
}
```

All network access lives in [`src/api.ts`](./src/api.ts) behind an injectable
`fetch`, so every path is unit-tested against a fake with no live backend.

## Stack

Plain React 18 + TypeScript, built by Vite into static assets. No router, no
state manager, no auth. Tests run on Vitest.

## Install

```bash
cd services/frontend/log-viewer
npm install
```

## Develop

```bash
npm run dev            # Vite dev server, prints a local URL
```

In dev the UI defaults to talking to the log-server at
`http://localhost:8086`. Override with `VITE_LOG_SERVER_URL` (see below).

## Build (production, static)

```bash
npm run build          # tsc --noEmit typecheck, then vite build -> dist/
npm run preview        # serve the built dist/ locally to sanity-check
```

`base` is `./` (relative), so the bundle works from any path. In production the
log-server serves the contents of `dist/` at its own root; because the UI then
calls the **same origin**, `VITE_LOG_SERVER_URL` should be left **unset** for a
production build (empty base ⇒ relative `/api/...` requests).

## Test

```bash
npm test               # vitest run
npm run typecheck      # tsc --noEmit only
```

Covered: the `/api/logs` query-string builder (every filter, `*glob*` passed
through untouched as `q`), the `{logs:[...]}` / `{services:[...]}` parsers, the
Jaeger link builder (and its empty-trace_id → none case), the live-tail cursor
(`next after = max id`), and the time-range preset math.

## Configuring the log-server base URL

`VITE_LOG_SERVER_URL` sets the base at build/dev time:

- **Dev** (`npm run dev`): defaults to `http://localhost:8086`.
- **Build**: defaults to **same-origin** (empty base) — correct when the
  log-server serves this UI.

```bash
# one-off (e.g. point dev at a remote log-server)
VITE_LOG_SERVER_URL=https://logs.perfectgift.example npm run dev

# or via a .env file
cp .env.example .env
```

## How it's served

The log-server hosts the built `dist/` as static files and answers `/api/logs`
and `/api/services` on the same origin. Build here, then point the log-server's
static-file root at this `dist/`.

## Layout

```
log-viewer/
├── index.html            # single HTML shell
├── package.json
├── tsconfig.json
├── vite.config.ts        # base "./", Vitest config
├── .env.example
├── README.md
└── src/
    ├── main.tsx          # React entry
    ├── App.tsx           # state + live-tail loop; wires the pieces together
    ├── FilterBar.tsx     # service/level/time/search + live/refresh controls
    ├── LogTable.tsx      # the table, row expansion, and the trace cell
    ├── api.ts            # network module — the only code that calls the log-server
    ├── trace.ts          # Jaeger link + short-id helpers
    ├── tail.ts           # live-tail cursor (nextAfter) + row merge
    ├── time.ts           # time-range preset -> {from,to}
    ├── types.ts          # LogRow / LogQuery / LogLevel
    ├── config.ts         # log-server base URL
    ├── styles.css
    ├── api.test.ts
    ├── trace.test.ts
    ├── tail.test.ts
    └── time.test.ts
```
