# API Gateway / BFF

> The single public HTTP edge of PerfectGift. Terminates TLS-less HTTP, validates JWTs,
> rate-limits, and translates **REST/JSON ⇄ gRPC** to the five domain services. Stateless.

- **Language:** Go · **Edge:** HTTP/JSON + Swagger · **Internal:** gRPC clients
- **Contract:** [`SERVICE.md`](SERVICE.md) · **System context:** [`architecture.md`](../../../architecture.md)

## What it does

- Exposes the public `/v1` REST API (the only thing clients talk to) and serves the OpenAPI/Swagger doc.
- Validates Identity-issued **EdDSA JWTs locally** via JWKS (no per-request call to Identity).
- Owner vs. anonymous routing: JWT routes for the User; opaque-token routes for the anonymous Subject poll flow.
- Rate limiting (per-user / per-IP), CORS for the poll web page, and a uniform error envelope
  `{"error":{"code","message","details"}}`.
- Propagates caller identity to downstream services as gRPC metadata (`authorization`, `x-user-id`, `x-forwarded-for`).
- BFF aggregation: `GET /v1/generations/{id}` merges `GetGenerationStatus` + `GetIdeas` into one payload.
- Generation is async — returns **`202 Accepted`**; never blocks on the LLM.

## Routes (→ downstream gRPC)

| Method & path | Auth | Downstream |
|---|---|---|
| `POST /v1/auth/signin` · `refresh` · `revoke` · `GET /v1/me` | public / JWT | Identity |
| `POST /v1/polls` · `GET /v1/polls/{id}/responses` | JWT | Poll |
| `GET /v1/polls/token/{t}` · `POST …/responses` | anonymous token | Poll |
| `POST /v1/generations` · `GET /v1/generations/{id}` · `…/refine` · `POST /v1/ideas/{id}/save` | JWT | Surprise |
| `GET /v1/holidays` · `GET /v1/categories` | JWT | Catalog |
| `POST /v1/devices` | JWT | Notification |

Full table in [`SERVICE.md`](SERVICE.md) §3.1.

## Run

```bash
# With the full stack (recommended) — from the repo root:
docker compose up --build -d
# → gateway on http://localhost:8080  (Swagger at /swagger/, metrics at /metrics)
```

Standalone (needs the domain services reachable at the configured addresses):

```bash
make generate                 # regenerate gRPC client stubs from api/*.proto (protoc)
CONFIG_PATH=configs/values_local.yaml go run ./cmd/gateway
```

## Test

```bash
go test ./...   # routing, auth gating, error mapping, 202-async — all against fakes (no live services)
```

## Configuration (`configs/values_docker.yaml`)

- `gateway_service.http_port` — edge port (8080).
- `downstreams.*` — gRPC addresses of the five services (`identity:9090`, …). Dialed with a `dns:///` scheme.
- `auth.jwks_url` / `issuer` / `audience` — JWT validation.
- `cors.poll_origins`, `ratelimit.*`.

## Notes

- The `api/*/v1/*.proto` files are **copies of the real service protos**; `make generate` produces the
  client stubs (`pkg/api/**`, git-ignored). Keep them in sync with the services.
- Stateless — no database. Optional Valkey for rate-limit counters.

## Layout

```
cmd/gateway/           entrypoint
internal/
  transport/rest/      handlers (auth, poll, generation, catalog, device), router, middleware
  clients/             gRPC client dialing (dns:/// targets)
  auth/                JWKS verifier
  ratelimit/           per-user / per-IP limiters
  infra/{config,telemetry,docs}
api/<svc>/v1/*.proto   real service protos (source for codegen)
```
