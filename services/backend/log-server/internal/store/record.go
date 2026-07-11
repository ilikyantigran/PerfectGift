package store

import "encoding/json"

// Record is the shared log-record contract. The client library and web UI
// depend on these exact JSON field names.
type Record struct {
	TS      string          `json:"ts"`                 // RFC3339 fractional seconds, UTC
	Level   string          `json:"level"`              // DEBUG|INFO|WARN|ERROR (uppercase)
	Service string          `json:"service"`            // originating service
	Message string          `json:"message"`            // log message
	TraceID string          `json:"trace_id"`           // 32 hex or ""
	SpanID  string          `json:"span_id"`            // 16 hex or ""
	Fields  json.RawMessage `json:"fields,omitempty"`   // object; may be {}
}

// LogRow is a stored Record plus the server-assigned monotonic ingest id.
type LogRow struct {
	Record
	ID int64 `json:"id"`
}

// Query holds the optional filters for GET /api/logs.
type Query struct {
	Service string
	Level   string
	Q       string // message search; '*' = wildcard, else substring; case-insensitive
	From    string // inclusive RFC3339 lower bound on ts
	To      string // inclusive RFC3339 upper bound on ts
	Limit   int    // clamped: default 200, max 1000
	After   int64  // return only rows with id > After (live-tail cursor)
}
