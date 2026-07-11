package logkit

import (
	"log/slog"
	"time"
)

// Record is the unit that is shipped to, stored by, and displayed from the
// central log-server. Its JSON encoding is a SHARED CONTRACT: the server side
// mirrors these exact tags. Do not rename or re-tag fields without changing the
// server in lockstep.
//
//	{
//	  "ts":"2026-07-10T18:34:53.123456Z",
//	  "level":"INFO",
//	  "service":"identity",
//	  "message":"gRPC listening",
//	  "trace_id":"0af7651916cd43dd8448eb211c80319c",
//	  "span_id":"b7ad6b7169203331",
//	  "fields":{"addr":":9090"}
//	}
type Record struct {
	// Ts is RFC3339 with fractional (microsecond) seconds, always UTC.
	Ts string `json:"ts"`
	// Level is one of DEBUG|INFO|WARN|ERROR (uppercase).
	Level string `json:"level"`
	// Service is the emitting service name.
	Service string `json:"service"`
	// Message is the human-readable log message.
	Message string `json:"message"`
	// TraceID is 32 lowercase hex chars, or "" when absent.
	TraceID string `json:"trace_id"`
	// SpanID is 16 lowercase hex chars, or "" when absent.
	SpanID string `json:"span_id"`
	// Fields holds the extra structured slog attributes. Never nil on the wire
	// (encodes as {} when empty).
	Fields map[string]any `json:"fields"`
}

// tsLayout is RFC3339 with 6 fractional digits. When formatted against a UTC
// time the "Z07:00" segment collapses to "Z", matching the contract example.
const tsLayout = "2006-01-02T15:04:05.000000Z07:00"

// formatTS renders t as the contract timestamp (UTC, microsecond precision).
func formatTS(t time.Time) string {
	return t.UTC().Format(tsLayout)
}

// levelString maps an slog.Level onto the contract's uppercase level set,
// bucketing any custom level to the nearest standard one.
func levelString(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}
