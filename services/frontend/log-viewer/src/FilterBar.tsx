// The filter bar: service + level dropdowns, time-range presets (with optional
// custom bounds), a glob-aware message search, and the live-tail / refresh
// controls. It is a controlled component — App owns the state.

import type { LogLevel } from "./types";
import { PRESET_LABELS, type RangePreset } from "./time";

const LEVELS: LogLevel[] = ["ERROR", "WARN", "INFO", "DEBUG"];
const PRESETS: RangePreset[] = ["15m", "1h", "24h", "all"];

export interface FilterState {
  service: string; // "" = All
  level: "" | LogLevel; // "" = All
  q: string;
  preset: RangePreset;
  from: string; // custom lower bound (datetime-local), only used when preset === "custom-ish"
  to: string; // custom upper bound
  useCustom: boolean;
}

export const emptyFilters: FilterState = {
  service: "",
  level: "",
  q: "",
  preset: "1h",
  from: "",
  to: "",
  useCustom: false,
};

interface Props {
  filters: FilterState;
  services: string[];
  live: boolean;
  loading: boolean;
  onChange: (next: FilterState) => void;
  onToggleLive: () => void;
  onRefresh: () => void;
}

export function FilterBar({
  filters,
  services,
  live,
  loading,
  onChange,
  onToggleLive,
  onRefresh,
}: Props) {
  const set = (patch: Partial<FilterState>) => onChange({ ...filters, ...patch });

  return (
    <div className="filters">
      <label className="field">
        <span className="field__label">Service</span>
        <select
          value={filters.service}
          onChange={(e) => set({ service: e.target.value })}
        >
          <option value="">All</option>
          {services.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </label>

      <label className="field">
        <span className="field__label">Level</span>
        <select
          value={filters.level}
          onChange={(e) => set({ level: e.target.value as "" | LogLevel })}
        >
          <option value="">All</option>
          {LEVELS.map((l) => (
            <option key={l} value={l}>
              {l}
            </option>
          ))}
        </select>
      </label>

      <label className="field field--grow">
        <span className="field__label">Search</span>
        <input
          type="text"
          placeholder="message search — supports *glob*  e.g. *auth problem:*"
          value={filters.q}
          onChange={(e) => set({ q: e.target.value })}
          spellCheck={false}
        />
      </label>

      <div className="field">
        <span className="field__label">Time</span>
        <div className="presets" role="group" aria-label="Time range">
          {PRESETS.map((p) => (
            <button
              key={p}
              type="button"
              className={
                "preset" + (!filters.useCustom && filters.preset === p ? " preset--on" : "")
              }
              onClick={() => set({ preset: p, useCustom: false })}
            >
              {PRESET_LABELS[p]}
            </button>
          ))}
          <button
            type="button"
            className={"preset" + (filters.useCustom ? " preset--on" : "")}
            onClick={() => set({ useCustom: !filters.useCustom })}
          >
            Custom
          </button>
        </div>
      </div>

      {filters.useCustom && (
        <div className="field field--grow custom-range">
          <span className="field__label">Custom range</span>
          <div className="custom-range__inputs">
            <input
              type="datetime-local"
              value={filters.from}
              onChange={(e) => set({ from: e.target.value })}
              aria-label="From"
            />
            <span className="custom-range__sep">→</span>
            <input
              type="datetime-local"
              value={filters.to}
              onChange={(e) => set({ to: e.target.value })}
              aria-label="To"
            />
          </div>
        </div>
      )}

      <div className="field controls">
        <span className="field__label">&nbsp;</span>
        <div className="controls__row">
          <button
            type="button"
            className={"toggle" + (live ? " toggle--on" : "")}
            onClick={onToggleLive}
            aria-pressed={live}
          >
            <span className={"dot" + (live ? " dot--live" : "")} aria-hidden="true" />
            {live ? "Live" : "Paused"}
          </button>
          <button
            type="button"
            className="refresh"
            onClick={onRefresh}
            disabled={live || loading}
            title={live ? "Turn off live tail to refresh manually" : "Reload logs"}
          >
            {loading ? "Loading…" : "Refresh"}
          </button>
        </div>
      </div>
    </div>
  );
}
