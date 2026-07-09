# poll — Progress

_Contract: services/backend/poll/SERVICE.md · updated: 2026-07-09 · STATUS: COMPLETE_

## Done
- [x] Explore (SERVICE.md, architecture.md, skill refs, knp-service canonical)
- [x] PLAN.md written
- [x] Scaffold tree + go.mod + templates + vendor-proto (copied from knp)
- [x] proto poll.v1 (4 RPCs) + `make generate` clean (pb/grpc/gateway/swagger)
- [x] config struct + values_local.yaml + values_docker.yaml
- [x] domain/token (hash) + tests
- [x] domain/model (Poll/Question/Answer + ValidateQuestions/ValidateAnswers) + tests
- [x] infra/auth (HS256 JWT interceptor, token-derived subject) + tests
- [x] internal/ports (Repo/RateLimiter/Publisher interfaces — breaks app<->store cycle)
- [x] internal/app server: all 4 RPCs + security rules + tests (fakes, hermetic)
- [x] domain/postgres (pgx, migrations embedded, one-response guard tx) + guarded integ test
- [x] domain/valkey (fixed-window rate limiter) + guarded integ test
- [x] domain/events (NATS JetStream publisher)
- [x] wire App (telemetry -> stores -> server -> gRPC+gateway+CORS+swagger+metrics)
- [x] Dockerfile + Makefile + README.md
- [x] VERIFY: gofmt clean, go vet clean, `go build ./...` OK, `go test ./...` GREEN
- [x] VERIFY: binary boots, loads config, wires telemetry, reaches dep-connect stage cleanly

## Notes / decisions
- Module github.com/ilikyantigran/PerfectGift/services/backend/poll
- Ports package decouples server from stores (avoids import cycle) — server depends on interfaces, hermetic fakes in tests.
- JWT HS256 (JWKS deferred, swappable). Opaque token SHA-256 hashed. Uniform 404 for expired/invalid/revoked/consumed and non-owner.
- Rate limit BEFORE Postgres (per-token + per-IP) to absorb link-spam.
- Live end-to-end w/ real PG/Valkey/NATS not run: Docker daemon down in this env (not part of DoD). Guarded integ tests provided for when deps are available.
- Ports gRPC 8080 / HTTP 8081
