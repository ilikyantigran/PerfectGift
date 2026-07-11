// Base URL of the log-server, configurable per environment via a Vite build-time
// env var.
//
// In production the log-server serves this bundle itself, so the app calls the
// same origin — an empty base ("") makes every request relative. During
// `npm run dev` the Vite dev server runs on a different port than the log-server,
// so we default to the local log-server (http://localhost:8086). An explicit
// VITE_LOG_SERVER_URL always wins.
const explicit = import.meta.env.VITE_LOG_SERVER_URL as string | undefined;
const fallback = import.meta.env.DEV ? "http://localhost:8086" : "";

// Strip any trailing slash so path joins are predictable ("" stays "").
export const LOG_SERVER_URL = (explicit ?? fallback).replace(/\/+$/, "");
