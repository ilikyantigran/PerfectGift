// Extracts the opaque poll link token from a URL. Deliberately tiny: no router.
//
// Supported link shapes (in priority order):
//   1. Query param:   https://poll.example.com/?t=OPAQUE_TOKEN
//   2. Path segment:  https://poll.example.com/p/OPAQUE_TOKEN
//   3. Hash:          https://poll.example.com/#/OPAQUE_TOKEN  (CDN-friendly fallback)
//
// Returns the token string, or null if none is present.
export function extractToken(url: string): string | null {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }

  const q = parsed.searchParams.get("t");
  if (q && q.trim()) return q.trim();

  // Path form: /p/<token> or just /<token>
  const pathParts = parsed.pathname.split("/").filter(Boolean);
  if (pathParts.length >= 2 && pathParts[0] === "p") {
    return decodeURIComponent(pathParts[1]);
  }
  if (pathParts.length === 1) {
    return decodeURIComponent(pathParts[0]);
  }

  // Hash form: #/<token> or #<token>
  const hash = parsed.hash.replace(/^#\/?/, "").trim();
  if (hash) return decodeURIComponent(hash);

  return null;
}

// Convenience for the running app.
export function currentToken(): string | null {
  return extractToken(window.location.href);
}
