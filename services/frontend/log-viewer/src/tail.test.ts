import { describe, expect, it } from "vitest";
import { nextAfter, mergeRows } from "./tail";
import type { LogRow } from "./types";

const row = (id: number): LogRow => ({
  id,
  ts: "2026-07-10T18:34:53.123456Z",
  level: "INFO",
  service: "identity",
  message: `row ${id}`,
  trace_id: "",
  span_id: "",
  fields: {},
});

describe("nextAfter", () => {
  it("returns the maximum id across the rows", () => {
    expect(nextAfter([row(3), row(9), row(7)])).toBe(9);
  });

  it("handles newest-first ordering the same as any other order", () => {
    expect(nextAfter([row(9), row(7), row(3)])).toBe(9);
  });

  it("falls back to the current cursor when no row is larger", () => {
    expect(nextAfter([row(2), row(4)], 10)).toBe(10);
    expect(nextAfter([], 42)).toBe(42);
  });

  it("defaults the starting cursor to 0", () => {
    expect(nextAfter([])).toBe(0);
  });
});

describe("mergeRows", () => {
  it("prepends incoming rows and keeps the list newest-first", () => {
    const existing = [row(3), row(2), row(1)];
    const incoming = [row(5), row(4)];
    expect(mergeRows(existing, incoming).map((r) => r.id)).toEqual([5, 4, 3, 2, 1]);
  });

  it("de-duplicates by id", () => {
    const existing = [row(3), row(2)];
    const incoming = [row(3), row(4)]; // 3 already present
    expect(mergeRows(existing, incoming).map((r) => r.id)).toEqual([4, 3, 2]);
  });

  it("caps the total number of rows", () => {
    const existing = [row(3), row(2), row(1)];
    const incoming = [row(5), row(4)];
    expect(mergeRows(existing, incoming, 2).map((r) => r.id)).toEqual([5, 4]);
  });
});
