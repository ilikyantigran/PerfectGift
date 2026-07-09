# notification — Progress

_Contract: SERVICE.md · house style: backend-service skill · status: COMPLETE_

## Done
- [x] Explore: SERVICE.md, architecture.md, poll/surprise event shapes, skill refs, knp-service
- [x] PLAN.md written (approval pre-authorized)
- [x] Scaffolded module (go.mod, dirs, vendor-proto copied from knp-service, infra templates, Makefile, Dockerfile)
- [x] Proto written + `make generate` clean (pb, grpc, gateway, swagger)
- [x] Config struct + values_{local,docker}.yaml
- [x] migrations/0001_init.sql (devices + notifications outbox)
- [x] Domain: types, Store/Pusher/Subscription interfaces
- [x] TDD notify pkg: enqueue dedupe, consumer decode, ack/nak, dispatcher
      (happy/zero/transient/max-attempts/dead-token/concurrency/crash/resolve-err) — 14 tests
- [x] TDD gRPC server: RegisterDevice upsert/validation, UnregisterDevice idempotent — 5 tests
- [x] Real adapters: pgx Store, NATS JetStream Subscription, APNs (ES256) + FCM (v1) Pushers
- [x] Wired App (2 consumers + dispatcher + gRPC/HTTP via errgroup, graceful shutdown)
- [x] Verify: `go build ./...` OK, `go vet` OK, `go test ./...` GREEN, `-race` GREEN, gofmt clean, config boot OK
- [x] README.md

## Notes / decisions
- Module: github.com/ilikyantigran/PerfectGift/services/backend/notification
- Outbox row = per (user,event); fan-out at dispatch; at-least-once + dedupe_key + atomic lease-claim
- Lease unifies retry scheduling and crash recovery (no separate reaper)
- All external deps behind interfaces with fakes → hermetic tests; real adapters compiled, not unit-tested
- NATS subjects chosen (no producer built yet): events.poll.completed / events.surprise.ideas_ready
