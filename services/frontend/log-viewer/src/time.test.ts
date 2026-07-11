import { describe, expect, it } from "vitest";
import { computeRange } from "./time";

const NOW = new Date("2026-07-10T12:00:00.000Z");

describe("computeRange", () => {
  it("computes a 15-minute lower bound from now", () => {
    expect(computeRange("15m", NOW)).toEqual({ from: "2026-07-10T11:45:00.000Z" });
  });

  it("computes a 1-hour lower bound from now", () => {
    expect(computeRange("1h", NOW)).toEqual({ from: "2026-07-10T11:00:00.000Z" });
  });

  it("computes a 24-hour lower bound from now", () => {
    expect(computeRange("24h", NOW)).toEqual({ from: "2026-07-09T12:00:00.000Z" });
  });

  it("returns no bounds for 'all'", () => {
    expect(computeRange("all", NOW)).toEqual({});
  });
});
