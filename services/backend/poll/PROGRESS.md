# poll — Progress

_Contract: services/backend/poll/SERVICE.md · updated: 2026-07-07_

## Done
- [x] Explore (read SERVICE.md, architecture.md, skill refs, knp-service canonical)
- [x] PLAN.md written

## Next
- [ ] Scaffold tree + go.mod + templates + vendor-proto (copied from knp)
- [ ] proto poll.v1 + make generate
- [ ] config + yaml
- [ ] domain: postgres (+migrations), valkey, events(nats)
- [ ] auth interceptor
- [ ] server test-first (4 RPCs)
- [ ] wire App
- [ ] Dockerfile/Makefile/README
- [ ] verify: go build ./... , go test ./...

## Notes / decisions
- Module github.com/ilikyantigran/PerfectGift/services/backend/poll
- Ports (Repo/RateLimiter/Publisher) interfaces in internal/app; fakes for hermetic tests
- JWT HS256 (JWKS deferred). Opaque token SHA-256 hashed. Uniform 404 for expired/invalid/consumed.
- Ports gRPC 8080 / HTTP 8081
