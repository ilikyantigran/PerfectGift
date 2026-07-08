# notification — Progress

_Contract: SERVICE.md · house style: backend-service skill · updated: 2026-07-07_

## Done
- [x] Explore: read SERVICE.md, architecture.md, poll/surprise specs, skill refs, reference knp-service
- [x] PLAN.md written (approval pre-authorized)

## Next
- [ ] Scaffold module (go.mod, dirs, vendor-proto copy, infra templates, Makefile, Dockerfile)
- [ ] Write proto + `make generate`
- [ ] Config struct + configs + migrations
- [ ] Domain types + Store/Pusher/Subscription interfaces
- [ ] TDD: enqueue dedupe, consumer decode, dispatcher (happy/zero/retry/max/deadtoken/concurrency/crash), gRPC server
- [ ] Real adapters: pgx store, NATS subscription, APNs/FCM pushers
- [ ] Wire App (consumers + dispatcher goroutines + gRPC/HTTP)
- [ ] Verify: go build ./... + go test ./... green; README.md

## Notes / decisions
- Module: github.com/ilikyantigran/PerfectGift/services/backend/notification
- Outbox row = per (user,event); fan-out at dispatch; at-least-once + dedupe_key + atomic lease-claim
- Vendor protos copied from knp-service (no git allowed); protoc + plugins present
- All external deps behind interfaces with fakes → hermetic tests
