# api-gateway — Progress

_Contract: services/backend/api-gateway/SERVICE.md · house style: /backend-service_

## Done
- [x] Read all 6 SERVICE.md + architecture.md
- [x] PLAN.md written

## Next
- [ ] go mod init + go.mod pinned to knp versions
- [ ] Write 5 client protos (api/*/v1/*.proto), Makefile, `make generate`
- [ ] config.go + configs/*.yaml
- [ ] auth verifier (test-first)
- [ ] ratelimit limiter (test-first)
- [ ] rest errors + middleware (test-first)
- [ ] rest handlers + router (test-first, fakes)
- [ ] clients.go + app.go + telemetry + docs + openapi.yaml
- [ ] Dockerfile, README
- [ ] go build ./... + go test ./... green; boot check

## Notes / decisions
- Gateway is HTTP-only + gRPC CLIENT (inverse of house template). Protos have NO
  google.api.http annotations → codegen needs no vendor-proto, no network.
- In-house JWT/JWKS verify (RS256/EdDSA), stdlib net/http router. Deps in module cache.
- Fakes = generated <Svc>Client interfaces.
