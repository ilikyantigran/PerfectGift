// Trace helpers: turn a trace_id into a Jaeger deep-link and a short display form.

// Where the Jaeger UI lives in the local stack. A trace opens at
// {JAEGER_BASE_URL}/trace/{trace_id}.
export const JAEGER_BASE_URL = "http://localhost:16686";

/**
 * Build the Jaeger deep-link for a trace_id.
 * Returns `null` for an empty/blank trace_id (nothing to link to).
 */
export function jaegerTraceUrl(traceId: string | undefined | null): string | null {
  const id = (traceId ?? "").trim();
  if (!id) return null;
  return `${JAEGER_BASE_URL}/trace/${id}`;
}

/**
 * Short display form of a trace_id: first `head` chars followed by an ellipsis.
 * Empty/blank ids render as "—". Ids already short enough are shown whole.
 */
export function shortTraceId(traceId: string | undefined | null, head = 8): string {
  const id = (traceId ?? "").trim();
  if (!id) return "—";
  return id.length > head ? `${id.slice(0, head)}…` : id;
}
