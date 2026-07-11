# logkit

A best-effort, resilient logging client for PerfectGift Go microservices.

`logkit` provides an `slog.Handler` that behaves **exactly** like each service's
current telemetry handler — always writing a trace-correlated JSON line to
stdout (so `docker compose logs` keeps working) — while **also** shipping every
log record to the central log-server.

Shipping is strictly non-mandatory:

- The app **never blocks or fails** because of logging.
- Records flow through a bounded in-memory queue → a batcher → `POST /api/ingest`.
- On any send failure, records are **spooled to disk** and **backfilled** once
  the server returns. Nothing is lost.
- If `LOG_SERVER_URL` is empty, shipping is disabled and logkit is a pure
  stdout logger.

## Install it in a service (one line)

Replace the logger setup in `telemetry.Setup` (the `slog.SetDefault(...)` call
around the existing `contextHandler`) with:

```go
import "github.com/ilikyantigran/PerfectGift/pkg/logkit"

flush := logkit.Install("identity") // service name
defer flush(context.Background())    // drains + backfills on shutdown
```

`Install` reads configuration from the environment, sets the default `slog`
logger, and returns a `flush` function to call on shutdown. Trace correlation is
automatic: `trace_id`/`span_id` are pulled from the context via
`trace.SpanContextFromContext`, identical to the old `contextHandler`.

Use context-aware logging so trace IDs propagate:

```go
slog.InfoContext(ctx, "gRPC listening", "addr", ":9090")
```

### Explicit options (instead of env)

```go
h := logkit.NewHandler("identity", logkit.Options{
    ServerURL: "http://log-server:8080",
    BatchSize: 200,
})
slog.SetDefault(slog.New(h))
defer h.Close(context.Background())
```

Any left-zero `Options` field is backfilled with its default.

## Configuration (environment)

| Env var          | Default          | Meaning                                            |
|------------------|------------------|----------------------------------------------------|
| `LOG_SERVER_URL` | *(empty)*        | Log-server base URL. **Empty ⇒ stdout-only.**      |
| `LOG_SPOOL_DIR`  | `/var/log/spool` | Directory for the on-disk store-and-forward spool. |
| `LOG_BATCH_SIZE` | `100`            | Records per batch.                                 |
| `LOG_FLUSH_MS`   | `2000`           | Flush a partial batch after at most this long.     |
| `LOG_QUEUE_SIZE` | `4096`           | In-memory queue depth (full ⇒ drop, never block).  |
| `LOG_HTTP_MS`    | `5000`           | Per-request timeout for `POST /api/ingest`.        |
| `LOG_RETRY_MS`   | `10000`          | How often the background loop retries the spool.   |
| `LOG_SPOOL_MAX`  | `52428800`       | Spool cap in bytes; oldest lines dropped beyond it.|

## Shared contract

Each shipped record (`logkit.Record`) is JSON of the form:

```json
{
  "ts": "2026-07-10T18:34:53.123456Z",
  "level": "INFO",
  "service": "identity",
  "message": "gRPC listening",
  "trace_id": "0af7651916cd43dd8448eb211c80319c",
  "span_id": "b7ad6b7169203331",
  "fields": { "addr": ":9090" }
}
```

Batches are delivered as `POST {LOG_SERVER_URL}/api/ingest` with body
`{"records":[<Record>, ...]}`. Any non-2xx or transport error is treated as
"not delivered" → spooled and retried. The spool is a JSON-lines file at
`{LOG_SPOOL_DIR}/{service}.jsonl`.

## Public API

```go
func Install(serviceName string) (flush func(context.Context))
func NewHandler(serviceName string, opts Options) *Handler
func OptionsFromEnv() Options

type Handler struct{ /* implements slog.Handler */ }
func (h *Handler) Handle(ctx context.Context, r slog.Record) error
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler
func (h *Handler) WithGroup(name string) slog.Handler
func (h *Handler) Flush(ctx context.Context) // drain + send + backfill, keep running
func (h *Handler) Close(ctx context.Context) // final drain + stop

type Options struct { /* see table above; plus Stdout, HTTPClient for testing */ }
type Record  struct { Ts, Level, Service, Message, TraceID, SpanID string; Fields map[string]any }
```

## Test

```
go build ./...
go test ./...
```

Tests are hermetic: an `httptest.Server` stands in for `/api/ingest` and a
`t.TempDir()` holds the spool. No real network, no flaky sleeps.
