# Identity Service — Implementation Plan

_Contract: `services/backend/identity/SERVICE.md` + root `architecture.md`. House style: `backend-service` skill / `knp-service`._

## What we're building & why

The Identity service answers "who is the User?": Sign in with Apple/Google (+ email/password
fallback), issue short-lived RS256/EdDSA **JWT access tokens** + rotating opaque **refresh
tokens**, store sessions in Valkey for **instant revocation**, and publish a **JWKS** so every
other service verifies JWTs locally. gRPC internal + HTTP/Swagger edge. Postgres owns the
`identity` schema; Valkey owns sessions + rate-limit counters + key material.

Go module: `github.com/ilikyantigran/PerfectGift/services/backend/identity`.

## RPCs (exactly the contract's six)

| RPC | HTTP | Behavior |
|---|---|---|
| `SignIn` | POST `/v1/auth/signin` | Verify Apple/Google ID token OR email+password; create user on first social login; issue token pair + session |
| `RefreshToken` | POST `/v1/auth/refresh` | Rotating refresh: validate → issue new pair → invalidate old; reuse → revoke session |
| `Revoke` | POST `/v1/auth/revoke` | Sign-out / kill session by refresh_token or session_id |
| `GetMe` | GET `/v1/auth/me` | Subject from Bearer access token in metadata → current profile |
| `ValidateToken` | POST `/v1/auth/validate` | Local JWT crypto/claims check → {valid, subject, claims} |
| `GetJWKS` | GET `/.well-known/jwks.json` | Public key set (current + previous) for local verification |

## Tokens & crypto decisions

- **Access token**: JWT, **EdDSA (Ed25519)** — allowed by contract, tiny keys, instant
  generation, standard JWKS `OKP`/RFC 8037. Claims: `sub`, `iss`, `aud`, `exp`, `iat`, `jti`,
  plus `sid` (session id). ~15 min TTL. Verified locally via JWKS; `ValidateToken` does NOT
  hit Valkey (hot-path-free per contract).
- **Refresh token**: opaque `"{sessionID}.{secret}"`; only `sha256(secret)` stored in Valkey
  under `session:{sid}`. Rotation on every refresh; presented-secret mismatch ⇒ reuse ⇒ delete
  session (revoke). Long TTL.
- **Key rotation**: `token.Manager` keeps current + previous key; `JWKS()` publishes both so
  verifiers tolerate rotation. `Rotate()` promotes current→previous, mints a new current.
- **Passwords**: Argon2id (`golang.org/x/crypto/argon2`), encoded PHC string with per-hash salt.
- **Provider verification** behind `oauth.Verifier` interface: real Google/Apple client (fetches
  provider JWKS over HTTPS, verifies RS256) + `FakeVerifier` for hermetic tests.

## Files

```
api/identity/v1/identity.proto          proto contract (+ grpc-gateway HTTP annotations)
cmd/identity/main.go                    thin entrypoint (CONFIG_PATH)
configs/values_local.yaml               local addresses
configs/values_docker.yaml              docker addresses
migrations/0001_init.sql                identity schema: users, credentials, oauth_links
internal/app/app.go                     App object: wire telemetry→stores→server→gRPC+HTTP
internal/app/identity_server.go         the 6 RPC handlers + store interfaces
internal/app/identity_server_test.go    unit tests w/ in-memory fakes + real token/password
internal/app/fakes_test.go              in-memory Users/Sessions/RateLimiter fakes
internal/token/token.go                 Ed25519 JWT issue/verify + JWKS + rotation
internal/token/token_test.go
internal/password/password.go           Argon2id hash/verify
internal/password/password_test.go
internal/oauth/oauth.go                 Verifier interface + Identity + FakeVerifier
internal/oauth/provider.go              real Google/Apple JWKS verifier
internal/oauth/oauth_test.go            fake + JWK parsing tests
internal/domain/postgres/postgres.go    pgx UserRepo (Users impl) + embedded migrate
internal/domain/postgres/postgres_test.go  testcontainers-guarded, skips w/o DB
internal/domain/valkey/valkey.go        SessionStore + RateLimiter (valkey-go)
internal/infra/config/config.go         Config struct + InitConfig
internal/infra/telemetry/telemetry.go   house telemetry (verbatim)
internal/infra/docs/docs.go             embedded swagger UI
pkg/api/identity/v1/*                    generated
Dockerfile, Makefile, README.md, PLAN.md, PROGRESS.md
```

## Tests written first (red → green)

- **token**: issue→verify round-trip; expired rejected; tampered rejected; wrong-audience/issuer
  rejected; JWKS has current+previous kids; after `Rotate()` old token still verifies; JWKS OKP
  shape is valid.
- **password**: hash→verify true; wrong password false; two hashes of same pw differ (salt);
  malformed encoded hash errors.
- **oauth**: FakeVerifier returns configured identity; unknown token errors; JWK n/e → rsa key.
- **identity_server** (fakes): social first login creates user+oauth_link; second social login
  same subject ⇒ same user; email first sign-in registers, second verifies, wrong pw ⇒
  Unauthenticated (no factor leak); rotating refresh issues new pair + invalidates old; refresh
  reuse ⇒ session revoked; Revoke kills session; GetMe w/ bearer ⇒ user, missing ⇒
  Unauthenticated; ValidateToken valid/invalid; GetJWKS returns keys; rate-limit blocks after N
  bad email attempts.
- **postgres** (integration): real upsert-oauth / credential round-trip — `t.Skip` when no DB.

## Key decisions / assumptions (deviations noted for final report)

1. **EdDSA** chosen over RS256 (contract allows either).
2. **No Register RPC exists** in the contract, so `SignIn` with `provider=EMAIL` is
   **sign-in-or-register**: first use of an unregistered email creates the account with the
   supplied password; later uses verify. Documented assumption.
3. **Error model**: idiomatic gRPC status codes (`Unauthenticated`, `InvalidArgument`,
   `NotFound`, `ResourceExhausted`), with credential failures returning a generic message (no
   account-enumeration oracle). gRPC codes map to HTTP via grpc-gateway.
4. `GetMe` reads the access token from the `authorization` gRPC metadata (grpc-gateway forwards
   the HTTP `Authorization` header), per the "subject from JWT" note.
5. Unit tests are hermetic (fakes/in-memory, real crypto). The Postgres integration test and the
   real OAuth verifier require external resources and are skipped/untested in `go test ./...`.

## Verify

`go build ./...` + `go test ./...` green (report exact output); `make generate` clean;
README documents env/migrations/docker/test.
