import { useCallback, useEffect, useRef, useState } from "react";
import { fetchLogs, fetchServices, LogApiError } from "./api";
import type { LogQuery, LogRow } from "./types";
import { nextAfter, mergeRows } from "./tail";
import { computeRange } from "./time";
import { FilterBar, emptyFilters, type FilterState } from "./FilterBar";
import { LogTable } from "./LogTable";

const TAIL_INTERVAL_MS = 2000;

// Turn the UI filter state into the wire query (minus the live-tail `after`).
function filtersToQuery(f: FilterState): LogQuery {
  const q: LogQuery = {};
  if (f.service) q.service = f.service;
  if (f.level) q.level = f.level;
  if (f.q.trim()) q.q = f.q.trim();

  if (f.useCustom) {
    const from = toIso(f.from);
    const to = toIso(f.to);
    if (from) q.from = from;
    if (to) q.to = to;
  } else {
    const range = computeRange(f.preset);
    if (range.from) q.from = range.from;
    if (range.to) q.to = range.to;
  }
  return q;
}

// datetime-local value ("2026-07-10T12:00") -> RFC3339 UTC, or undefined.
function toIso(local: string): string | undefined {
  if (!local) return undefined;
  const d = new Date(local);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

export function App() {
  const [filters, setFilters] = useState<FilterState>(emptyFilters);
  const [services, setServices] = useState<string[]>([]);
  const [rows, setRows] = useState<LogRow[]>([]);
  const [live, setLive] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);

  // Refs the tail interval reads so it never closes over stale state.
  const cursorRef = useRef(0);
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  // Populate the service dropdown once. A failure here is non-fatal — the log
  // fetch surfaces connectivity problems on its own.
  useEffect(() => {
    let cancelled = false;
    fetchServices()
      .then((s) => !cancelled && setServices(s))
      .catch(() => {
        /* leave the dropdown at just "All" */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Full (re)load: replace all rows and reset the tail cursor to the newest id.
  const reload = useCallback(async (f: FilterState) => {
    setLoading(true);
    setError(null);
    try {
      const logs = await fetchLogs(filtersToQuery(f));
      setRows(logs);
      cursorRef.current = nextAfter(logs);
    } catch (e) {
      setRows([]);
      setError(e instanceof LogApiError ? e.message : "Something went wrong loading logs.");
    } finally {
      setLoading(false);
      setLoaded(true);
    }
  }, []);

  // Reload whenever the filters change. Serialize so preset/custom edits refire.
  const filterKey = JSON.stringify(filters);
  useEffect(() => {
    void reload(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterKey, reload]);

  // Live tail: poll for rows with id > cursor and prepend them.
  useEffect(() => {
    if (!live) return;
    let stopped = false;
    const tick = async () => {
      try {
        const logs = await fetchLogs({
          ...filtersToQuery(filtersRef.current),
          after: cursorRef.current,
        });
        if (stopped || logs.length === 0) return;
        cursorRef.current = nextAfter(logs, cursorRef.current);
        setRows((prev) => mergeRows(prev, logs));
        setError(null);
      } catch (e) {
        setError(e instanceof LogApiError ? e.message : "Live tail interrupted.");
      }
    };
    const id = setInterval(tick, TAIL_INTERVAL_MS);
    return () => {
      stopped = true;
      clearInterval(id);
    };
  }, [live]);

  const onRefresh = useCallback(() => void reload(filtersRef.current), [reload]);
  const onToggleLive = useCallback(() => setLive((v) => !v), []);

  return (
    <div className="app">
      <header className="app__header">
        <h1 className="app__title">
          PerfectGift <span className="app__title-dim">/ logs</span>
        </h1>
        <div className="app__count">{rows.length} rows</div>
      </header>

      <FilterBar
        filters={filters}
        services={services}
        live={live}
        loading={loading}
        onChange={setFilters}
        onToggleLive={onToggleLive}
        onRefresh={onRefresh}
      />

      {error && (
        <div className="banner banner--error" role="alert">
          {error}
        </div>
      )}

      <main className="app__body">
        {loading && !loaded ? (
          <div className="state state--loading">Loading logs…</div>
        ) : rows.length === 0 && !error ? (
          <div className="state state--empty">
            No logs match these filters.
            {!live && " Adjust the filters, hit Refresh, or turn on Live."}
          </div>
        ) : (
          <LogTable rows={rows} />
        )}
      </main>
    </div>
  );
}
