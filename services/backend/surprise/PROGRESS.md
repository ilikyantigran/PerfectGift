# surprise — Progress

_Contract: SERVICE.md · updated: 2026-07-09 · status: COMPLETE_

## Done
- [x] Scaffolded house-style layout; module `github.com/ilikyantigran/PerfectGift/services/backend/surprise`
- [x] proto `api/surprise/v1/surprise.proto` + `make generate` (pb, grpc, gateway, swagger) clean
- [x] Local downstream protos (poll, catalog) generated into pkg/api
- [x] config (env-selected YAML, secrets from env) + telemetry (copied) + docs (embedded swagger)
- [x] domain entities + Repository/Cache interfaces; in-memory store (memory)
- [x] resilience: circuit breaker + retry/backoff (+ tests)
- [x] llm: Client interface + deterministic fake + resilient decorator + raw-HTTP Anthropic client (+ tests)
- [x] events: NATS JetStream producer/durable consumer + in-memory Bus fake
- [x] clients: Poll/Catalog interfaces + gRPC stubs + fakes
- [x] postgres store + embedded migrations (surprise schema + pgvector)
- [x] valkey store (status, idempotency, LLM cache)
- [x] pipeline (generation algorithm §5) + worker (+ tests)
- [x] gRPC server: all 5 RPCs, validation, idempotency, owner-scoping (+ tests)
- [x] app wiring (telemetry → stores → NATS → clients → resilient LLM → worker → gRPC+HTTP)
- [x] main, configs (local+docker), Dockerfile, Makefile
- [x] README.md
- [x] `go build ./...` GREEN · `go vet ./...` clean · `go test ./...` GREEN (22 tests, hermetic)

## Notes / decisions
- Anthropic client is raw HTTP (no SDK dep); forced tool use `emit_ideas` for typed JSON.
- Embeddings: pseudo-embedding into vector(1536); swap for real provider matching Catalog in prod.
- Owner id from `x-user-id` metadata (gateway injects JWT subject); skipped when absent.
- Business failures mark request FAILED and return nil (no infinite redelivery); infra errors returned.
