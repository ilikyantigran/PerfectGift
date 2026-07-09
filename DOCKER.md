# Running PerfectGift locally with Docker

The whole backend — six Go services plus their infrastructure (PostgreSQL + pgvector,
Valkey, NATS/JetStream) — runs from a single `docker compose` stack.

## Prerequisites
- Docker Desktop (or any running Docker daemon) with **Docker Compose v2+**.
- No Go toolchain needed — everything builds inside containers.

## Quick start
```bash
cp .env.example .env          # optional: add your ANTHROPIC_API_KEY
make up                       # == docker compose up --build -d
make ps                       # watch health; wait for infra to be healthy
```
Then open the gateway Swagger UI: **http://localhost:8080/swagger/**

Tear down:
```bash
make down                     # stop, keep the database
make clean                    # stop AND wipe the Postgres volume (fresh DB)
```

## What's in the stack

| Container | Image | Host port | Purpose |
|---|---|---|---|
| `gateway` | built | **8080** | Public REST/JSON + Swagger edge (the entrypoint) |
| `identity` | built | 8081 | Auth: sign-in, JWT, JWKS |
| `poll` | built | 8082 | Anonymous link polls |
| `surprise` | built | 8083 | LLM idea generation |
| `catalog` | built | 8084 | Reference data + pgvector grounding |
| `notification` | built | 8085 | Push fan-out (APNs/FCM disabled locally) |
| `postgres` | pgvector/pgvector:pg16 | 5432 | One DB per service |
| `valkey` | valkey/valkey:8-alpine | 6379 | Sessions / cache / rate limits |
| `nats` | nats:2.10-alpine | 4222, 8222 | JetStream events + jobs (monitoring on 8222) |

Each service's own Swagger + `/metrics` is on its host port (e.g. identity →
http://localhost:8081/swagger/). Internally every service listens on gRPC **:9090**
and HTTP **:8080** and is reachable by its compose name (`identity`, `poll`, …).

## How data is set up
- On first `up`, Postgres creates one database per service (`perfectgift` for
  identity, then `poll`, `surprise`, `catalog`, `notification`).
- **identity / poll / surprise** apply their own schema automatically at boot
  (embedded migrations).
- **catalog / notification** do not self-migrate, so their schema is applied by
  `deploy/postgres/initdb/00-init.sh` during Postgres init.
- `make clean` wipes the volume so the next `up` re-runs all of this from scratch.

## Trying it out
- **Health/metrics:** `curl http://localhost:8080/metrics` (gateway), or any service port.
- **Reference data:** `GET http://localhost:8080/v1/holidays` (via Swagger UI).
- **Anonymous poll flow** works with no auth (`GET /v1/polls/token/{t}`).
- **Generation** (`POST /v1/generations`) enqueues a job; the surprise worker calls
  Anthropic — set `ANTHROPIC_API_KEY` in `.env` for it to actually produce ideas.
- Inspect infra: `make psql` (Postgres shell), `make nats-info` (JetStream summary).

## Known caveats (local dev)
These are pre-existing cross-service gaps documented during the build, not compose bugs:

1. **Owner-authenticated Poll routes.** The Poll service verifies JWTs as HS256 with a
   shared secret, while Identity issues EdDSA tokens validated via JWKS. So `CreatePoll`
   and `GetResponses` (which need a user token) won't authenticate end-to-end until
   Poll is switched to Identity's JWKS. Anonymous poll routes are unaffected.
2. **Generation needs a real key.** Without `ANTHROPIC_API_KEY`, generation requests are
   accepted (202) but fail at the LLM call.
3. **Embedding space.** Surprise uses a placeholder embedding and Catalog a deterministic
   fake embedder locally; real semantic grounding requires both to use the same real
   embedding model.
4. **First-boot ordering.** The gateway may log a JWKS fetch error if it starts before
   Identity is serving; `restart: unless-stopped` lets it recover automatically.

## Rebuilding after code changes
```bash
make build            # rebuild all images
make restart S=poll   # restart a single service
make logs S=surprise  # tail one service's logs
```
> Note: services build from generated protobuf code committed under each service's
> `pkg/api/`. If you change a `.proto`, run that service's `make generate` before
> rebuilding its image.
