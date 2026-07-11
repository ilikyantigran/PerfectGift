// Time-range presets, computed client-side into RFC3339 from/to bounds.

export type RangePreset = "15m" | "1h" | "24h" | "all";

export const PRESET_LABELS: Record<RangePreset, string> = {
  "15m": "Last 15m",
  "1h": "Last 1h",
  "24h": "Last 24h",
  all: "All time",
};

const MINUTE = 60_000;
const PRESET_MS: Record<Exclude<RangePreset, "all">, number> = {
  "15m": 15 * MINUTE,
  "1h": 60 * MINUTE,
  "24h": 24 * 60 * MINUTE,
};

/**
 * Turn a preset into `{ from, to }` RFC3339 bounds. "all" yields no bounds.
 * The other presets are open-ended at the top (to = now is left implicit) so
 * live-tailed rows past `now` still show; only a lower bound is set.
 */
export function computeRange(preset: RangePreset, now: Date = new Date()): {
  from?: string;
  to?: string;
} {
  if (preset === "all") return {};
  const from = new Date(now.getTime() - PRESET_MS[preset]);
  return { from: from.toISOString() };
}
