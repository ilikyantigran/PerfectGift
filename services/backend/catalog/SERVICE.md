# Catalog Service — Service Specification

> **Source of truth for building this service from scratch.** Read together with the root
> [`architecture.md`](../../../architecture.md). The architecture wins on any conflict.

---

## 1. What this service is for

Catalog is the **read-mostly reference and grounding** service. It holds two kinds of data:

1. **Reference data** clients need — holidays (with dates/regions), gift/date categories,
   budget bands.
2. A **curated inspiration corpus** — short, editorially-vetted idea seeds, **embedded
   (pgvector)** so the Surprise Service can pull the most relevant grounding for each
   request.

That corpus is **the main quality lever of the whole product**: it is what keeps generated
ideas concrete and on-brand instead of generic LLM filler. Building it is an editorial
investment, not just a schema.

Its write pattern is the opposite of everything else: **rare admin writes, massive
cache-hit reads.** That distinct profile is why it is its own service.

**In scope**
- Serve holidays, categories, budget bands to clients.
- Serve **semantic grounding** to Surprise via pgvector similarity search.
- Own the curated corpus and its embeddings; admin/editorial write path.
- Aggressive caching (reference data changes rarely).

**Not in scope**
- Generation (Surprise). Auth (Identity). Any user/poll data.

---

## 2. Ownership & data

Owns its own **PostgreSQL + pgvector** schema and a **Valkey** cache.

### PostgreSQL (`catalog` schema, + pgvector)
- **`holidays`** — `id`, `name`, `date_rule (fixed|relative)`, `region`, `tags (jsonb)`,
  `active`.
- **`categories`** — `id`, `name`, `kind (gift|date)`, `parent_id (nullable)`.
- **`budget_bands`** — `id`, `label`, `min`, `max`, `currency`.
- **`inspiration`** — `id`, `title`, `body`, `category_id`, `budget_band_id`, `tags (jsonb)`,
  `embedding (vector)`, `curated_by`, `curated_at`, `active`. The **grounding corpus**.

### Valkey
- **Hot reference reads** — cached holidays/categories/budget bands. Cache aggressively;
  invalidate on the rare admin write.

---

## 3. Contracts

### 3.1 gRPC API (`catalog.v1`) — internal

| RPC | Request | Response | Notes |
|---|---|---|---|
| `ListHolidays` | `{ region?, active?, on_or_after? }` | `{ holidays[] }` | Cached; served to clients |
| `GetCategories` | `{ kind? }` | `{ categories[], budget_bands[] }` | Cached; served to clients |
| `SearchInspiration` | `{ query_text \| query_embedding, filters{category,budget}, top_k }` | `{ snippets[] }` | **pgvector similarity**; the grounding call for Surprise |
| `UpsertInspiration` | `{ inspiration }` (admin) | `{ id }` | Rare editorial write; (re)computes embedding, invalidates cache |

### 3.2 Events

None. Catalog is request/response only. (Cache invalidation is internal on write.)

### 3.3 Public REST surface (via gateway)

`GET /v1/holidays` → `ListHolidays`; `GET /v1/categories` → `GetCategories`. Both are
JWT-authenticated client reads. `SearchInspiration` is **internal only** (called by
Surprise), never exposed to clients.

---

## 4. Required integrations

| Integration | Direction | Protocol | Purpose |
|---|---|---|---|
| API Gateway | in | gRPC | Client reference reads (holidays, categories) |
| Surprise Service | in | gRPC (`SearchInspiration`) | pgvector grounding per generation |
| Admin/editorial tooling | in | gRPC (`UpsertInspiration`) | Maintain the curated corpus |
| Embedding source | out | HTTPS (or local) | Compute embeddings for corpus + queries — must match Surprise's embedding space |
| PostgreSQL + pgvector | out | SQL | reference data + corpus + embeddings |
| Valkey | out | RESP | hot reference read cache |
| Identity | — (indirect) | JWT | Client reads validated locally via JWKS |

**Embedding contract:** the vector model/dimension used to embed the corpus **must match**
what Surprise uses to embed the query, or similarity search is meaningless. Pin the model +
dimension and version it; re-embed the corpus on model change.

---

## 5. Tech stack & build notes

- **Language:** Go. gRPC internal. Postgres + pgvector. Valkey.
- **Migrations:** own the `catalog` schema incl. pgvector extension + embedding columns +
  vector index (e.g. HNSW/IVFFlat).
- **Config (env):** DB DSN, Valkey URL, embedding model/endpoint + dimension, cache TTLs.
- **Build order:** **step 6.** Replaces the hardcoded grounding seeds used during the early
  Surprise MVP with a real curated corpus + semantic retrieval — the main quality lever.
  Cold-start risk: a thin corpus yields generic ideas, so seed it with real editorial content.

## 6. Non-functional targets

- Reference reads: near-100% cache hits, p95 < 50 ms.
- `SearchInspiration` p95 comfortably within the generation budget (it runs inside the
  async worker, but should be fast — tens of ms).
- pgvector chosen over a dedicated vector DB (Pinecone/Weaviate) because volumes are modest
  and it keeps ops simple. Revisit only if vector volume explodes.
