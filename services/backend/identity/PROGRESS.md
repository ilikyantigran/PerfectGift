# identity — Progress

_Contract: services/backend/identity/SERVICE.md · house style: knp-service_

## Done
- [x] Scaffolded module + tree (cmd, configs, internal, pkg/api, migrations)
- [x] proto written (`api/identity/v1/identity.proto`) + `make vendor-proto` + `make generate` clean
- [x] config.go (+ values_local/docker.yaml), telemetry (verbatim), docs.go, main.go, Dockerfile, Makefile
- [x] internal/token: Ed25519 JWT issue/verify + JWKS + rotation — tests GREEN
- [x] internal/password: Argon2id PHC hash/verify — tests GREEN
- [x] internal/oauth: Verifier iface + FakeVerifier + real ProviderVerifier (hermetic httptest JWKS) — tests GREEN

## Next
- [ ] internal/app/identity_server.go — 6 RPC handlers + store interfaces
- [ ] internal/app/fakes_test.go + identity_server_test.go
- [ ] internal/domain/valkey (Sessions + RateLimiter)
- [ ] internal/domain/postgres (Users + embedded migrations) + guarded integration test
- [ ] wire app.go Run()
- [ ] go build ./... + go test ./... green; README.md

## Notes / decisions
- EdDSA access tokens; refresh = "{sid}.{secret}", sha256(secret) stored in Valkey session:{sid}.
- SignIn EMAIL = sign-in-or-register (no Register RPC in contract) — documented assumption.
- gRPC status codes for errors; generic credential errors (no enumeration).
- GetMe subject from `authorization` metadata (Bearer), forwarded by grpc-gateway.
