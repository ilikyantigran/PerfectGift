// Domain types for the log viewer. These mirror the SHARED CONTRACT exactly:
// the log-server returns snake_case JSON and we consume it as-is (no renaming),
// because the wire shape is already flat and friendly.

export type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";

// One row from GET /api/logs. Field names match the server's JSON verbatim.
export interface LogRow {
  id: number; // int64 monotonic ingest id; use max seen as the next `after`
  ts: string; // RFC3339, fractional seconds, UTC
  level: LogLevel;
  service: string;
  message: string;
  trace_id: string; // 32 hex, or "" when absent
  span_id: string; // 16 hex, or "" when absent
  fields: Record<string, unknown>; // object; may be {}
}

// The filters the UI turns into a /api/logs query string. Every field is
// optional; only the set ones are sent.
export interface LogQuery {
  service?: string; // exact match; omit for "All"
  level?: LogLevel; // exact match; omit for "All"
  q?: string; // message search; may contain '*' glob wildcards
  from?: string; // RFC3339 lower bound (inclusive)
  to?: string; // RFC3339 upper bound
  limit?: number; // default 200 server-side
  after?: number; // live-tail cursor: only rows with id > after
}
