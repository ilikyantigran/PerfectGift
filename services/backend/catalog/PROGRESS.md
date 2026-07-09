# catalog — Progress

_Contract: services/backend/catalog/SERVICE.md · updated: 2026-07-08 · STATUS: COMPLETE_

## Done
- [x] Explored SERVICE.md, architecture.md, knp-service reference, templates, toolchain
- [x] PLAN.md written + self-approved (pre-authorized)
- [x] Scaffolded from templates (cmd, configs, internal/infra, Dockerfile, Makefile)
- [x] go mod init + vendor-proto copied from knp
- [x] proto written (api/catalog/v1/catalog.proto) + `make generate` clean
- [x] config.go sections (postgres, valkey, embedding, catalog tuning) + both YAMLs
- [x] embedding pkg: Embedder iface + HTTP client + deterministic fake + tests (GREEN)
- [x] migrations/0001_init.sql: catalog schema, pgvector ext, tables, HNSW index
- [x] postgres store (pgx/v5, vector literals) + env-guarded integration test
- [x] valkey reference cache store
- [x] catalog_server.go: all 4 RPCs + mapping.go + unit tests vs fakes (GREEN)
- [x] app.go wired (embedder → postgres → valkey → server → grpc+gateway)
- [x] README.md
- [x] Verified: go build ./... + go vet + go test ./... GREEN; binary boots (config+
      fake embedder), REST surface = only /v1/holidays + /v1/categories

## Next
- (none — service complete to Definition of Done)

## Notes / decisions / blockers
- Module: github.com/ilikyantigran/PerfectGift/services/backend/catalog
- Ports gRPC 9096 / HTTP 8096. Embedding default text-embedding-3-small dim 1536.
- Integration test env-guarded by CATALOG_TEST_DB_DSN (skips hermetically). Docker
  daemon was not running in the build env, so it was not exercised live here.
- on_or_after accepted but not yet applied (rule-based holiday dates); reserved.
