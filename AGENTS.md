# AGENTS.md — Go Microservice Boilerplate + High-Throughput Event Ingestion

> Architecture: Event-Driven (EDA) + Clean Architecture + Domain-Driven Design
> Module: `github.com/boilerplate/microservice-boilerplate`
> Go: 1.23
> See [ARCHITECTURE.md](ARCHITECTURE.md) for the full system diagram.

---

## Project Overview

A production-ready Go microservice boilerplate with two reference domains:

- **`internal/product/`** — a standard CRUD domain (Create/Read/List/Update/Delete)
  with Redis caching and Kafka event publishing on writes. Uses GORM.
- **`internal/analytics/`** — a high-throughput event *ingestion* domain: an
  HTTP endpoint accepts events, publishes to Kafka asynchronously, and a
  separate consumer batches records into Postgres via `COPY`. This domain
  intentionally bypasses GORM for its hot write path — see
  ["The GORM exception"](#the-gorm-exception-internalanalytics) below.

Clone and extend by adding new domains under `internal/<name>/` following
`internal/product/`'s conventions for standard CRUD work, or
`internal/analytics/`'s conventions for high-volume write-heavy ingestion.

---

## Tech Stack

| Concern | Package | Notes |
|---|---|---|
| HTTP framework | `gin-gonic/gin` | Fast router, excellent middleware ecosystem |
| ORM | `gorm.io/gorm` | Auto-migrate, clean query API. **Not used on the `internal/analytics` bulk-write path — see exception below.** |
| DB drivers | `gorm.io/driver/postgres`, `gorm.io/driver/mysql`, `github.com/glebarez/sqlite`, `gorm.io/driver/sqlserver` | Selected at runtime via `DB_DRIVER` |
| Read/write split | `gorm.io/plugin/dbresolver` | Random replica policy, configurable per domain |
| Kafka client | `segmentio/kafka-go` | Pure Go, no CGO. See [decision note](#decision-kafka-client) — this repo standardized on `kafka-go` for both domains rather than introducing `franz-go` alongside it. |
| Redis client | `redis/go-redis/v9` | Context-aware, full pool config |
| Configuration | `spf13/viper` | Env vars + `.env` file, typed getters |
| Logging | `uber-go/zap` | Structured, zero-allocation |
| OTel SDK | `go.opentelemetry.io/otel` | Vendor-neutral traces + metrics |
| OTel Gin | `otelgin` | Auto-instrument all Gin routes |
| OTel GORM | `uptrace/opentelemetry-go-extra/otelgorm` | Auto-instrument all GORM queries (product domain only) |
| OTel OTLP exporters | `otlptracehttp` + `otlpmetricgrpc` | Export to any OTLP-compatible backend |
| Swagger | `swaggo/swag` | Generated from handler annotations |

### Decision: Kafka client

This boilerplate originally specified `segmentio/kafka-go` for the product
domain and considered `twmb/franz-go` for `internal/analytics`, on the
theory that per-record producer overhead and batch throughput directly
determine ingest capacity. **Resolution: this repo standardized on
`kafka-go` for both domains.** `kafka-go` was already installed and in use
by the product domain; adding a second Kafka client for one domain is a
real cost (two dependency surfaces, two configuration shapes, two things to
patch on CVEs) that wasn't justified without a benchmark showing `kafka-go`
is actually the bottleneck. If ingest throughput becomes provably
Kafka-client-bound in practice, benchmark `franz-go` under real load before
switching — see Good First Task #7. Don't default silently in either
direction; this note records that the silent default was deliberately
avoided and `kafka-go` was the measured/reasoned choice, not an accident.

---

## Architecture Layout

```
microservice-boilerplate/
├── AGENTS.md
├── README.md
├── Dockerfile                          # Multi-stage: builder (alpine) + runtime (distroless)
├── .dockerignore
├── go.mod
├── go.sum
├── .env.example
├── docker-compose.yml                  # Postgres, Kafka, Redis, OTel Collector
├── Makefile
├── otel-collector-config.yaml
├── cmd/
│   ├── web/
│   │   └── main.go                     # HTTP server: config → OTel → DB → Redis → Kafka → handler → router
│   ├── worker/
│   │   └── main.go                     # Kafka consumer: analytics domain's batch-insert loop lives here
│   └── scheduler/
│       └── main.go                     # Scheduled jobs (e.g. partition creation, rollup refresh)
├── api/
│   ├── http/
│   │   ├── v1/
│   │   │   ├── route/router.go
│   │   │   ├── middleware/
│   │   │   │   ├── correlation.go
│   │   │   │   ├── tracing.go
│   │   │   │   ├── logger.go
│   │   │   │   └── error_handler.go
│   │   │   └── response/response.go
│   │   └── v2/
│   └── grpc/
├── config/
│   └── config.go
├── docs/
├── internal/
│   ├── product/                        # Reference CRUD domain (see original layout — unchanged)
│   │   ├── domain/{entity,event,repository,request,response}/
│   │   ├── usecase/
│   │   ├── repository/product_repository.go   # GORM
│   │   ├── cache/product_cache.go
│   │   ├── gateway/messaging/product_publisher.go
│   │   └── delivery/http/product_handler.go
│   └── analytics/                      # High-throughput event ingestion domain
│       ├── domain/
│       │   ├── entity/
│       │   │   └── event.go            # Event struct — event_id, user_id, event_type, payload, occurred_at
│       │   ├── event/                  # (Kafka wire events, if analytics itself emits derived events)
│       │   ├── repository/
│       │   │   └── event_repository.go # Interface: InsertBatch(ctx, []Event) error; HourlyCounts(ctx, ...)
│       │   └── request/
│       ├── usecase/
│       │   └── ingest_usecase.go       # PublishEvent(ctx, Event) — validates, hands to gateway, returns immediately
│       ├── repository/
│       │   ├── event_repository.go     # Raw pgx + COPY implementation — see GORM exception below
│       │   └── event_repository_test.go
│       ├── gateway/messaging/
│       │   ├── event_publisher.go      # Kafka producer (async, non-blocking)
│       │   └── event_consumer.go       # Kafka consumer group + batching logic (size-or-time)
│       └── delivery/http/
│           └── ingest_handler.go       # POST /events — Gin handler, returns 202 immediately
└── pkg/
    ├── databases/db.go                 # NewDB — GORM multi-driver factory (product domain)
    ├── databases/rawdb.go              # NewRawPool — pgxpool for analytics domain's COPY path
    ├── redis/client.go
    ├── kafka/kafka.go                  # kafka-go writer + reader factories (both domains)
    ├── telemetry/provider.go
    ├── tracing/tracing.go
    ├── viper/viper.go
    ├── logger/logger.go
    ├── crypto/crypto.go
    ├── http/client.go
    └── utils/
```

---

## Layer Responsibilities

### `domain/`
- Plain Go structs — GORM model tags allowed on entities used by GORM-backed domains
- Event structs + `Publisher` interface
- Repository + cache interfaces (contracts only, no implementations)
- **Zero imports from any other internal layer, zero external framework imports**

### `usecase/`
- Interface + implementation for every use case
- Depends only on `domain/` interfaces — never on concrete implementations
- Propagates `context.Context` to all I/O calls
- Creates OTel spans (`otel.Tracer("<domain>/usecase").Start(...)`)
- **`analytics.IngestUseCase` specifically**: publishes to Kafka and returns.
  Never calls the repository directly — ingest must not depend on Postgres
  being reachable.

### `repository/`
- **Product domain**: only layer allowed to import `gorm`. Creates OTel spans
  per method; `otelgorm` auto-creates child spans for each SQL query. Maps
  `gorm.ErrRecordNotFound` → domain sentinel errors.
- **Analytics domain**: uses `pgxpool` directly (see
  [GORM exception](#the-gorm-exception-internalanalytics)). Bulk writes go
  through a staging-table + `COPY` + `INSERT ... ON CONFLICT DO NOTHING`
  pattern, not `gorm.CreateInBatches`.

### `cache/`
- Only layer allowed to import `go-redis`
- Creates OTel spans per method with `cache.key` and `cache.hit` attributes
- Returns `nil, nil` on cache miss so callers fall through to the DB
- Not used by `internal/analytics` — there's nothing to cache on a
  write-mostly ingest path; don't add caching here without a concrete read
  pattern that needs it.

### `gateway/messaging/`
- Only layer allowed to import the Kafka client
- Creates OTel spans and injects W3C trace context into Kafka message
  headers via `propagation.TextMapCarrier`
- **Product domain**: publish failures are non-fatal (best-effort) — do not
  roll back the DB write, since the write already succeeded before publish.
- **Analytics domain**: publish is the *primary* write path (there is no
  prior DB write to roll back). A publish failure here must surface to the
  HTTP handler as an error — see `ingest_handler.go`.

### `delivery/http/`
- Gin handlers only — no router setup, no middleware
- Creates OTel spans per handler method
- **`analytics.IngestHandler`**: returns `202 Accepted` the moment the event
  is handed to Kafka — never waits on a DB write. This is the single most
  important behavioral contract in this domain; do not add a synchronous
  DB call here for any reason (validation, dedup checks, etc. — all of
  that happens downstream in the consumer).

### `api/http/v1/`
Unchanged from the original boilerplate — see middleware stack order below.

Middleware stack order:
```
RequestTimer → CorrelationID → otelgin → TraceContext → Logger → ErrorHandler
```

### `pkg/`
- Domain-agnostic, process-wide infrastructure — no imports from `internal/`
- `databases/db.go` (GORM) and `databases/rawdb.go` (pgxpool) coexist
  deliberately — see the exception below for which domains use which.

### `cmd/`
- Thin wiring only: load config → init OTel → connect infra → build use
  cases → start server / consumer loop → graceful shutdown
- `cmd/worker/main.go` runs the analytics domain's Kafka consumer group,
  batching records and calling `event_repository.InsertBatch`.

---

## The GORM exception (`internal/analytics`)

The boilerplate's general rule is **"no raw `database/sql` or `pgx` — always
go through GORM."** `internal/analytics/repository/event_repository.go` is
the one deliberate, documented exception to that rule, for one reason:
GORM's batch insert (`CreateInBatches`) is still row-oriented SQL under the
hood. Postgres's `COPY` protocol is typically 10-50x faster for bulk loads,
and that difference is the entire point of a high-throughput ingest domain.

Rules for this exception:

1. **Scope is narrow.** Only `internal/analytics/repository/` uses
   `pgxpool` directly, via `pkg/databases/rawdb.go`. Every other domain
   (`internal/product/` and any new CRUD domain you add) uses GORM as
   normal. Don't let the exception spread.
2. **The pattern must stay staging-table + `COPY` + `INSERT ... ON CONFLICT
   DO NOTHING`** — not a raw `INSERT` loop. If you're not using `COPY`,
   there's no reason for the exception to exist; go back to GORM.
3. **Idempotency is still required.** `event_id` + `occurred_at` unique
   constraint, `ON CONFLICT DO NOTHING`, exactly as in a GORM-backed domain
   — Kafka's at-least-once delivery doesn't care which DB driver you used.
4. **If you add a second high-throughput domain**, it may reuse this
   pattern, but don't generalize it into a framework prematurely — copy
   the pattern deliberately and adjust, the same way `internal/product` is
   meant to be copied for new CRUD domains.

---

## Dependency Rules (Hard Constraints)

```
domain                          → nothing internal
usecase                         → domain
repository (product)            → domain, gorm
repository (analytics)          → domain, pgxpool   [documented exception, see above]
cache                           → domain
gateway                         → domain
api/http/v1/middleware          → nothing internal
api/http/v1/response            → nothing internal
api/http/v1/route               → delivery, middleware, response, internal/*/domain (error mapper only)
delivery/http                   → usecase, domain, api/http/v1/response
cmd/*                           → all layers (wiring only)
pkg/*                           → nothing internal
```

---

## Observability (OpenTelemetry)

### Span waterfall — product domain (unchanged)

```
otelgin: HTTP <METHOD> <PATH>
  product.handler.<Method>
    product.usecase.<Action>
      cache.product.<op>
      product.repository.<Method>
        gorm: <SQL>
      cache.product.set
    kafka.publish.<event>
```

### Span waterfall — analytics domain

```
otelgin: HTTP POST /events
  analytics.handler.Ingest
    analytics.usecase.PublishEvent
      kafka.publish.raw-events          ← async; span ends when handed to producer, not on ack

# separately, in cmd/worker (no HTTP parent span — root span per batch):
analytics.consumer.batch
  analytics.repository.InsertBatch
    pgx.copy: staged_events
    pgx.exec: insert_from_staging
```

Batches don't have a single incoming request to inherit a trace from, so
`analytics.consumer.batch` starts a fresh root span per flush, with
`batch.size` and `batch.trigger` (`"size"` or `"timeout"`) as span attributes.

### Trace propagation

Same as original: `otelgin` extracts W3C `traceparent`/`tracestate` on
inbound HTTP; `TraceContext` middleware stores `trace_id`/`span_id` in Gin
context. Outgoing Kafka messages (both domains) inject trace context via
`otel.GetTextMapPropagator().Inject(ctx, &kafkaHeaderCarrier)`.

### Span instrumentation points

| Layer | Tracer name | Span name pattern |
|---|---|---|
| Product handler | `product/handler` | `product.handler.<Method>` |
| Product usecase | `product/usecase` | `product.usecase.<Action>` |
| Product repository | `product/repository` | `product.repository.<Method>` |
| Product cache | `product-service/cache` | `cache.product.<op>` |
| Product Kafka publisher | `product-service/kafka` | `kafka.publish.<EventName>` |
| Analytics handler | `analytics/handler` | `analytics.handler.Ingest` |
| Analytics usecase | `analytics/usecase` | `analytics.usecase.PublishEvent` |
| Analytics consumer | `analytics/consumer` | `analytics.consumer.batch` |
| Analytics repository | `analytics/repository` | `analytics.repository.InsertBatch` |

All spans call `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)` on failure.

### OTel provider setup, OTel Collector

Unchanged from the original boilerplate.

---

## Database (`pkg/databases`)

### `NewDB(cfg Config) (*gorm.DB, error)` — product domain and any new CRUD domain

Unchanged: selects dialector via `DB_DRIVER` (`postgres` / `mysql` /
`sqlite` / `sqlserver`), configures pool settings, wires `dbresolver` for
read/write split.

### `NewRawPool(ctx context.Context, dsn string) (*pgxpool.Pool, error)` — analytics domain only

Postgres-only (bulk `COPY` isn't a portable concept across the multi-driver
abstraction). Used exclusively by `internal/analytics/repository`. Pool size
should be tuned separately from the GORM pool — batch writes are fewer,
larger transactions, not many small ones.

### Schema notes for `analytics`

The `events` table should be time-partitioned (`PARTITION BY RANGE
(occurred_at)`) with a BRIN index on `occurred_at` rather than the default
B-tree GORM would generate — BRIN stays tiny for time-ordered inserts at
volume, and partitioning lets you drop old data cheaply. This isn't
something `AutoMigrate` expresses well; keep this table's schema in a plain
SQL migration file rather than a GORM model, even though the Go-side
`entity.Event` struct still exists for type safety in application code.

---

## Event Architecture

### Event contract (both domains)

```go
type Event interface {
    EventName() string
    OccurredAt() time.Time
}
```

### Topics

| Topic | Domain | Trigger |
|---|---|---|
| `product.created` | product | Product created |
| `product.updated` | product | Product updated |
| `raw-events` | analytics | Client-submitted event, published by ingest handler |
| `dead-letter-events` | analytics | *(planned — see Good First Tasks)* malformed or repeatedly-failing records |

### Rules

- **Product domain**: publish **after** a successful DB write, never before;
  publish failure is non-fatal.
- **Analytics domain**: publish **is** the write (there's no prior DB write
  to sequence after); publish failure must be surfaced as an ingest error.
- Both: consumers must be idempotent (at-least-once delivery).
- Both: always inject OTel trace context into Kafka message headers.
- **Analytics consumer specifically**: commit Kafka offsets only *after* a
  batch is durably written to Postgres — never before. Batch by size OR
  time (whichever comes first), not by size alone or time alone.

---

## Coding Conventions

### General
- No `panic()` in application code — always return errors
- Wrap errors: `fmt.Errorf("context: %w", err)`
- No global state — inject all dependencies via constructors
- `context.Context` is the first parameter on all I/O functions

### Naming
- Files: `snake_case.go`
- Interfaces: noun, no `I` prefix (`ProductRepository`, `EventRepository`)
- Use case structs: `<Action><Entity>UseCase` (`CreateProductUseCase`, `PublishEventUseCase`)
- Handler structs: `<Entity>Handler` (`ProductHandler`, `IngestHandler`)
- Constructors: `New<Type>`

### Dependency injection
- Wire manually in `cmd/*/main.go` — no DI framework (no Wire, no Fx)
- Order: telemetry → DB (GORM + raw pool, as needed) → Redis → Kafka →
  infrastructure → use cases → handler → router → server
- Pass interfaces, not concrete types, to constructors

### Database
- Product domain: `AutoMigrate` in `cmd/web/main.go` on startup
- Analytics domain: plain SQL migration file (partitioning + BRIN aren't
  well-expressed by `AutoMigrate`) — run manually or via your migration
  tool of choice, not `AutoMigrate`
- `*gorm.DB` never leaves `internal/product/repository/`
- `*pgxpool.Pool` never leaves `internal/analytics/repository/`

### List endpoints (search / sort / preload)
- Convention established by `internal/product`'s `GET /api/v1/products` —
  follow this shape for any new CRUD domain's list endpoint.
- Query params: `search` (substring match), `sort_by` + `sort_dir`
  (`asc`/`desc`), `preload` (comma-separated relation names).
- **`sort_by` and `preload` values must be validated against a fixed
  allow-list in the usecase layer before being passed to the repository.**
  GORM's `Order()` and `Preload()` do not parameterize/escape arbitrary
  input the way `Where()` placeholders do — passing a raw query-string
  value straight into `Order()` is a SQL-injection-shaped bug even though
  it goes through GORM. See `internal/product/usecase/list_products.go`'s
  `allowedSortFields` / `allowedPreloads` maps. Unrecognized values return
  a domain sentinel error (`ErrInvalidSortField` / `ErrInvalidPreload`)
  mapped to HTTP `400`, never silently ignored or passed through.
- Search is a portable `LOWER(col) LIKE ?` across all four GORM drivers —
  not driver-specific full-text search (`tsvector`, `FULLTEXT`, FTS5,
  `CONTAINS`). This is a deliberate simplification (`ponytail:` comment in
  `product_repository.go`): fine for a boilerplate reference domain;
  swap for Postgres `tsvector` + GIN index if ranked relevance is ever
  needed, rather than silently defaulting either way.
- `preload`'s allow-list may be empty for a domain with no relations
  (e.g. `Product` today) — that's correct, not a bug; every `preload`
  value is then rejected until a real relation is added.

### Error handling
- Domain sentinel errors defined in `domain/entity/`
- HTTP status mapping in `api/http/v1/route/router.go` (closure passed to `ErrorHandler`)
- Record errors on the OTel span at the layer that catches them

### Testing
- Use case tests: mock repository/cache/publisher interfaces — no real infra
- Repository integration tests: real DB via `DB_DSN` (product) or
  `ANALYTICS_DB_DSN` (analytics) env var; skipped when absent (`t.Skip`)
- Analytics consumer batching logic (size trigger vs. timeout trigger) is
  pure logic — test it without Kafka/Postgres at all
- Table-driven tests preferred
- Test files: `<file>_test.go` in the same package

---

## Docker

Multi-stage Dockerfile, distroless runtime — unchanged from original.
`CGO_ENABLED=0` remains safe: `pgx` (used by the analytics domain) is
pure-Go, same as every GORM driver already in the stack.

### Makefile targets

```bash
make docker-build [IMAGE_NAME=...] [IMAGE_TAG=...]
make docker-run
make docker-push [IMAGE_NAME=...] [IMAGE_TAG=...]
```

---

## Environment Variables

### Application
| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8003` | HTTP server port |
| `GIN_MODE` | `debug` | `debug` or `release` |

### Database (product domain — GORM)
| Variable | Default | Description |
|---|---|---|
| `DB_DRIVER` | `postgres` | `postgres` · `mysql` · `sqlite` · `sqlserver` |
| `DB_DSN` | postgres://… | Primary connection string |
| `DB_REPLICA_DSNS` | — | Comma-separated replica DSNs |
| `DB_MAX_OPEN_CONNS` | `25` | Pool: max connections |
| `DB_MAX_IDLE_CONNS` | `10` | Pool: idle connections |
| `DB_CONN_MAX_LIFETIME` | `5m` | Pool: max connection age |
| `DB_CONN_MAX_IDLE_TIME` | `5m` | Pool: max idle time |

### Database (analytics domain — raw pgxpool)
| Variable | Default | Description |
|---|---|---|
| `ANALYTICS_DB_DSN` | postgres://… | Postgres-only; used by `pkg/databases.NewRawPool` |
| `ANALYTICS_DB_MAX_CONNS` | `10` | Separate from the GORM pool — batch writes are fewer, larger transactions |
| `BATCH_SIZE` | `500` | Consumer flush trigger (record count) |
| `BATCH_TIMEOUT` | `200ms` | Consumer flush trigger (max wait) |

### Redis
| Variable | Default |
|---|---|
| `REDIS_ADDR` | `localhost:6379` |
| `REDIS_PASSWORD` | — |
| `REDIS_DB` | `0` |
| `REDIS_POOL_SIZE` | `10` |
| `REDIS_MIN_IDLE_CONNS` | `5` |
| `REDIS_MAX_RETRIES` | `3` |
| `REDIS_DIAL_TIMEOUT` | `5s` |
| `REDIS_READ_TIMEOUT` | `3s` |
| `REDIS_WRITE_TIMEOUT` | `3s` |

### Kafka
| Variable | Default | Options |
|---|---|---|
| `KAFKA_BROKERS` | `localhost:9092` | comma-separated |
| `KAFKA_TOPIC` | `raw-events` | analytics domain's ingest topic |
| `KAFKA_CONSUMER_GROUP` | `analytics-processor` | analytics domain's consumer group |
| `KAFKA_BATCH_SIZE` | `100` | producer batch size |
| `KAFKA_REQUIRED_ACKS` | `all` | `all` · `leader` · `none` |
| `KAFKA_COMPRESSION` | `snappy` | `none` · `gzip` · `snappy` · `lz4` · `zstd` |

### OpenTelemetry
| Variable | Default | Notes |
|---|---|---|
| `OTEL_SERVICE_NAME` | `product-service` | Read by app and OTel SDK — set per binary (`web`, `worker`, `scheduler`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | Read by OTel SDK directly |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | `localhost:4317` | gRPC — no `http://` prefix |

---

## Commands

```bash
# Infrastructure
make infra-up / infra-down

# Development
make run              # go run ./cmd/web
make worker           # go run ./cmd/worker   (analytics consumer)
make build            # swagger regen + go build ./...
make test             # go test ./...
make test-race        # go test -race ./...
make lint             # golangci-lint run ./...
make tidy             # go mod tidy
make load-test        # fire load at POST /events, report p50/p95/p99

# Docker
make docker-build
make docker-run
make docker-push
```

---

## Constraints for AI Agents

- **No DI framework** — wire manually in `cmd/*/main.go`
- **No raw `database/sql` or `pgx` in `internal/product/` or any new CRUD
  domain** — use `pkg/databases.NewDB`; never bypass GORM there.
- **`internal/analytics/repository/` is the sole, documented exception** —
  it uses `pgxpool` for `COPY`-based bulk writes. Don't extend this
  exception to other domains without a concrete throughput justification,
  and don't "clean up" this domain by rewriting it to use GORM — that
  reintroduces the exact bottleneck it exists to solve.
- **`*gorm.DB` stays in product-style `repository/`; `*pgxpool.Pool` stays
  in `internal/analytics/repository/`** — never pass either to usecase,
  domain, or handler layers.
- **`repository/` and `gateway/` never imported by `domain/` or
  `usecase/`** — only interfaces cross that boundary.
- **`pkg/` has no `internal/` imports** — must remain domain-agnostic.
- **No `init()` functions** — explicit initialisation only.
- **OTel spans belong in `delivery/`, `usecase/`, `repository/`, `cache/`,
  and `gateway/`** — not in `domain/`; always propagate `context.Context`.
- **Always call the OTel `shutdown` function on graceful shutdown.**
- **`internal/analytics` ingest handler never performs a synchronous
  Postgres call** — validation and Kafka publish only. This is the single
  most important behavioral rule in the analytics domain; treat any change
  that adds a DB call to the ingest path as a regression, not a feature.
- **`internal/analytics` consumer commits Kafka offsets only after a batch
  is durably written** — preserve this ordering in any change to the
  consumer loop.
- **New Kafka topics must be added to the Topics table above.**
- **New CRUD domains follow `internal/product/`'s structure. New
  high-throughput ingestion domains follow `internal/analytics/`'s
  structure** — pick the one that matches the domain's actual read/write
  shape rather than defaulting to whichever one you copied last.

---

## Good First Tasks

Ordered roughly by how self-contained each is. One task per PR.

1. **Tests** for `analytics.usecase.PublishEvent` (validation) and the
   consumer's batching logic (size trigger vs. timeout trigger) — pure
   logic, no infra needed.
2. **Dead-letter path**: records that fail JSON unmarshal or repeatedly
   fail `InsertBatch` should go to `dead-letter-events` instead of being
   logged and dropped.
3. **Automate partition creation** for the `events` table — `pg_partman`,
   or a scheduled job in `cmd/scheduler` that creates next month's
   partition ahead of time.
4. **Hourly rollup job** in `cmd/scheduler`: populate an
   `event_hourly_rollup` table from raw `events`, then switch the
   analytics read endpoint to query the rollup instead of aggregating raw
   rows live.
5. **Prometheus `/metrics`** on `cmd/web` and `cmd/worker` — ingest rate,
   batch size distribution, insert latency, consumer lag.
6. **Graceful degradation**: if Kafka is unreachable, `POST /events`
   should return `503` with a clear error rather than hanging.
7. **Benchmark `kafka-go` vs `franz-go`** under realistic load if ingest
   throughput is ever suspected to be client-bound — this repo already
   standardized on `kafka-go` for both domains (see
   [decision note](#decision-kafka-client)); only revisit with numbers.
