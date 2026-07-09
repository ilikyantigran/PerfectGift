// Gateway base URL, configurable per environment via a Vite build-time env var.
// Defaults to the local Docker stack's gateway.
const RAW_BASE =
  (import.meta.env.VITE_GATEWAY_BASE_URL as string | undefined) ??
  "http://localhost:8080";

// Strip any trailing slash so path joins are predictable.
export const GATEWAY_BASE_URL = RAW_BASE.replace(/\/+$/, "");
