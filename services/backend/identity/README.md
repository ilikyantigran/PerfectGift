# Identity Service

> "Who is the User?" — the auth core of PerfectGift. Sign in with Apple / Google
> (+ email/password fallback), issue short-lived JWT access tokens and rotating
> refresh tokens, keep sessions in Valkey for instant revocation, and publish a
> JWKS every other service uses to verify tokens locally.

Go module: `github.com/ilikyantigran/PerfectGift/services/backend/identity`.
See [`SERVICE.md`](./SERVICE.md) for the contract and [`PLAN.md`](./PLAN.md) for
the design decisions.

## RPCs (`identity.v1`)

| RPC | HTTP edge | What it does |
|---|---|---|
| `SignIn` | `POST /v1/auth/signin` | Verify Apple/Google ID token **or** email+password; returns an access/refresh pair + user. First social login creates the user; first email sign-in registers it. |
| `RefreshToken` | `POST /v1/auth/refresh` | Rotating refresh — old token invalidated; reuse of a stale token revokes the session. |
| `Revoke` | `POST /v1/auth/revoke` | Sign-out / kill a session by `refresh_token` or `session_id`. |
| `GetMe` | `GET /v1/auth/me` | Current profile; subject taken from the `Authorization: Bearer` access token. |
| `ValidateToken` | `POST /v1/auth/validate` | Local JWT verification → `{valid, subject, claims}`. |
| `GetJWKS` | `GET /.well-known/jwks.json` | Public keys (current + previous) for local verification. |

Two listeners: **gRPC** (internal) on `grpc_port` and **HTTP** on `http_port`
carrying the REST gateway, `/metrics` (Prometheus) and `/swagger/` (Swagger UI).

## Tokens

- **Access token** — JWT signed with **EdDSA (Ed25519)**, ~15 min TTL. Claims:
  `sub`, `iss`, `aud`, `exp`, `iat`, `jti`, `sid`. Verified locally everywhere via
  the JWKS; `ValidateToken` never touches Valkey.
- **Refresh token** — opaque `"{sessionID}.{secret}"`; only `sha256(secret)` is
  stored in Valkey under `identity:session:{sid}`. Every refresh rotates the
  secret; a presented secret that no longer matches ⇒ reuse ⇒ the session is
  deleted (instant revocation).
- **Key rotation** — `token.Manager` keeps a current + previous key and publishes
  both in the JWKS, so verifiers keep working across a rotation.
- **Passwords** — Argon2id, stored as a self-describing PHC string.

## Data

- **PostgreSQL — `identity` schema** (`users`, `credentials`, `oauth_links`).
  Migrations are embedded (`internal/domain/postgres/migrations/`) and applied
  automatically at startup; the SQL is also readable there for manual runs.
- **Valkey** — sessions/refresh tokens (`identity:session:*`) and login
  rate-limit counters (`identity:ratelimit:*`).

## Configuration

Config is a YAML file selected by `CONFIG_PATH` (defaults to
`./configs/values_local.yaml`; the Docker image sets
`/app/configs/values_docker.yaml`). It is data-only — no secrets. See
[`configs/values_local.yaml`](./configs/values_local.yaml).

| Key | Meaning |
|---|---|
| `identity_service.{host,grpc_port,http_port}` | listeners (gateway dials its own gRPC on `host:grpc_port`) |
| `postgres.dsn` | Postgres DSN (credentials live here, from your secrets manager) |
| `valkey.address` | Valkey address |
| `token.{issuer,audience,access_ttl_seconds,refresh_ttl_seconds}` | JWT settings |
| `oauth.{google_client_ids,apple_client_ids}` | accepted audiences for provider ID tokens (non-secret) |
| `rate_limit.{max_attempts,window_seconds}` | email sign-in brute-force guard |

Signing keys are generated in-process at boot (nothing sensitive in config).
Tracing is enabled when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

## Run locally

Needs Postgres and Valkey reachable at the addresses in `values_local.yaml`:

```bash
docker run --rm -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=perfectgift -p 5432:5432 postgres:16
docker run --rm -p 6379:6379 valkey/valkey:8

go run ./cmd/identity          # applies migrations, then serves gRPC + HTTP
```

Then: `open http://localhost:8081/swagger/`, `curl localhost:8081/metrics`,
`curl localhost:8081/.well-known/jwks.json`.

Example email sign-in (registers on first call):

```bash
curl -s localhost:8081/v1/auth/signin -d \
  '{"provider":"PROVIDER_EMAIL","email":"a@example.com","password":"hunter2horse"}'
```

## Docker

```bash
docker build -t perfectgift/identity .
docker run --rm -p 8080:8080 -p 8081:8081 perfectgift/identity   # uses values_docker.yaml
```

## Regenerate protobuf/gRPC/gateway/Swagger

```bash
make vendor-proto   # one-time: vendor google/api + well-known protos
make generate       # regenerate pkg/api/** and the embedded swagger.json
```

## Test

```bash
go build ./...
go test ./...       # hermetic: no live DB, Valkey, network, or provider creds
```

Unit tests cover the token manager, Argon2id, the OAuth verifier (real RS256
path exercised against an in-process JWKS server), and every RPC via in-memory
fakes; `transport_e2e_test.go` drives real HTTP through the gRPC + gateway edge.

The Postgres integration tests are **skipped** unless a disposable database is
provided:

```bash
docker run --rm -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:16
IDENTITY_TEST_DATABASE_DSN='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable' \
  go test ./internal/domain/postgres/...
```
