# catalog — Progress

_Contract: services/backend/catalog/SERVICE.md · updated: 2026-07-07_

## Done
- [x] Explored SERVICE.md, architecture.md, knp-service reference, templates, toolchain
- [x] PLAN.md written + self-approved (pre-authorized)

## Next
- [ ] Scaffold from templates (cmd, configs, internal/infra, Dockerfile, Makefile)
- [ ] go mod init + copy vendor-proto
- [ ] Write proto + make generate
- [ ] config.go sections
- [ ] embedding pkg (interface + http + fake) + tests
- [ ] postgres store + migration + env-guarded integration test
- [ ] valkey cache store
- [ ] catalog_server.go handlers + unit tests (TDD)
- [ ] wire app.go
- [ ] README + verify build/test

## Notes / decisions / blockers
- Module: github.com/ilikyantigran/PerfectGift/services/backend/catalog
- Ports gRPC 9096 / HTTP 8096. Embedding default model text-embedding-3-small dim 1536.
- Integration test env-guarded by CATALOG_TEST_DB_DSN (hermetic default).
- Fake embedder is local/default so service boots without an embedding API.
