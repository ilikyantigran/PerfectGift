import { describe, expect, it } from "vitest";
import { jaegerTraceUrl, shortTraceId, JAEGER_BASE_URL } from "./trace";

const TRACE = "0af7651916cd43dd8448eb211c80319c";

describe("jaegerTraceUrl", () => {
  it("builds http://localhost:16686/trace/<id>", () => {
    expect(jaegerTraceUrl(TRACE)).toBe(`${JAEGER_BASE_URL}/trace/${TRACE}`);
    expect(jaegerTraceUrl(TRACE)).toBe(`http://localhost:16686/trace/${TRACE}`);
  });

  it("returns null for an empty, blank, null, or undefined trace_id", () => {
    expect(jaegerTraceUrl("")).toBeNull();
    expect(jaegerTraceUrl("   ")).toBeNull();
    expect(jaegerTraceUrl(null)).toBeNull();
    expect(jaegerTraceUrl(undefined)).toBeNull();
  });
});

describe("shortTraceId", () => {
  it("shows the first 8 chars with an ellipsis by default", () => {
    expect(shortTraceId(TRACE)).toBe("0af76519…");
  });

  it("renders an em dash for an empty trace_id", () => {
    expect(shortTraceId("")).toBe("—");
    expect(shortTraceId(undefined)).toBe("—");
  });

  it("does not truncate ids already at or below the head length", () => {
    expect(shortTraceId("abcd", 8)).toBe("abcd");
  });
});
