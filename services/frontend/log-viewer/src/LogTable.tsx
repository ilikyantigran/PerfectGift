// The log table: time · level · service · message · trace. Rows expand to show
// their `fields` as pretty JSON. Presentational only — App owns the data.

import { useState } from "react";
import type { LogRow } from "./types";
import { jaegerTraceUrl, shortTraceId } from "./trace";

interface Props {
  rows: LogRow[];
}

export function LogTable({ rows }: Props) {
  return (
    <div className="table-wrap">
      <table className="logs">
        <thead>
          <tr>
            <th className="col-expand" aria-label="expand" />
            <th className="col-time">Time</th>
            <th className="col-level">Level</th>
            <th className="col-service">Service</th>
            <th className="col-message">Message</th>
            <th className="col-trace">Trace</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <LogRowView key={r.id} row={r} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function LogRowView({ row }: { row: LogRow }) {
  const [open, setOpen] = useState(false);
  const hasFields = row.fields && Object.keys(row.fields).length > 0;

  return (
    <>
      <tr className={"row row--" + row.level.toLowerCase()}>
        <td className="col-expand">
          {hasFields ? (
            <button
              type="button"
              className="expander"
              aria-expanded={open}
              aria-label={open ? "Collapse fields" : "Expand fields"}
              onClick={() => setOpen((v) => !v)}
            >
              {open ? "▾" : "▸"}
            </button>
          ) : null}
        </td>
        <td className="col-time" title={row.ts}>
          {formatTs(row.ts)}
        </td>
        <td className="col-level">
          <span className={"level level--" + row.level.toLowerCase()}>{row.level}</span>
        </td>
        <td className="col-service">{row.service}</td>
        <td className="col-message">{row.message}</td>
        <td className="col-trace">
          <TraceCell traceId={row.trace_id} />
        </td>
      </tr>
      {open && hasFields && (
        <tr className="row-fields">
          <td />
          <td colSpan={5}>
            <pre className="fields">{JSON.stringify(row.fields, null, 2)}</pre>
          </td>
        </tr>
      )}
    </>
  );
}

function TraceCell({ traceId }: { traceId: string }) {
  const url = jaegerTraceUrl(traceId);
  if (!url) return <span className="trace trace--empty">—</span>;

  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(traceId);
    } catch {
      /* clipboard blocked (e.g. insecure origin) — silently ignore */
    }
  };

  return (
    <span className="trace">
      <a
        className="trace__link"
        href={url}
        target="_blank"
        rel="noreferrer noopener"
        title={`Open trace ${traceId} in Jaeger`}
      >
        {shortTraceId(traceId)}
      </a>
      <button
        type="button"
        className="trace__copy"
        onClick={copy}
        title="Copy trace_id"
        aria-label="Copy trace id"
      >
        ⧉
      </button>
    </span>
  );
}

// Show HH:MM:SS.mmm (local) but keep the full ISO string in a tooltip. Falls
// back to the raw string if it is somehow unparseable.
function formatTs(ts: string): string {
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  const p = (n: number, w = 2) => String(n).padStart(w, "0");
  return (
    `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}` +
    `.${p(d.getMilliseconds(), 3)}`
  );
}
