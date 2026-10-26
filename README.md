# microservice-boilerplate

A Go microservice baseline built on Clean Architecture with a product CRUD reference domain. Review each service's delivery guarantees, security requirements, and infrastructure settings before production use.

The product domain is the reference implementation: standard CRUD with Redis caching and Kafka event publishing on writes (GORM).

Shared event and publisher contracts live in `pkg/event`; product event payloads
remain in `internal/product/domain/event`.

## Stack

| Concern | Library |
|---|---|
| HTTP framework | [Gin](https://github.com/gin-gonic/gin) |
| ORM | [GORM](https://gorm.io) + DBResolver (read/write split) |
| Database drivers | PostgreSQL, MySQL, SQLite (pure-Go), SQL Server |
| Event streaming | [kafka-go](https://github.com/segmentio/kafka-go) |
| Caching | [go-redis v9](https://github.com/redis/go-redis) |
| Configuration | [Viper](https://github.com/spf13/viper) (env + `.env` file) |
| Logging | [Zap](https://github.com/uber-go/zap) |
| Observability | [OpenTelemetry](https://opentelemetry.io) — traces + metrics, OTLP export |
| API docs | [Swaggo](https://github.com/swaggo/swag) |

## Project layout

```
cmd/
  web/          # HTTP server — wiring, OTel bootstrap, graceful shutdown
  scheduler/    # Scheduled jobs — same wiring pattern as web
api/
  http/
    v1/         # Shared HTTP layer: router, middleware, response types
    v2/         # Placeholder for future API version
  grpc/         # Placeholder for future protobuf definitions
config/         # Viper-based typed config (DB, Redis, Kafka, OTel)
internal/
  product/      # Reference CRUD domain
    domain/     # Entities, events, repository interfaces, DTOs (no external deps)
    usecase/    # Business logic — depends only on domain interfaces
    repository/ # GORM persistence — only layer that imports gorm
    cache/      # Redis caching — only layer that imports go-redis
    gateway/
      messaging/  # Kafka producer — only layer that imports kafka-go
    delivery/
      http/     # Gin handlers
pkg/
  databases/    # Multi-driver DB factory (postgres/mysql/sqlite/sqlserver + DBResolver)
  redis/        # Redis client with pool and timeout config
  kafka/        # Kafka writer with batch, compression, and acks config
  telemetry/    # OTel TracerProvider + MeterProvider (OTLP export)
  tracing/      # trace_id / span_id extraction helper
  viper/        # Shared Viper initialiser (env + .env file)
  logger/       # Zap production logger
  crypto/       # Crypto-random ID and SHA-256 hash utilities
  http/         # Shared HTTP client
  utils/        # General-purpose utilities
docs/           # Generated Swagger UI (swag init output)
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full system diagram and flow reference.  
See [AGENTS.md](AGENTS.md) for the architecture specification and coding conventions.

## Prerequisites

- Go 1.23+
- Docker + Docker Compose

## Quick start

```bash
# 1. Copy and edit environment config
cp .env.example .env

# 2. Start infrastructure (Postgres, Kafka, Redis, OTel Collector)
make infra-up

# 3. Run the web server
make run
```

API: `http://localhost:8003` · Swagger UI: `http://localhost:8003/swagger/index.html`

## Docker

```bash
# Build image
make docker-build                          # IMAGE_NAME=microservice-boilerplate IMAGE_TAG=latest

# Run container (loads .env automatically)
make docker-run

# Override image name / tag
make docker-build IMAGE_NAME=myapp IMAGE_TAG=v1.0.0
make docker-push  IMAGE_NAME=myapp IMAGE_TAG=v1.0.0
```

The image is built in two stages: a `golang:1.23-alpine` builder that compiles a fully static binary, and a `distroless/static-debian12` runtime with no shell or package manager. `CGO_ENABLED=0` is safe because all bundled DB drivers are pure-Go.

## Environment variables

Copy `.env.example` to `.env` — Viper loads it automatically on startup.

### Application

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8003` | HTTP server port |
| `GIN_MODE` | `debug` | `debug` or `release` |
| `LOG_LEVEL` | `info` | Minimum severity: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` for aggregation or `console` for local readability |
| `LOG_SAMPLING` | `false` | Opt in to sampling repeated messages |
| `HTTP_REQUEST_TIMEOUT` | `10s` | Cooperative execution deadline; shorter than HTTP write timeout |
| `DB_AUTO_MIGRATE` | `false` | Enable only for local development; production applies migrations separately |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Maximum time to read request headers |
| `HTTP_READ_TIMEOUT` | `15s` | Maximum time to read a request body |
| `HTTP_WRITE_TIMEOUT` | `15s` | Maximum time to write a response |
| `HTTP_IDLE_TIMEOUT` | `60s` | Maximum idle keep-alive duration |

### Logging

Both entry points use the same validated Zap configuration and write to stderr.
For local debugging, set `LOG_LEVEL=debug` and `LOG_FORMAT=console`.
`GIN_MODE` controls Gin's framework diagnostics independently; use
`GIN_MODE=release` for production. Console output does not enable Zap's
development panic behavior.

Request logs use Info for successful responses, Warn for 4xx and Error for 5xx.
Successful health/readiness probes use Debug. Adapter failures emit one warning
after retries, preserving trace/span IDs and correlation ID; recovered panics
emit a structured error with a stack, without dumping request headers or bodies.
The subsequent request summary still records the 500 response.

Logs include the entry point's service name. Sampling is disabled by default
so qualifying access/error entries are retained. If enabled, repeated messages
at the same level are sampled using Zap's production policy (first 100 per
second, then every 100th). Sampling affects warnings and errors too; use metrics
for counts and do not treat sampled logs as an audit trail. Level filtering
always applies regardless of sampling.

Application code should use Debug for diagnostics, Info for normal lifecycle,
Warn for recoverable degradation and Error for failed operations. Return errors
through the entry point so cleanup executes before exit; avoid Fatal, Panic and
DPanic in routine application code.

All product use cases also log business outcomes independently of HTTP. Success
uses Info; validation, not-found, conflict, cancellation and deadline rejections
use Warn; unexpected failures use Error. A committed operation with a failed
optional cache/publisher step completes with a Warn-level degraded outcome.
Adapter diagnostics and HTTP summaries remain separate records.

With `LOG_LEVEL=debug`, each use case emits typed request and response snapshots:

```json
{"level":"debug","msg":"operation request","component":"product.usecase","operation":"CreateProduct","request":{"name":"Widget","description":"[REDACTED]","price":0,"stock":1}}
{"level":"warn","msg":"operation rejected","component":"product.usecase","operation":"CreateProduct","outcome":"rejected","reason":"invalid_product","validation_field":"price","validation_rule":"finite_positive"}
{"level":"debug","msg":"operation response","component":"product.usecase","operation":"CreateProduct","response":{"error":"invalid_product"}}
```

Examples omit timestamps, service identity and trace/correlation fields for
brevity. These snapshots represent use-case input/output, not the complete HTTP
response envelope. Malformed JSON gets transport-level rejection logs with
partial typed input; raw malformed bytes and request headers are never dumped.

The shared sanitizer masks known sensitive keys (passwords, tokens, credentials,
cookies and selected personal/payment fields) and product descriptions.
Payloads over 16KiB are omitted with a reason; nesting is capped at eight levels
and arrays at 20 items. Serialization happens only when Debug logging is enabled.
Key-based masking cannot detect secrets inside arbitrary strings; review new
DTO fields and extend the sanitizer before logging sensitive domains. Keep
production at Info unless Debug payload visibility is explicitly needed.

Product name, finite positive price and non-negative stock are validated in
the domain from create/update use cases. HTTP decodes JSON before invoking those
use cases, so workers and other transports enforce the same business rules.

Reusable operation logging lives in `pkg/logger/operation.go`. Any module
can call `StartOperation` with its component and operation names. Shared error
classification lives in `pkg/apperror`; modules declare expected errors with stable
codes and messages, without a use-case classifier. Required-string validation and
structured field/rule errors live in `pkg/validation`, including required IDs.
Product-specific constraints (price and stock) and error codes remain in the domain.
The shared logger imports no product or other internal packages.

Defer `Finish` through a closure so it observes the final response and error.
Call `Succeed` after constructing a successful response; `Degrade` records optional
dependency failures. If execution unwinds before success without a returned error,
the helper logs an interrupted operation rather than a false success.

### Database

| Variable | Default | Description |
|---|---|---|
| `DB_DRIVER` | `postgres` | `postgres` · `mysql` · `sqlite` · `sqlserver` |
| `DB_DSN` | *(see below)* | Connection string — format depends on driver |
| `DB_REPLICA_DSNS` | — | Comma-separated replica DSNs; enables DBResolver read/write split |
| `DB_MAX_OPEN_CONNS` | `25` | Max simultaneous connections (primary + each replica) |
| `DB_MAX_IDLE_CONNS` | `10` | Idle connections kept in pool |
| `DB_CONN_MAX_LIFETIME` | `5m` | Max connection age before recycling |
| `DB_CONN_MAX_IDLE_TIME` | `5m` | Max idle time before closing |

DSN format per driver:

| Driver | Example DSN |
|---|---|
| `postgres` | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `mysql` | `user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=True&loc=Local` |
| `sqlite` | `app.db` or `:memory:` |
| `sqlserver` | `sqlserver://user:pass@host:1433?database=db` |

### Redis

| Variable | Default | Description |
|---|---|---|
| `REDIS_ADDR` | `localhost:6379` | Host and port |
| `REDIS_PASSWORD` | — | Auth password (leave unset if disabled) |
| `REDIS_DB` | `0` | Database index |
| `REDIS_POOL_SIZE` | `10` | Max socket connections |
| `REDIS_MIN_IDLE_CONNS` | `5` | Minimum idle connections |
| `REDIS_MAX_RETRIES` | `-1` | Client retries disabled; cache adapter owns retry policy |
| `REDIS_DIAL_TIMEOUT` | `5s` | Connection timeout |
| `REDIS_READ_TIMEOUT` | `3s` | Read timeout |
| `REDIS_WRITE_TIMEOUT` | `3s` | Write timeout |

### Kafka

| Variable | Default | Description |
|---|---|---|
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated broker list |
| `KAFKA_BATCH_SIZE` | `100` | Messages per batch |
| `KAFKA_BATCH_TIMEOUT` | `1s` | Max wait before flushing a partial batch |
| `KAFKA_REQUIRED_ACKS` | `all` | `all` or `leader`; no-ack mode is rejected |
| `KAFKA_ASYNC` | `false` | Must remain false for observable delivery failures |
| `KAFKA_COMPRESSION` | `none` | `none` · `gzip` · `snappy` · `lz4` · `zstd` |

Redis cache operations and Kafka product-event publishing use up to three
in-memory attempts for transient failures. Full jitter randomizes the two
backoff waits within 0–50ms and 0–100ms. Client-level retries are disabled.
Cache operations have a 200ms budget, repository operations 2s, and publishing
2s, all constrained by the incoming request's 10s deadline. These are execution
budgets, distinct from socket read/write timeouts.

Publishing is synchronous and best-effort after the database commit.
`KAFKA_ASYNC=true` and `KAFKA_REQUIRED_ACKS=none` are rejected at startup.
Events have stable IDs, schema versions, aggregate versions, and a product-ID
partition key. Consumers must deduplicate IDs and reject stale aggregate versions:
concurrent publishers and retries can still arrive out of order.
Process restarts or exhausted attempts can lose events. Adopt a transactional
outbox only for domains requiring durable delivery.

### Consistency and schema changes

Updates use an optimistic version check; a competing update returns `409`,
and an update never recreates a deleted product. Single-product database reads
use the primary. List queries may use replicas and remain eventually consistent.

Cache fills carry a generation token obtained before the database read.
Invalidation atomically rotates that token and removes cached data; a stale
fill is rejected. Generation tokens expire after 30s, and fills with expired
tokens are rejected. Cached products expire after 5m. Redis failures still permit
database access; a failed invalidation can leave stale data until the cache TTL.
The product cache adapter owns the five-minute data TTL; use cases do not choose
expiration or pass a TTL when filling the cache.
This is an availability-oriented cache, not a strongly consistent read model.

Concurrent cache misses for the same product share one database load per process.
Each caller can cancel its own wait; cancellation does not abort another caller's
load. Shared loads have an independent two-second timeout and recheck the cache
before reading the database. This reduces hot-key stampedes within one instance,
not across the whole deployment.

### Request and query limits

JSON requests are limited to 1 MiB, including trailing data, and oversized bodies
return `413`. List queries allow at most 100 items per page and an offset of
100,000; malformed, negative, or excessive pagination returns `400`. The same
limits are enforced when a list use case is called outside HTTP. Zero-valued
pagination in a direct Go request selects the defaults (page 1, size 20).

Product lists default to `id asc`. Explicit sorts append `id asc` as a unique
tie-breaker. New modules configure their unique pagination column through the
sql-generator repository schema. Offset pagination is not snapshot isolation: concurrent
writes can still shift pages. Use cursor pagination for large or fast-changing
datasets. Product has no relations, so any requested preload is rejected.

### SQL generator dependency

List handlers parse `filter=field:value:operator` with `|` between clauses.
For example:

```text
GET /api/v1/products?page=1&page_size=20&filter=price:10:greaterthanorequal%7Cstock:0:equals&sort=price:desc
```

Name supports `equals` and `contains`; price and stock support `equals`,
`greaterthanorequal` and `lessthanorequal`. Sortable fields are name, price,
stock, created_at and updated_at. Filters are combined with AND. Projection is
not exposed and Product has no allowed preloads. Filter/sort counts are capped
at 20/5. This replaces the previous `field:operator:value` filter syntax.

Handlers produce product-owned structured requests. Use cases validate pagination,
including for direct Go callers. Repositories validate filter, sort and preload
allow-lists, own explicit schemas, numeric value validation and GORM scope
generation through `github.com/susilo001/sql-generator`. Filtered totals use
`CountScopes` separately from paginated `Scopes`.

The service pins `github.com/susilo001/sql-generator` v1.5.0, so standalone
CI and Docker builds do not need a sibling library checkout. Go 1.23 remains
the baseline in both modules.

### Extending business workflows

Use `entity.NewProduct` and `Product.ChangeDetails` to validate construction and
mutation before changing state. Persistence fields remain exported for GORM;
new application code must not bypass domain methods for business mutations.
Repository `FindByID` reads the primary but does not lock rows; updates retain
their optimistic version check.

Multi-record workflows require a concrete transactional repository operation
or a transaction runner that binds all participating repositories to one database
transaction. Do not expose GORM transactions to use cases. This CRUD reference
does not invent inventory reservation, payment, or saga workflows.

Create commands are not idempotent across HTTP retries. Before using this pattern
for commands that must not execute twice, define an idempotency key scope and
retention policy, then persist the key, request fingerprint, and result atomically
with the business write. An in-memory map does not provide this guarantee across
restarts or replicas. Best-effort Kafka publishing remains unchanged; guaranteed
downstream processing requires durable delivery and idempotent consumers.

`DB_AUTO_MIGRATE` defaults to `false`. The example environment enables it for
local development. Production deployments should apply reviewed, versioned
migrations before starting the new binary. PostgreSQL migrations are provided:

```bash
psql "$DB_DSN" -v ON_ERROR_STOP=1 -1 -f migrations/postgres/001_products.sql -f migrations/postgres/002_product_version.sql
```

Run migrations once as a controlled deployment step; use your migration runner
to track applied versions and coordinate multiple deployers. Other supported
database engines require equivalent reviewed migrations. Do not enable
AutoMigrate as a substitute for production schema-change planning.

`/health` is liveness; `/ready` verifies primary database access and the product
table. Redis and Kafka failures do not remove readiness because they are optional
for the chosen best-effort behavior. Protect mutation routes with the deployment's
authentication and authorization policy before exposing the service.

CI runs tests with PostgreSQL and Redis, the race detector, vet, a build, and
Swagger drift checks. Local integration tests skip when their infrastructure
environment variables are absent.

### OpenTelemetry

| Variable | Default | Description |
|---|---|---|
| `OTEL_SERVICE_NAME` | `product-service` | Service name on all signals |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTLP/HTTP endpoint for traces (read by SDK directly) |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | `localhost:4317` | OTLP/gRPC endpoint for metrics — no `http://` scheme (read by SDK directly) |

## API endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness check |
| `GET` | `/ready` | Primary database and product-schema readiness |
| `GET` | `/swagger/*any` | Swagger UI |
| `GET` | `/api/v1/products` | List products (paginated `?page=1&page_size=20`, searchable `?search=widget`, sortable `?sort=price:desc`, filterable `?filter=price:10:greaterthanorequal%7Cprice:100:lessthanorequal`) |
| `GET` | `/api/v1/products/:id` | Get product by ID |
| `POST` | `/api/v1/products` | Create product |
| `PUT` | `/api/v1/products/:id` | Update product |
| `DELETE` | `/api/v1/products/:id` | Delete product |

## Makefile targets

```bash
make infra-up        # Start Docker infrastructure
make infra-down      # Stop Docker infrastructure
make run             # Run the web server (go run)
make build           # Regenerate Swagger docs then go build ./...
make swagger         # Regenerate Swagger docs only
make swagger-install # Install swag CLI
make test            # Run unit tests
make test-race       # Run unit tests with race detector
make lint            # Run golangci-lint
make tidy            # go mod tidy
make docker-build    # Build Docker image
make docker-run      # Run container with .env
make docker-push     # Push image to registry
```

## Observability

Every request produces a full trace waterfall across all layers:

```
otelgin: GET /api/v1/products/:id       ← HTTP root span (auto)
  product.handler.GetByID               ← handler span
    product.usecase.GetProduct          ← use case span
      cache.product.get                 ← Redis span (cache.hit attribute)
      product.repository.FindByID       ← repository span
        gorm: SELECT * FROM products    ← DB query span (otelgorm, auto)
      cache.product.set
```

Signals exported via OTLP:
- **Traces** — HTTP (otelgin), DB queries (otelgorm), Kafka publishes, Redis ops, handler/usecase/repository spans
- **Metrics** — HTTP request duration/count (otelgin), DB query duration (otelgorm)
- **Logs** — Zap structured logs with `trace_id` and `span_id` fields injected by the `TraceContext` middleware

The local OTel Collector (`otel-collector-config.yaml`) prints signals to stdout. Swap the `debug` exporter for Jaeger, Grafana Tempo, Prometheus, etc. in production.

## Adding a new domain

1. Create `internal/<name>/` following the same structure as `internal/product/`
2. Define entities, events, and repository interfaces in `domain/` (no external imports)
3. Implement use cases in `usecase/` (depend only on domain interfaces)
4. Add GORM repository in `repository/`, Redis cache in `cache/`, Kafka publisher in `gateway/messaging/`
5. Add Gin handlers in `delivery/http/` with Swagger annotations
6. Wire everything in the relevant `cmd/` entry point

Follow the dependency rules in [AGENTS.md](AGENTS.md): domain has zero external deps, use cases depend only on domain interfaces, infrastructure layers never cross domain boundaries.
