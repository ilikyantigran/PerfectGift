// Live-tail cursor + row-merge logic. Pure and framework-free so it can be
// unit-tested without React.

import type { LogRow } from "./types";

/**
 * The next `after` cursor for live tailing: the maximum ingest id across the
 * given rows, or `current` if none is larger. Ids are monotonic, so this never
 * moves backwards. Starts from `current` (default 0 => "everything").
 */
export function nextAfter(rows: readonly LogRow[], current = 0): number {
  let max = current;
  for (const r of rows) {
    if (r.id > max) max = r.id;
  }
  return max;
}

/**
 * Merge freshly-tailed rows into the existing list: incoming rows first
 * (they are newer), de-duplicated by id, kept newest-first, and capped at
 * `cap` rows so the table never grows without bound.
 */
export function mergeRows(
  existing: readonly LogRow[],
  incoming: readonly LogRow[],
  cap = 5000,
): LogRow[] {
  if (incoming.length === 0) return existing.slice(0, cap);
  const seen = new Set<number>();
  const merged: LogRow[] = [];
  for (const r of [...incoming, ...existing]) {
    if (seen.has(r.id)) continue;
    seen.add(r.id);
    merged.push(r);
  }
  merged.sort((a, b) => b.id - a.id); // newest (highest id) first
  return merged.slice(0, cap);
}
