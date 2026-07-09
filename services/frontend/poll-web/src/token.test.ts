import { describe, expect, it } from "vitest";
import { extractToken } from "./token";

describe("extractToken", () => {
  it("reads the ?t= query param first", () => {
    expect(extractToken("https://poll.example.com/?t=abc123")).toBe("abc123");
  });

  it("reads a /p/<token> path segment", () => {
    expect(extractToken("https://poll.example.com/p/xyz789")).toBe("xyz789");
  });

  it("reads a bare single path segment", () => {
    expect(extractToken("https://poll.example.com/tok-42")).toBe("tok-42");
  });

  it("reads a hash fallback (#/<token>)", () => {
    expect(extractToken("https://poll.example.com/#/hashtok")).toBe("hashtok");
  });

  it("prefers the query param over a path segment", () => {
    expect(extractToken("https://poll.example.com/p/pathtok?t=querytok")).toBe("querytok");
  });

  it("url-decodes a path token", () => {
    expect(extractToken("https://poll.example.com/p/a%20b")).toBe("a b");
  });

  it("returns null when there is no token", () => {
    expect(extractToken("https://poll.example.com/")).toBeNull();
  });

  it("returns null for a malformed url", () => {
    expect(extractToken("not a url")).toBeNull();
  });
});
