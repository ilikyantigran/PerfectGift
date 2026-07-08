# API Gateway / BFF — Implementation Plan

_Source of truth: `SERVICE.md` (this folder) + root `architecture.md` + the five domain
`SERVICE.md` files. Built with the `/backend-service` house style and `/tdd-workflow`._

## What this service is

The single public HTTP/JSON + Swagger edge of PerfectGift. **Stateless** — no database,
no schema. It is a **gRPC client** to all five domain services (identity, poll, surprise,
catalog, notification). It validates Identity-issued JWTs **locally** via JWKS (no
per-request call on the hot path), rate-limits, enforces CORS on the two anonymous poll
routes, translates REST ⇄ gRPC, and owns the public OpenAPI/Swagger document.

## Deviation from the standard house template (deliberate, justified)

The canonical `app.go` template registers a **gRPC server + grpc-gateway**. The gateway is
the **inverse**: it exposes **HTTP only** and is a gRPC **client**. So:

- No gRPC server / no `Register<Svc>Server`. `App.Run` dials 5 downstream clients and
  serves one HTTP port (router + `/metrics` + `/swagger/`).
- Local proto copies are **client contracts** — pure proto3 messages + service defs, **no
  `google.api.http` annotations** (those belong to each domain service's own REST edge, not
  to the gateway calling them by gRPC). Consequence: codegen needs **no vendored protos and
  no network** — only `--go_out` + `--go-grpc_out`.
- The public OpenAPI doc is **hand-authored** (`openapi.yaml`, embedded + served), because
  the gateway has no proto-defined REST server to generate it from. It owns this contract.
- Structural house conventions kept verbatim: one Go module, `cmd/`/`configs/`/`internal/`/
  `pkg/`, `CONFIG_PATH` + local/docker YAML, `App` object (`NewApp`/`Run`), telemetry
  (slog+otel+prometheus) copied as-is, `internal/infra/docs` Swagger UI.

## Module

`go mod init github.com/ilikyantigran/PerfectGift/services/backend/api-gateway`
Dependency versions pinned to `knp-service` (all present in the local module cache →
offline build): grpc 1.81.1, protobuf 1.36.11, otel 1.44.0, prometheus, yaml.v3.
Router uses the **stdlib** `net/http.ServeMux` method+pattern routing (Go 1.22+) — no chi
dependency. JWT/JWKS verification is **implemented in-house** (crypto/rsa + crypto/ed25519)
— no external JWT lib, so tests run fully offline.

## Route table → gRPC mapping (every row of SERVICE.md §3.1)

| Method & Path | Auth | gRPC call |
|---|---|---|
| `POST /v1/auth/signin` | Public | `identity.Identity/SignIn` |
| `POST /v1/auth/refresh` | Public (strict RL) | `identity.Identity/RefreshToken` |
| `POST /v1/auth/revoke` | JWT | `identity.Identity/Revoke` |
| `GET  /v1/me` | JWT | `identity.Identity/GetMe` |
| `POST /v1/polls` | JWT | `poll.Poll/CreatePoll` |
| `GET  /v1/polls/{id}/responses` | JWT | `poll.Poll/GetResponses` |
| `GET  /v1/polls/token/{t}` | Token (CORS) | `poll.Poll/GetPollByToken` |
| `POST /v1/polls/token/{t}/responses` | Token (CORS) | `poll.Poll/SubmitResponse` |
| `POST /v1/generations` | JWT | `surprise.Surprise/RequestGeneration` → **202 {request_id}** |
| `GET  /v1/generations/{id}` | JWT | `surprise.Surprise/GetGenerationStatus` (+ `GetIdeas` when ready) |
| `POST /v1/generations/{id}/refine` | JWT | `surprise.Surprise/Refine` |
| `POST /v1/ideas/{id}/save` | JWT | `surprise.Surprise/SaveIdea` |
| `GET  /v1/holidays` | JWT | `catalog.Catalog/ListHolidays` |
| `GET  /v1/categories` | JWT | `catalog.Catalog/GetCategories` |
| `POST /v1/devices` | JWT | `notification.Notification/RegisterDevice` |

## File tree to create

```
api/{identity,poll,surprise,catalog,notification}/v1/*.proto   # local client contracts
pkg/api/.../v1/*.pb.go, *_grpc.pb.go                            # generated (make generate)
cmd/gateway/main.go
configs/values_local.yaml, values_docker.yaml
openapi.yaml                                                    # public contract (embedded)
internal/
  app/app.go                     # App: NewApp/Run — telemetry, dial clients, HTTP serve
  clients/clients.go             # Dial all 5 downstreams; hold generated clients; Close
  infra/config/config.go         # Config struct + InitConfig
  infra/telemetry/telemetry.go   # verbatim template
  infra/docs/docs.go             # embed+serve openapi.yaml + Swagger UI
  auth/verifier.go               # JWKS fetch/cache + local JWT verify (RS256/EdDSA), fail-closed
  ratelimit/limiter.go           # Limiter interface + in-memory fixed-window (Valkey optional)
  transport/rest/
    router.go        # Router(deps) -> http.Handler; all 15 routes + middleware chains
    errors.go        # uniform envelope {error:{code,message,details}} + gRPC→HTTP mapping
    middleware.go    # requireJWT, rateLimit, cors(poll-token only), recover, requestID
    dto.go           # snake_case JSON request/response DTOs + proto mapping helpers
    auth_handlers.go poll_handlers.go generation_handlers.go catalog_handlers.go device_handlers.go
Makefile  Dockerfile  README.md  PROGRESS.md
```

## Tests to write first (TDD)

- `auth/verifier_test.go`: valid RS256 token → claims; expired → fail; wrong-issuer/aud →
  fail; unknown kid → fail; bad signature → fail; JWKS fallback to last-known on fetch error.
- `ratelimit/limiter_test.go`: allows under budget, blocks over budget, window reset, keys
  isolated (per-ip vs per-user vs global).
- `transport/rest/errors_test.go`: each gRPC code → correct HTTP status + envelope code
  string; envelope shape `{error:{code,message,details}}`.
- `transport/rest/middleware_test.go`: JWT route rejects missing/invalid token (401, fail
  closed); token routes bypass JWT; CORS headers only on `/v1/polls/token/*`; preflight
  OPTIONS; rate-limit 429.
- `transport/rest/router_test.go` (against **fakes** for all 5 gRPC clients):
  - routing: each route reaches the right client method with mapped fields;
  - identity from JWT subject, not body (user_id injected server-side);
  - `POST /v1/generations` returns **202** with `{request_id}` and forwards
    `Idempotency-Key`;
  - `GET /v1/generations/{id}` aggregates status(+ideas when ready);
  - anonymous token routes work with no JWT;
  - downstream gRPC error → mapped HTTP status + envelope.

## Key decisions

- **Fakes = generated `<Svc>Client` interfaces.** protoc-gen-go-grpc already emits an
  interface per service; deps hold interface types; tests inject func-field fakes. No real
  service/DB/network in tests.
- **Explicit JSON DTO ↔ proto mapping** (not protojson) to guarantee snake_case field names
  and the exact `202 {request_id}` / error-envelope shapes.
- **In-house JWT/JWKS verify** keeps tests hermetic and offline; RS256 primary, EdDSA
  supported; fail-closed on unverifiable/expired.
- **Rate limiter** is an interface with an in-memory fixed-window default; Valkey is
  optional (documented) — the only stateful bit, per the spec.
- **BFF aggregation**: `GET /v1/generations/{id}` collapses `GetGenerationStatus` +
  `GetIdeas` into one payload.

## Definition of done

`go build ./...` and `go test ./...` green (reported verbatim). README with env vars,
run/docker/test instructions. `openapi.yaml` present and served.
