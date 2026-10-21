# AGENTS.md — Go Product Microservice Boilerplate

> **Current scope:** this repository is a product-only CRUD baseline. The
> `internal/analytics/` domain, `cmd/worker`, raw `pgx` pool, analytics
> migration, and `/api/v1/events` endpoint have been removed. Any analytics
> capabilities must not be restored without an explicit new requirement.

> Architecture: Event-Driven (EDA) + Clean Architecture + Domain-Driven Design
> Module: `github.com/boilerplate/microservice-boilerplate`
> Go: 1.23
> See [ARCHITECTURE.md](ARCHITECTURE.md) for the full system diagram.

---

## Project Overview

A product CRUD baseline with Redis caching and best-effort Kafka publishing.
New CRUD domains follow `internal/product/`. Add infrastructure adapters behind
domain-owned interfaces when a concrete requirement needs them. Shared CRUD
contracts must stay small and optional; business operations and transactions
remain domain-specific. Avoid a global interfaces package.

---

## Tech Stack

| Concern | Package | Notes |
|---|---|---|
| HTTP framework | `gin-gonic/gin` | Fast router, excellent middleware ecosystem |
| ORM | `gorm.io/gorm` | Auto-migrate, clean query API. Used by CRUD repository adapters. |
| DB drivers | `gorm.io/driver/postgres`, `gorm.io/driver/mysql`, `github.com/glebarez/sqlite`, `gorm.io/driver/sqlserver` | Selected at runtime via `DB_DRIVER` |
| Read/write split | `gorm.io/plugin/dbresolver` | Random replica policy, configurable per domain |
| Kafka client | `segmentio/kafka-go` | Pure Go, no CGO. See [decision note](#decision-kafka-client) — this repo standardized on `kafka-go` for product publishing rather than introducing `franz-go` alongside it. |
| Redis client | `redis/go-redis/v9` | Context-aware, full pool config |
| Configuration | `spf13/viper` | Env vars + `.env` file, typed getters |
| Logging | `uber-go/zap` | Structured, zero-allocation |
| OTel SDK | `go.opentelemetry.io/otel` | Vendor-neutral traces + metrics |
| OTel Gin | `otelgin` | Auto-instrument all Gin routes |
| OTel GORM | `uptrace/opentelemetry-go-extra/otelgorm` | Auto-instrument all GORM queries (product domain only) |
| OTel OTLP exporters | `otlptracehttp` + `otlpmetricgrpc` | Export to any OTLP-compatible backend |
| Swagger | `swaggo/swag` | Generated from handler annotations |

### Decision: Kafka client

Use `segmentio/kafka-go`. The publisher owns bounded transient-error retries;
writer-level retries are disabled. Synchronous acknowledged publishing is
required for error reporting. Preserve event identity across retries.

---

## Architecture Layout

```text
cmd/web/                              # Wiring and lifecycle
cmd/scheduler/                        # Scheduled-job placeholder
api/http/v1/{route,middleware,response}/
config/
internal/product/
  domain/{entity,event,repository,request,response}/
  usecase/
  repository/
  cache/
  gateway/messaging/
  delivery/http/
pkg/{databases,redis,kafka,retry,telemetry,tracing,viper,logger,crypto,http,queryutil,utils}/
migrations/postgres/                  # Reviewed schema changes
docs/                                # Generated Swagger
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
- Inject a logger for instrumentation. Business rules belong in domain methods
  and must run even for callers outside HTTP. Use-case instrumentation may use
  pkg/logger and Zap; provider clients still stay in adapters.

### `repository/`
- **Product domain**: only layer allowed to import `gorm`. Creates OTel spans
  per method; `otelgorm` auto-creates child spans for each SQL query. Maps
  `gorm.ErrRecordNotFound` → domain sentinel errors.

### `cache/`
- Only layer allowed to import `go-redis`
- Creates OTel spans per method with `cache.key` and `cache.hit` attributes
- Returns `nil, nil` on cache miss so callers fall through to the DB

### `gateway/messaging/`
- Only layer allowed to import the Kafka client
- Creates OTel spans and injects W3C trace context into Kafka message
  headers via `propagation.TextMapCarrier`
- **Product domain**: publish failures are non-fatal (best-effort) — do not
  roll back the DB write, since the write already succeeded before publish.

### `delivery/http/`
- Gin handlers only — no router setup, no middleware
- Creates OTel spans per handler method

### `api/http/v1/`
Unchanged from the original boilerplate — see middleware stack order below.

Middleware stack order:
```
RequestTimer → CorrelationID → otelgin → TraceContext → Logger → ErrorHandler
```

### `pkg/`
- Domain-agnostic, process-wide infrastructure — no imports from `internal/`
- Use GORM-backed persistence infrastructure for CRUD domains.
- `queryutil/` owns generic GORM list-query building and Gin list-query
  parsing — it may import GORM/Gin but no domain package.

### `cmd/`
- Thin wiring only: load config → init OTel → connect infra → build use
  cases → start server / consumer loop → graceful shutdown

---

## Persistence and cache consistency

Updates use explicit columns, ID plus version, and affected-row checks.
Never use GORM Save for an existing-record update: its insert fallback may
recreate a deleted product. Zero-valued fields must remain writable.
`FindByIDForUpdate` selects the primary; it does not lock the row.
Optimistic version checks protect the subsequent write. Transactions spanning
multiple writes belong behind a business-specific repository contract.

Single-product reads use the primary. Lists may use replicas and are eventually
consistent. Cache Get returns a generation token; Set requires that token and
rejects fills after invalidation or token expiry. Delete atomically rotates
the token and removes the data. Tokens expire after 30s; data after 5m.
Failed invalidation can leave stale data until TTL; do not claim strong consistency.

---

## Dependency Rules (Hard Constraints)

```
domain                          → nothing internal
usecase                         → domain
repository (product)            → domain, gorm
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

### Logging

Use the injected Zap logger with LOG_LEVEL (debug/info/warn/error), LOG_FORMAT
(json/console) and explicit LOG_SAMPLING (default false). Gin mode does not
control Zap severity; console format does not enable development panics.
Successful requests log Info, 4xx Warn, 5xx Error. Successful health probes
log Debug. Sampling, when enabled, may drop repeated warnings and errors too.
Use metrics for failure counts.

Use pkg/logger.FromContext to preserve trace/span and correlation IDs in
adapter logs. Log one terminal warning for cache/publish failures after retries.
Do not log raw request headers/bodies, credentials or complete configuration.
Explicitly authorized typed request/response snapshots use pkg/logger.DebugPayload
at Debug only. It masks known sensitive keys and product descriptions, omits
payloads over 16KiB, caps nesting at eight levels and arrays at 20 items.
Key-based redaction cannot detect secrets embedded in arbitrary string values;
review new DTO fields before logging them. Never bypass the sanitizer.
Panic recovery uses structured errors and a stack; disable Gin's additional
plain-text recovery dump. Keep the request summary as a separate status record.
Include service identity and return startup/server errors for structured logging
after deferred cleanup. Do not use Fatal/Panic/DPanic for routine errors.

Every product use case logs its operation and terminal outcome: successful
completion at Info, validation/not-found/conflict/cancellation rejections at
Warn, unexpected errors at Error. Committed writes with optional cache/publisher
failures log a degraded completion at Warn. Keep payloads off these summary logs.
Validation logs include stable reason, field and rule identifiers.
Malformed JSON is a transport rejection, logged before calling a use case.
Only partially decoded typed input is available for malformed requests; never
dump raw malformed bytes. Domain validation returns ErrInvalidProduct (HTTP 400).

Generic operation logging belongs in pkg/logger/operation.go. Configure component
and operation names; pkg/apperror classifies typed, module-owned expected errors.
Generic required-string validation (including IDs) and field/rule metadata belong
in pkg/validation. These two framework-free shared primitives may be imported by
domain packages; product-specific constraints and error codes stay in the domain.
Never import domains into pkg. Defer Operation.Finish through a closure, call Succeed after a successful
response is built, and use Degrade for optional dependency failures.

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

### Failure budgets and HTTP safety

HTTP execution defaults to 10s and must be shorter than write timeout. Cache
operations use 200ms, repository operations 2s and publishing 2s, constrained
by the parent context. Deadlines are cooperative; adapters must honor them.

Adapters own retries. Disable duplicate client retries; retry transient failures
only, with jitter, bounded attempts and capped delays. Three attempts have
two default waits bounded by 50ms and 100ms. Do not blindly retry non-idempotent
operations.

Return safe public 500 messages; log internal details and correlation IDs.
Map concurrent updates to 409 and deadlines to 504. Install panic recovery.
`/health` is liveness; `/ready` checks primary access and product schema.
Redis/Kafka are optional for readiness under the chosen best-effort policy.

### Trace propagation

Same as original: `otelgin` extracts W3C `traceparent`/`tracestate` on
inbound HTTP; `TraceContext` middleware stores `trace_id`/`span_id` in Gin
context. Outgoing Kafka messages  inject trace context via
`otel.GetTextMapPropagator().Inject(ctx, &kafkaHeaderCarrier)`.

### Span instrumentation points

| Layer | Tracer name | Span name pattern |
|---|---|---|
| Product handler | `product/handler` | `product.handler.<Method>` |
| Product usecase | `product/usecase` | `product.usecase.<Action>` |
| Product repository | `product/repository` | `product.repository.<Method>` |
| Product cache | `product-service/cache` | `cache.product.<op>` |
| Product Kafka publisher | `product-service/kafka` | `kafka.publish.<EventName>` |

All spans call `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)` on failure.

### OTel provider setup, OTel Collector

Unchanged from the original boilerplate.

---

## Database (`pkg/databases`)

### `NewDB(cfg Config) (*gorm.DB, error)` — product domain and any new CRUD domain

Unchanged: selects dialector via `DB_DRIVER` (`postgres` / `mysql` /
`sqlite` / `sqlserver`), configures pool settings, wires `dbresolver` for
read/write split.

### Schema changes

`DB_AUTO_MIGRATE` is opt-in for development. Production applies reviewed,
versioned migrations before starting the binary. PostgreSQL migrations live
in `migrations/postgres/`; other engines require equivalent migrations.
Use a migration runner to track versions and coordinate deployers.

---

## Event Architecture

### Event contract

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

### Rules

- Publish after a successful DB write; product publish failures are non-fatal.
- Publishing is in-memory and may lose events after crashes or exhausted attempts.
  Adopt a transactional outbox only for an explicit durable-delivery requirement.
- Events carry event ID, schema version and aggregate version. Preserve identity
  across retries. Use product ID as the Kafka key with a key-aware partitioner.
- Consumers deduplicate event IDs and ignore stale aggregate versions;
  concurrent publishing and retries may arrive out of order.
- Inject W3C trace headers.
- Startup rejects async Kafka mode and unacknowledged writes for this publisher.

---

## Coding Conventions

### General
- No `panic()` in application code — always return errors
- Wrap errors: `fmt.Errorf("context: %w", err)`
- No global state — inject all dependencies via constructors
- `context.Context` is the first parameter on all I/O functions

### Naming
- Files: `snake_case.go`
- Interfaces: noun, no `I` prefix (`ProductRepository`, `ProductCache`)
- Use case structs: `<Action><Entity>UseCase` (`CreateProductUseCase`)
- Handler structs: `<Entity>Handler` (`ProductHandler`)
- Constructors: `New<Type>`

### Dependency injection
- Wire manually in `cmd/*/main.go` — no DI framework (no Wire, no Fx)
- Order: telemetry → DB → Redis → Kafka →
  infrastructure → use cases → handler → router → server
- Pass interfaces, not concrete types, to constructors

### Database
- Product domain: AutoMigrate on startup only when DB_AUTO_MIGRATE=true (development)
- `*gorm.DB` never leaves `internal/product/repository/`

### List endpoints (search / sort / filter / preload)
- Convention established by `internal/product`'s `GET /api/v1/products` —
  follow this shape for any new CRUD domain's list endpoint.
- Query params: `search` (substring match), repeated
  `filter=field:operator:value` (e.g. `filter=price:gte:10`),
  `sort=field:direction` (e.g. `sort=price:desc`), `preload`
  (comma-separated relation names).
- **Shared query-building lives in `pkg/queryutil`** — don't hand-roll
  `Where()`/`Order()`/`Preload()`/`Offset()`/`Limit()` chains per domain.
  Define one tagged request struct, parse it in handlers with
  `queryutil.Bind`, call `queryutil.BuildListParams` in usecase to reject
  malformed query syntax, then pass same request to repository.
  Repository calls `BuildListParams`, `ApplySearchAndFilters`, and
  `ApplySortPreloadPage`. See product's `ListProductsRequest` for reference.
- Tags form fixed allow-lists for SQL identifiers and value types:
  `search:"name,description"`, `sort:"name,price"`,
  `preload:"Category"`, and
  `filter:"price:float64:eq|gte|lte,stock:int:eq|gte|lte"`. Never infer
  columns/relations from entities or raw URL values. GORM does not
  parameterize `ORDER BY`/`Preload` identifiers.
- Filter values use GORM `?` placeholders. Only columns and SQL operators
  from compile-time tags reach SQL. Invalid filter field/operator/value,
  sort field/direction, or preload returns `400` via
  `queryutil.ErrInvalidQuery`; never silently ignore malformed syntax.
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
- PostgreSQL repository tests use DB_DSN; Redis tests use REDIS_ADDR. Both skip when absent. SQLite regression tests always run.
- Table-driven tests preferred
- Test files: `<file>_test.go` in the same package

---

## Docker

Multi-stage Dockerfile, distroless runtime — unchanged from original.
`CGO_ENABLED=0` is supported by the bundled database drivers.

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
| `HTTP_REQUEST_TIMEOUT` | `10s` | Cooperative request execution budget |
| `DB_AUTO_MIGRATE` | `false` | Development-only schema creation/update |

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



### Redis
| Variable | Default |
|---|---|
| `REDIS_ADDR` | `localhost:6379` |
| `REDIS_PASSWORD` | — |
| `REDIS_DB` | `0` |
| `REDIS_POOL_SIZE` | `10` |
| `REDIS_MIN_IDLE_CONNS` | `5` |
| `REDIS_MAX_RETRIES` | `-1` (adapter owns retries) |
| `REDIS_DIAL_TIMEOUT` | `5s` |
| `REDIS_READ_TIMEOUT` | `3s` |
| `REDIS_WRITE_TIMEOUT` | `3s` |

### Kafka
| Variable | Default | Options |
|---|---|---|
| `KAFKA_BROKERS` | `localhost:9092` | comma-separated |
| `KAFKA_BATCH_SIZE` | `100` | producer batch size |
| `KAFKA_REQUIRED_ACKS` | `all` | `all` · `leader` · `none` |
| `KAFKA_COMPRESSION` | `none` | `none` · `gzip` · `snappy` · `lz4` · `zstd` |

### OpenTelemetry
| Variable | Default | Notes |
|---|---|---|
| `OTEL_SERVICE_NAME` | `product-service` | Read by app and OTel SDK — set per binary (`web`, `scheduler`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | Read by OTel SDK directly |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | `localhost:4317` | gRPC — no `http://` prefix |

---

## Commands

```bash
# Infrastructure
make infra-up / infra-down

# Development
make run              # go run ./cmd/web
make build            # swagger regen + go build ./...
make test             # go test ./...
make test-race        # go test -race ./...
make lint             # golangci-lint run ./...
make tidy             # go mod tidy

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
- **GORM clients stay in repository adapters and composition roots**; never pass clients to use cases, domain or handlers.
- **`repository/` and `gateway/` never imported by `domain/` or
  `usecase/`** — only interfaces cross that boundary.
- **`pkg/` has no `internal/` imports** — must remain domain-agnostic.
- **No `init()` functions** — explicit initialisation only.
- **OTel spans belong in `delivery/`, `usecase/`, `repository/`, `cache/`,
  and `gateway/`** — not in `domain/`; always propagate `context.Context`.
- **Always call the OTel `shutdown` function on graceful shutdown.**
- **New Kafka topics must be added to the Topics table above.**
- **New CRUD domains follow internal/product**, with domain-owned contracts.

---

## Extension guidelines

For every new adapter, document timeout, retry ownership, idempotency,
consistency, failure behavior, lifecycle and shutdown requirements.
Add providers only behind contracts that reflect a real use case.
Keep clients out of use cases. Domain-specific transactions stay behind
repository contracts.

CI runs vet, a build, race detection with PostgreSQL/Redis, and Swagger drift
checks. Unit tests cover update conflicts/deletion, retry cancellation and
classification, safe HTTP errors, and stable event identity/partition keys.
Redis tests cover stale fills after invalidation. Integration tests require
disposable infrastructure; never point tests at production.

Close HTTP, Kafka, Redis, primary/replica pools and telemetry at shutdown.
Return startup/server errors so deferred cleanup executes. Flush both telemetry
providers even if one fails. Apply authentication and authorization policies
before exposing mutation routes.

Postman CLI is installed and available for API engineering work.
