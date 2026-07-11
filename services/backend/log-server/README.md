# log-server

Ingests structured logs from PerfectGift's services, stores them in SQLite
(pure-Go, CGO-free), and serves a query API plus the log-viewer web UI.

## Run locally

```bash
# from services/backend/log-server
go run ./cmd/log-server            # uses ./configs/values_local.yaml (DB at ./data/logs.db)
# or point at any config:
CONFIG_PATH=./configs/values_local.yaml go run ./cmd/log-server
```

Listens on `http://localhost:8086`.

## Test / build

```bash
go build ./...
go test ./...
```

Tests are hermetic (temp-file SQLite, no network).

## Config

Selected by the `CONFIG_PATH` env var (defaults to `./configs/values_local.yaml`).
`configs/values_local.yaml` and `configs/values_docker.yaml` share the same keys.

| key                    | default (docker)  | meaning                                   |
|------------------------|-------------------|-------------------------------------------|
| `log_service.http_port`| `8086`            | HTTP port for the API + UI                |
| `store.path`           | `/data/logs.db`   | SQLite file path (put on a Docker volume) |
| `retention.window`     | `72h`             | delete rows older than this               |
| `retention.interval`   | `10m`             | how often the pruner sweeps               |

## Endpoints

Log Record JSON:

```json
{
  "ts": "2026-07-10T18:34:53.123456Z",
  "level": "INFO",
  "service": "identity",
  "message": "gRPC listening",
  "trace_id": "0af7651916cd43dd8448eb211c80319c",
  "span_id": "b7ad6b7169203331",
  "fields": {"addr": ":9090"}
}
```

- **POST `/api/ingest`** — body `{"records":[<Record>,...]}`. Stores all rows and
  responds `200 {"accepted":<int>}`. Lenient: missing `level` defaults to `INFO`,
  missing `ts` defaults to now, missing `fields` defaults to `{}`. Hot path.
- **GET `/api/logs`** — all params optional. Returns
  `{"logs":[<LogRow>,...]}` newest-first (highest `id` first). `LogRow` = Record +
  `"id":<int64>` (server-assigned monotonic ingest id).
  - `service`, `level` — exact-match filters
  - `q` — message search, case-insensitive. Contains `*` ⇒ wildcard
    (`*auth problem:*` ⇒ contains "auth problem:"). No `*` ⇒ substring. Literal
    `%` and `_` in the query are matched literally.
  - `from`, `to` — inclusive RFC3339 range on `ts`
  - `limit` — default 200, max 1000
  - `after` — only rows with `id > after` (live-tail cursor)
- **GET `/api/services`** — `{"services":[...]}` distinct service names.
- **GET `/healthz`** — `200 ok`.
- **GET `/metrics`** — Prometheus (`logserver_ingested_records_total`,
  `logserver_pruned_records_total`, plus Go runtime).
- **`/` and any non-`/api` path** — serves the embedded SPA, falling back to
  `index.html` for client-side routes.

## Where the UI dist goes

The React UI is built separately in `services/frontend/log-viewer`. Its build
output (`dist/`) is embedded here via `go:embed all:dist` in
`internal/web/web.go`. During integration, **replace the contents of
`internal/web/dist/`** (currently a placeholder `index.html`) with that project's
`dist/` output and rebuild. No code changes needed — the embed picks up whatever
lives in `internal/web/dist/`.

## Store

- `modernc.org/sqlite` (pure Go → builds with `CGO_ENABLED=0`).
- Opened with WAL journal + `busy_timeout` so ingest doesn't block readers.
- Table `logs(id, ts, level, service, message, trace_id, span_id, fields)` with
  indexes on `ts`, `service`, `level`, `trace_id` (`id` is the primary key).
  `fields` is stored as JSON text.
- A background pruner deletes rows older than `retention.window` every
  `retention.interval`.

## Docker

```bash
docker build -t perfectgift/log-server .
docker run -p 8086:8086 -v log-data:/data perfectgift/log-server
```

Build stage uses `mirror.gcr.io/library/golang:1.26-alpine` (Docker Hub is
rate-limited on the build host); the binary is built `CGO_ENABLED=0` and runs on
a distroless nonroot base. The DB lives at `/data/logs.db` (declared as a
`VOLUME`) so logs survive restarts.
