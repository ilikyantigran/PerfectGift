import { describe, expect, it, vi } from "vitest";
import { buildLogsQuery, logsUrl, fetchLogs, fetchServices, LogApiError } from "./api";
import type { LogRow } from "./types";

const BASE = "http://logs.test";

// Build a fake fetch that returns a single canned Response.
function fakeFetch(res: Partial<Response> & { jsonBody?: unknown }) {
  const response = {
    ok: res.ok ?? true,
    status: res.status ?? 200,
    headers: res.headers ?? new Headers(),
    json: async () => res.jsonBody,
    ...res,
  } as unknown as Response;
  return vi.fn(async () => response) as unknown as typeof fetch;
}

const row = (over: Partial<LogRow> = {}): LogRow => ({
  id: 1,
  ts: "2026-07-10T18:34:53.123456Z",
  level: "INFO",
  service: "identity",
  message: "gRPC listening",
  trace_id: "",
  span_id: "",
  fields: {},
  ...over,
});

describe("buildLogsQuery", () => {
  it("emits only the set filters, omitting undefined/All fields", () => {
    const qs = buildLogsQuery({ service: "identity", level: "ERROR", limit: 50 });
    const p = new URLSearchParams(qs);
    expect(p.get("service")).toBe("identity");
    expect(p.get("level")).toBe("ERROR");
    expect(p.get("limit")).toBe("50");
    expect(p.has("q")).toBe(false);
    expect(p.has("from")).toBe(false);
    expect(p.has("to")).toBe(false);
    expect(p.has("after")).toBe(false);
  });

  it("builds the full query string from every filter", () => {
    const qs = buildLogsQuery({
      service: "poll",
      level: "WARN",
      q: "listening",
      from: "2026-07-10T00:00:00Z",
      to: "2026-07-10T23:59:59Z",
      limit: 200,
      after: 42,
    });
    const p = new URLSearchParams(qs);
    expect(p.get("service")).toBe("poll");
    expect(p.get("level")).toBe("WARN");
    expect(p.get("q")).toBe("listening");
    expect(p.get("from")).toBe("2026-07-10T00:00:00Z");
    expect(p.get("to")).toBe("2026-07-10T23:59:59Z");
    expect(p.get("limit")).toBe("200");
    expect(p.get("after")).toBe("42");
  });

  it("passes a *glob* query through untouched as q", () => {
    const glob = "*auth problem:*";
    const qs = buildLogsQuery({ q: glob });
    // Round-trips exactly — the wildcards and spaces survive unescaped.
    expect(new URLSearchParams(qs).get("q")).toBe(glob);
  });

  it("emits after=0 (a real cursor value) but skips it when undefined", () => {
    expect(new URLSearchParams(buildLogsQuery({ after: 0 })).get("after")).toBe("0");
    expect(new URLSearchParams(buildLogsQuery({})).has("after")).toBe(false);
  });

  it("returns an empty string when no filters are set", () => {
    expect(buildLogsQuery()).toBe("");
  });
});

describe("logsUrl", () => {
  it("joins base + /api/logs with a query", () => {
    expect(logsUrl(BASE, { service: "identity" })).toBe(`${BASE}/api/logs?service=identity`);
  });

  it("omits the ? when there are no filters and tolerates a same-origin empty base", () => {
    expect(logsUrl(BASE)).toBe(`${BASE}/api/logs`);
    expect(logsUrl("", { level: "ERROR" })).toBe(`/api/logs?level=ERROR`);
  });
});

describe("fetchLogs", () => {
  it("GETs /api/logs with the filter query and parses { logs: [...] }", async () => {
    const spy = fakeFetch({ jsonBody: { logs: [row({ id: 7 }), row({ id: 6 })] } });
    const logs = await fetchLogs({ service: "identity", level: "ERROR" }, { baseUrl: BASE, fetchImpl: spy });

    const [url, init] = (spy as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe(`${BASE}/api/logs?service=identity&level=ERROR`);
    expect((init as RequestInit).method).toBe("GET");
    expect(logs).toHaveLength(2);
    expect(logs[0].id).toBe(7);
  });

  it("returns [] when the body has no logs field", async () => {
    const spy = fakeFetch({ jsonBody: {} });
    await expect(fetchLogs({}, { baseUrl: BASE, fetchImpl: spy })).resolves.toEqual([]);
  });

  it("throws LogApiError on a non-2xx status", async () => {
    const spy = fakeFetch({ ok: false, status: 500 });
    const err = await fetchLogs({}, { baseUrl: BASE, fetchImpl: spy }).catch((e) => e);
    expect(err).toBeInstanceOf(LogApiError);
    expect((err as LogApiError).status).toBe(500);
  });

  it("throws LogApiError when the network call itself fails", async () => {
    const spy = vi.fn(async () => {
      throw new Error("offline");
    }) as unknown as typeof fetch;
    await expect(fetchLogs({}, { baseUrl: BASE, fetchImpl: spy })).rejects.toBeInstanceOf(LogApiError);
  });
});

describe("fetchServices", () => {
  it("GETs /api/services and parses { services: [...] }", async () => {
    const spy = fakeFetch({ jsonBody: { services: ["identity", "poll", "gateway"] } });
    const services = await fetchServices({ baseUrl: BASE, fetchImpl: spy });

    const [url] = (spy as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe(`${BASE}/api/services`);
    expect(services).toEqual(["identity", "poll", "gateway"]);
  });

  it("returns [] when the body has no services field", async () => {
    const spy = fakeFetch({ jsonBody: {} });
    await expect(fetchServices({ baseUrl: BASE, fetchImpl: spy })).resolves.toEqual([]);
  });
});
