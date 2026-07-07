# Identity Service — Service Specification

> **Source of truth for building this service from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this service is for

Identity answers exactly one question: **who is the User?** It handles sign-in, issues
and validates tokens, and manages sessions. *Every* other service depends on it (they all
validate its JWTs), so it is isolated for security blast-radius and hardened
independently.

It owns **nothing** about surprises, polls, ideas, or notifications. The **Subject**
(partner) never has an account here — the anonymous poll flow lives entirely in the Poll
Service.

**In scope**
- Sign in with **Apple** and **Google** (mobile-first), plus **email/password** fallback.
- Issue short-lived **JWT access tokens** and rotating **refresh tokens**.
- Session storage + **instant revocation** (sign-out, security events).
- Publish the **JWKS public key** used by the gateway and every service to verify JWTs locally.
- Login rate limiting / brute-force protection.
- Basic user profile (`GetMe`).

**Not in scope**
- Any product/domain data (polls, ideas, devices).
- Payments/entitlements (future Billing service).
- Authorization *decisions* about domain resources — those are owner-scoped checks made
  inside Poll and Surprise using the JWT subject. Identity only proves identity.

---

## 2. Ownership & data

Owns its own **PostgreSQL** schema and a **Valkey** cache. No cross-service DB access.

### PostgreSQL (`identity` schema)
- **`users`** — `id (uuid pk)`, `email (unique, nullable for social-only)`,
  `display_name`, `created_at`, `updated_at`, `status`.
- **`credentials`** — `user_id (fk)`, `type (password)`, `password_hash (argon2/bcrypt)`,
  `updated_at`. Only for email/password users.
- **`oauth_links`** — `user_id (fk)`, `provider (apple|google)`, `provider_subject
  (unique per provider)`, `linked_at`. Maps external identities → local user.

### Valkey
- **Sessions / refresh tokens** — keyed by session id → `{ user_id, refresh_token_hash,
  device, issued_at, expires_at }`. Enables instant revocation (delete the key).
- **Login rate-limit counters** — per email / per IP, with TTL.
- **Signing key material / JWKS** — current + previous public keys for rotation.

---

## 3. Contracts

### 3.1 gRPC API (`identity.v1`) — internal

| RPC | Request | Response | Notes |
|---|---|---|---|
| `SignIn` | `{ provider, id_token \| email+password }` | `{ access_token, refresh_token, expires_in, user }` | Verifies Apple/Google ID token or password; creates user on first social login |
| `RefreshToken` | `{ refresh_token }` | `{ access_token, refresh_token, expires_in }` | **Rotating** refresh: old token invalidated |
| `Revoke` | `{ refresh_token \| session_id }` | `{}` | Sign-out / kill session |
| `GetMe` | `{}` (subject from JWT) | `{ user }` | Current profile |
| `ValidateToken` | `{ access_token }` | `{ valid, subject, claims }` | Used by the gateway *only when needed*; hot-path validation is local via JWKS |
| `GetJWKS` | `{}` | `{ keys[] }` | Public key set for local verification (also exposed as an HTTP JWKS endpoint) |

### 3.2 Tokens

- **Access token:** JWT, **short-lived** (~15 min). Signed with a rotating asymmetric key
  (RS256/EdDSA). Claims: `sub` (user id), `iss`, `aud`, `exp`, `iat`, `jti`. Verified
  **locally** everywhere via JWKS — no network call on the hot path.
- **Refresh token:** opaque, long-lived, **rotating**, stored **hashed** in Valkey.
  Presenting a refresh token issues a new pair and invalidates the old one (reuse
  detection → revoke session).

### 3.3 Events

None published or consumed. Identity is purely request/response.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose |
|---|---|---|---|
| API Gateway | in | gRPC | All auth RPCs; JWKS fetch |
| Apple Sign In | out | HTTPS | Verify Apple ID tokens (Apple public keys) |
| Google Sign In | out | HTTPS | Verify Google ID tokens (Google public keys) |
| Every other service | in (indirect) | — | They fetch JWKS once and verify JWTs locally; no direct call per request |
| PostgreSQL | out | SQL | Users, credentials, oauth links |
| Valkey | out | RESP | Sessions, rate limits, key material |

**Consumers of the JWT contract:** the gateway plus Poll, Surprise, Catalog, Notification
all rely on Identity's JWT format and rotating public key. **Changing the token
claims/signing scheme is a breaking change for the whole platform** — version it.

---

## 5. Cross-cutting responsibilities owned here

- **Instant revocation:** deleting the Valkey session invalidates refresh; access tokens
  are short-lived so the blast window is minutes.
- **Key rotation:** publish current+previous public keys via JWKS so verifiers tolerate
  rotation without downtime.
- **Brute-force protection:** login rate limits per email + IP in Valkey.
- **Password hygiene:** argon2id (or bcrypt) for password hashes; never log secrets.

---

## 6. Tech stack & build notes

- **Language:** Go. gRPC internal. Postgres + Valkey.
- **Migrations:** own the `identity` schema migrations.
- **Config (env):** DB DSN, Valkey URL, Apple/Google client IDs & keys, JWT issuer/audience,
  access-token TTL, refresh-token TTL, signing keys (from secrets manager).
- **Build order:** this is **step 1 of the MVP** (Identity + gateway auth + Swagger).
  Build Sign in with Apple/Google + JWT first; email/password can follow.

## 7. Non-functional targets

- `ValidateToken`/local verify effectively free (local crypto).
- Sign-in p95 < 300 ms (dominated by external provider verification).
- Security-critical: transactional Postgres writes; no PII in logs; secrets from a manager.
