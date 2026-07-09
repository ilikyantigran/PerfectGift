# identity — Progress

_Contract: services/backend/identity/SERVICE.md · house style: knp-service · COMPLETE_

## Done
- [x] Scaffolded module + tree (cmd, configs, internal, pkg/api)
- [x] proto (`api/identity/v1/identity.proto`) + `make vendor-proto` + `make generate` clean
- [x] config.go (+ values_local/docker.yaml), telemetry (verbatim), docs.go, main.go, Dockerfile, Makefile
- [x] internal/token: Ed25519 JWT issue/verify + JWKS + rotation — tests GREEN
- [x] internal/password: Argon2id PHC hash/verify — tests GREEN
- [x] internal/oauth: Verifier iface + FakeVerifier + real ProviderVerifier (hermetic httptest JWKS) — GREEN
- [x] internal/model: shared User/Session value types (breaks app<->domain import cycle)
- [x] internal/app: all 6 RPC handlers + store interfaces — unit tests + transport e2e GREEN
- [x] internal/domain/valkey: Sessions + RateLimiter (valkey-go)
- [x] internal/domain/postgres: Users repo + embedded migrations + guarded integration test
- [x] wired app.go Run() (telemetry -> postgres+migrate -> valkey -> token mgr -> server -> gRPC+HTTP)
- [x] `go build ./...` GREEN, `go test ./...` GREEN, `go vet` clean
- [x] boot check: binary loads config+telemetry, stops cleanly at Postgres connect
- [x] README.md

## Notes / decisions
- EdDSA access tokens; refresh = "{sid}.{secret}", sha256(secret) in Valkey session:{sid}.
- SignIn EMAIL = sign-in-or-register (no Register RPC in contract) — documented assumption.
- gRPC status codes for errors; generic credential errors (no enumeration).
- GetMe subject from `authorization` metadata (Bearer), forwarded by grpc-gateway.
- Postgres integration tests skip unless IDENTITY_TEST_DATABASE_DSN set (hermetic default).
