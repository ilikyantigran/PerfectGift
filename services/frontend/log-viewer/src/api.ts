// Network module: the only place that talks to the log-server.
//
// It consumes exactly two same-origin routes:
//   GET /api/logs?service=&level=&q=&from=&to=&limit=&after=  -> { logs: [...] }
//   GET /api/services                                         -> { services: [...] }
//
// A `fetch` implementation is injectable so this module is fully unit-testable
// against a fake, with no live backend.

import { LOG_SERVER_URL } from "./config";
import type { LogQuery, LogRow } from "./types";

export interface ApiOptions {
  baseUrl?: string;
  fetchImpl?: typeof fetch;
}

/** Any failure talking to the log-server (network error, non-2xx, bad body). */
export class LogApiError extends Error {
  readonly status?: number;
  constructor(message = "Could not reach the log server.", status?: number) {
    super(message);
    this.name = "LogApiError";
    this.status = status;
  }
}

// --- wire shapes (exactly what the log-server returns) ---

interface WireLogs {
  logs?: LogRow[];
}

interface WireServices {
  services?: string[];
}

function resolve(options?: ApiOptions) {
  return {
    // Note: baseUrl may legitimately be "" (same-origin in production).
    baseUrl: (options?.baseUrl ?? LOG_SERVER_URL).replace(/\/+$/, ""),
    doFetch: options?.fetchImpl ?? fetch,
  };
}

/**
 * Build the query string for GET /api/logs from a LogQuery.
 *
 * Only set fields are emitted, so "All" filters (undefined) simply disappear.
 * The `q` glob string is passed through UNTOUCHED — the server does the matching,
 * we never escape or rewrite the '*' wildcards. Returns the string WITHOUT a
 * leading "?"; empty when no filters are set.
 */
export function buildLogsQuery(query: LogQuery = {}): string {
  const params = new URLSearchParams();
  if (query.service) params.set("service", query.service);
  if (query.level) params.set("level", query.level);
  if (query.q) params.set("q", query.q); // raw glob, untouched
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.after !== undefined) params.set("after", String(query.after));
  return params.toString();
}

/** Full path (base + /api/logs + query) for a given filter set. */
export function logsUrl(baseUrl: string, query: LogQuery = {}): string {
  const qs = buildLogsQuery(query);
  const base = baseUrl.replace(/\/+$/, "");
  return qs ? `${base}/api/logs?${qs}` : `${base}/api/logs`;
}

/**
 * Fetch log rows matching the given filters. Newest-first, as the server sends.
 * @throws LogApiError on network failure, non-2xx, or an unreadable body.
 */
export async function fetchLogs(
  query: LogQuery = {},
  options?: ApiOptions,
): Promise<LogRow[]> {
  const { baseUrl, doFetch } = resolve(options);
  const url = logsUrl(baseUrl, query);

  let res: Response;
  try {
    res = await doFetch(url, { method: "GET", headers: { Accept: "application/json" } });
  } catch (e) {
    throw new LogApiError(e instanceof Error ? e.message : "Network request failed.");
  }

  if (!res.ok) throw new LogApiError(`Request failed with status ${res.status}.`, res.status);

  let body: WireLogs;
  try {
    body = (await res.json()) as WireLogs;
  } catch {
    throw new LogApiError("The log server sent an unreadable response.");
  }

  return body.logs ?? [];
}

/**
 * Fetch the list of known service names for the filter dropdown.
 * @throws LogApiError on network failure, non-2xx, or an unreadable body.
 */
export async function fetchServices(options?: ApiOptions): Promise<string[]> {
  const { baseUrl, doFetch } = resolve(options);
  const url = `${baseUrl.replace(/\/+$/, "")}/api/services`;

  let res: Response;
  try {
    res = await doFetch(url, { method: "GET", headers: { Accept: "application/json" } });
  } catch (e) {
    throw new LogApiError(e instanceof Error ? e.message : "Network request failed.");
  }

  if (!res.ok) throw new LogApiError(`Request failed with status ${res.status}.`, res.status);

  let body: WireServices;
  try {
    body = (await res.json()) as WireServices;
  } catch {
    throw new LogApiError("The log server sent an unreadable response.");
  }

  return body.services ?? [];
}
