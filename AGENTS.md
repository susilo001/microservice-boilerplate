# AGENTS.md — Go Microservice Boilerplate

> Architecture: Event-Driven (EDA) + Clean Architecture + Domain-Driven Design  
> Module: `github.com/boilerplate/microservice-boilerplate`  
> Go: 1.23  
> See [ARCHITECTURE.md](ARCHITECTURE.md) for the full system diagram.

---

## Project Overview

A production-ready Go microservice boilerplate. `internal/product/` is the reference implementation covering a full product catalog domain (Create, Read, List, Update, Delete) with Redis caching, Kafka event publishing, and end-to-end OpenTelemetry instrumentation.

Clone and extend by adding new domains under `internal/<name>/` following the same layer conventions.

---

## Tech Stack

| Concern | Package | Notes |
|---|---|---|
| HTTP framework | `gin-gonic/gin` | Fast router, excellent middleware ecosystem |
| ORM | `gorm.io/gorm` | Auto-migrate, clean query API |
| DB drivers | `gorm.io/driver/postgres`, `gorm.io/driver/mysql`, `github.com/glebarez/sqlite`, `gorm.io/driver/sqlserver` | Selected at runtime via `DB_DRIVER` |
| Read/write split | `gorm.io/plugin/dbresolver` | Random replica policy, configurable per domain |
| Kafka client | `segmentio/kafka-go` | Pure Go, no CGO |
| Redis client | `redis/go-redis/v9` | Context-aware, full pool config |
| Configuration | `spf13/viper` | Env vars + `.env` file, typed getters |
| Logging | `uber-go/zap` | Structured, zero-allocation |
| OTel SDK | `go.opentelemetry.io/otel` | Vendor-neutral traces + metrics |
| OTel Gin | `otelgin` | Auto-instrument all Gin routes |
| OTel GORM | `uptrace/opentelemetry-go-extra/otelgorm` | Auto-instrument all GORM queries |
| OTel OTLP exporters | `otlptracehttp` + `otlpmetricgrpc` | Export to any OTLP-compatible backend |
| Swagger | `swaggo/swag` | Generated from handler annotations |

---

## Architecture Layout

```
microservice-boilerplate/
├── AGENTS.md
├── README.md
├── Dockerfile                          # Multi-stage: builder (alpine) + runtime (distroless)
├── .dockerignore
├── go.mod                              # github.com/boilerplate/microservice-boilerplate
├── go.sum
├── .env.example                        # All supported env vars with defaults
├── docker-compose.yml                  # Postgres, Kafka, Redis, Zookeeper, OTel Collector
├── Makefile                            # build, test, run, docker-build, infra-up/down …
├── otel-collector-config.yaml          # OTLP receivers → batch processor → debug exporter
├── cmd/
│   ├── web/
│   │   └── main.go                     # HTTP server: config → OTel → DB → Redis → Kafka → handler → router
│   ├── worker/
│   │   └── main.go                     # Kafka consumer skeleton (same wiring pattern)
│   └── scheduler/
│       └── main.go                     # Scheduled jobs skeleton (same wiring pattern)
├── api/
│   ├── http/
│   │   ├── v1/
│   │   │   ├── route/
│   │   │   │   └── router.go           # Middleware stack + versioned route groups
│   │   │   ├── middleware/
│   │   │   │   ├── correlation.go      # X-Correlation-ID propagation
│   │   │   │   ├── tracing.go          # Stores trace_id/span_id in Gin context
│   │   │   │   ├── logger.go           # Structured request log with trace_id + span_id
│   │   │   │   └── error_handler.go    # Maps domain errors → HTTP status codes
│   │   │   └── response/
│   │   │       └── response.go         # APIResponse[T], ListData[T], NewSuccess/Error/List
│   │   └── v2/                         # Placeholder for future API version
│   └── grpc/                           # Placeholder for future protobuf definitions
├── config/
│   └── config.go                       # Viper-backed Config struct (DB, Redis, Kafka, OTel)
├── docs/                               # Generated Swagger UI (swag init output)
├── internal/
│   └── product/
│       ├── domain/
│       │   ├── entity/                 # Product struct with GORM tags
│       │   ├── event/                  # Event interface + ProductCreatedEvent, ProductUpdatedEvent
│       │   ├── repository/             # ProductRepository + ProductCache interfaces
│       │   ├── request/                # Input DTOs
│       │   └── response/               # Output DTOs + FromEntity mapper
│       ├── usecase/                    # CreateProduct, GetProduct, ListProducts, UpdateProduct, DeleteProduct
│       ├── repository/
│       │   ├── product_repository.go   # GORM implementation with per-method OTel spans
│       │   └── product_repository_test.go
│       ├── cache/
│       │   ├── product_cache.go        # Redis implementation with per-method OTel spans
│       │   └── product_cache_test.go
│       ├── gateway/
│       │   └── messaging/
│       │       └── product_publisher.go # Kafka producer with W3C trace context injection
│       └── delivery/
│           └── http/
│               └── product_handler.go  # Gin handlers with Swagger annotations + OTel spans
└── pkg/
    ├── databases/
    │   └── db.go                       # NewDB — multi-driver factory + DBResolver + otelgorm
    ├── redis/
    │   └── client.go                   # NewClient — full pool + timeout config
    ├── kafka/
    │   └── kafka.go                    # NewWriter — batch, compression, acks config
    ├── telemetry/
    │   └── provider.go                 # InitProvider — TracerProvider + MeterProvider (OTLP)
    ├── tracing/
    │   └── tracing.go                  # IDs(ctx) — extracts trace_id, span_id from context
    ├── viper/
    │   └── viper.go                    # New(envFile) — Viper with AutomaticEnv + .env support
    ├── logger/
    │   └── logger.go                   # NewLogger — Zap production logger
    ├── crypto/
    │   └── crypto.go                   # NewID, Hash
    ├── http/
    │   └── client.go                   # Shared HTTP client
    └── utils/                          # General-purpose utilities (placeholder)
```

---

## Layer Responsibilities

### `domain/`
- Plain Go structs — GORM model tags are allowed on entity structs
- Event structs + `Publisher` interface
- Repository + cache interfaces (contracts only, no implementations)
- `request/` and `response/` sub-packages for input/output DTOs
- **Zero imports from any other internal layer, zero external framework imports**

### `usecase/`
- Interface + implementation for every use case
- All use cases are interface-backed for testability
- Depends only on `domain/` interfaces — never on concrete implementations
- Propagates `context.Context` to all I/O calls
- Creates OTel spans (`otel.Tracer("product/usecase").Start(...)`) — see Observability section

### `repository/`
- Only layer allowed to import `gorm`
- Creates OTel spans per method; `otelgorm` auto-creates child spans for each SQL query
- Maps `gorm.ErrRecordNotFound` → domain sentinel errors (e.g., `ErrProductNotFound`)

### `cache/`
- Only layer allowed to import `go-redis`
- Creates OTel spans per method with `cache.key` and `cache.hit` attributes
- Returns `nil, nil` on cache miss so callers can fall through to the DB

### `gateway/messaging/`
- Only layer allowed to import `kafka-go`
- Creates OTel spans and injects W3C trace context into Kafka message headers via `propagation.TextMapCarrier`
- Publish failures are non-fatal (best-effort) — do not roll back DB writes

### `delivery/http/`
- Gin handlers only — no router setup, no middleware
- Creates OTel spans per handler method
- Reads `correlationid` and `trace_id` from Gin context (set by middleware)
- Uses `c.Error(err)` to delegate error rendering to `ErrorHandler` middleware

### `api/http/v1/`
- **`route/`** — assembles the middleware stack and registers versioned route groups; imports `delivery/http` handlers and domain entities for the error mapper closure
- **`middleware/`** — no domain imports; error mapping is supplied by `route/` via a closure
- **`response/`** — generic wrappers; no `internal/` imports

Middleware stack order:
```
RequestTimer → CorrelationID → otelgin → TraceContext → Logger → ErrorHandler
```
`TraceContext` runs after `otelgin` so the OTel span (propagated or freshly created) is already in the request context when it reads it.

### `pkg/`
- Domain-agnostic, process-wide infrastructure — no imports from `internal/`

### `cmd/`
- Thin wiring only: load config → init OTel → connect infra → build use cases → start server → graceful shutdown

---

## Dependency Rules (Hard Constraints)

```
domain                    → nothing internal
usecase                   → domain
repository                → domain
cache                     → domain
gateway                   → domain
api/http/v1/middleware    → nothing internal
api/http/v1/response      → nothing internal
api/http/v1/route         → delivery, middleware, response, internal/domain (error mapper only)
delivery/http             → usecase, domain, api/http/v1/response
cmd/*                     → all layers (wiring only)
pkg/*                     → nothing internal
```

---

## Observability (OpenTelemetry)

### Span waterfall per request

```
otelgin: HTTP <METHOD> <PATH>           ← auto (middleware)
  product.handler.<Method>             ← delivery layer
    product.usecase.<Action>           ← use case layer
      cache.product.<op>               ← Redis (cache.hit=true/false)
      product.repository.<Method>      ← repository layer
        gorm: <SQL>                    ← auto (otelgorm plugin)
      cache.product.set
    kafka.publish.<event>              ← gateway layer (on writes)
```

### Trace propagation

Incoming requests: `otelgin` calls `otel.GetTextMapPropagator().Extract()` on W3C `traceparent`/`tracestate` headers. If headers are present the request joins an existing distributed trace; if absent a fresh root span is created automatically.

`TraceContext` middleware then stores `trace_id` and `span_id` from the resolved span into the Gin context so handlers and logs can access them without touching the OTel API directly.

Outgoing Kafka messages: the publisher calls `otel.GetTextMapPropagator().Inject(ctx, &kafkaHeaderCarrier)` to propagate the trace across the message bus.

### Span instrumentation points

| Layer | Tracer name | Span name pattern |
|---|---|---|
| Handler | `product/handler` | `product.handler.<Method>` |
| Use case | `product/usecase` | `product.usecase.<Action>` |
| Repository | `product/repository` | `product.repository.<Method>` |
| Cache | `product-service/cache` | `cache.product.<op>` |
| Kafka publisher | `product-service/kafka` | `kafka.publish.<EventName>` |

All spans call `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)` on failure.

### OTel provider setup (`pkg/telemetry/provider.go`)

```go
func InitProvider(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error)
```

- **Traces** — OTLP/HTTP exporter; reads `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
- **Metrics** — OTLP/gRPC exporter; reads `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`
- Propagator: `TraceContext` + `Baggage` (W3C standard)
- Returns a shutdown function — always call it on graceful shutdown

### OTel Collector (`otel-collector-config.yaml`)

Receives OTLP on gRPC (4317) and HTTP (4318). Pipeline:
```
receivers: [otlp] → processors: [memory_limiter, batch] → exporters: [debug]
```
Replace `debug` exporter with Jaeger, Grafana Tempo, or Prometheus for production.

---

## Database (`pkg/databases`)

### `NewDB(cfg Config) (*gorm.DB, error)`

Selects the GORM dialector based on `cfg.Driver`:

| `DB_DRIVER` | Engine | DSN format |
|---|---|---|
| `postgres` | PostgreSQL | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `mysql` | MySQL / MariaDB | `user:pass@tcp(host:3306)/db?charset=utf8mb4&parseTime=True&loc=Local` |
| `sqlite` | SQLite (pure-Go, no CGO) | file path or `:memory:` |
| `sqlserver` | SQL Server | `sqlserver://user:pass@host:1433?database=db` |

Pool settings (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `ConnMaxIdleTime`) are applied to both the primary and every replica. Pool settings are skipped for SQLite (file-based).

### Read/write split with DBResolver

Set `DB_REPLICA_DSNS` (comma-separated, same driver as primary). DBResolver routes:
- `INSERT` / `UPDATE` / `DELETE` → primary
- `SELECT` / `Find` / `First` / `Count` → random replica

Pool settings are applied to each replica via `dbresolver.SetMaxOpenConns(...)` chain.

To force a specific connection per query:
```go
db.Clauses(dbresolver.Use("primary")).Find(&products)
```

---

## Event Architecture

### Event contract

```go
// internal/product/domain/event/event.go
type Event interface {
    EventName() string
    OccurredAt() time.Time
}
```

### Topics

| Topic | Trigger |
|---|---|
| `product.created` | Product created |
| `product.updated` | Product updated |

### Rules

- Publish **after** a successful DB write, never before
- Publishing failure is non-fatal — do not roll back
- Consumers must be idempotent (at-least-once delivery)
- Always inject OTel trace context into Kafka message headers

---

## Coding Conventions

### General
- No `panic()` in application code — always return errors
- Wrap errors: `fmt.Errorf("context: %w", err)`
- No global state — inject all dependencies via constructors
- `context.Context` is the first parameter on all I/O functions

### Naming
- Files: `snake_case.go`
- Interfaces: noun, no `I` prefix (`ProductRepository` not `IProductRepository`)
- Use case structs: `<Action><Entity>UseCase` (`CreateProductUseCase`)
- Handler structs: `<Entity>Handler` (`ProductHandler`)
- Constructors: `New<Type>` (`NewProductHandler`)

### Dependency injection
- Wire manually in `cmd/*/main.go` — no DI framework (no Wire, no Fx)
- Order: telemetry → DB → Redis → Kafka → infrastructure → use cases → handler → router → server
- Pass interfaces, not concrete types, to constructors

### Database
- `AutoMigrate` in `cmd/*/main.go` on startup — no separate migration tool
- GORM tags go on domain entity structs
- `*gorm.DB` never leaves `repository/`

### Error handling
- Domain sentinel errors defined in `domain/entity/` (e.g., `ErrProductNotFound`)
- HTTP status mapping in `api/http/v1/route/router.go` (closure passed to `ErrorHandler`)
- Record errors on the OTel span at the layer that catches them: `span.RecordError(err); span.SetStatus(codes.Error, err.Error())`

### Testing
- Use case tests: mock repository/cache/publisher interfaces — no real DB
- Repository integration tests: real DB via `DB_DSN` env var; skipped when absent (`t.Skip`)
- Table-driven tests preferred
- Test files: `<file>_test.go` in the same package

---

## Docker

### Image

Multi-stage Dockerfile:
1. **builder** (`golang:1.23-alpine`) — installs `swag`, downloads deps, regenerates Swagger docs, builds a static binary with `-ldflags="-w -s"`
2. **runtime** (`gcr.io/distroless/static-debian12`) — copies the binary only; no shell, no package manager

`CGO_ENABLED=0` is safe because all included DB drivers (including SQLite via `glebarez/sqlite`) are pure-Go.

### Makefile targets

```bash
make docker-build [IMAGE_NAME=...] [IMAGE_TAG=...]
make docker-run                   # docker run --env-file .env -p 8003:8003
make docker-push [IMAGE_NAME=...] [IMAGE_TAG=...]
```

---

## Environment Variables

### Application
| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8003` | HTTP server port |
| `GIN_MODE` | `debug` | `debug` or `release` |

### Database
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
| `REDIS_MAX_RETRIES` | `3` |
| `REDIS_DIAL_TIMEOUT` | `5s` |
| `REDIS_READ_TIMEOUT` | `3s` |
| `REDIS_WRITE_TIMEOUT` | `3s` |

### Kafka
| Variable | Default | Options |
|---|---|---|
| `KAFKA_BROKERS` | `localhost:9092` | comma-separated |
| `KAFKA_BATCH_SIZE` | `100` | |
| `KAFKA_BATCH_TIMEOUT` | `1s` | |
| `KAFKA_REQUIRED_ACKS` | `all` | `all` · `leader` · `none` |
| `KAFKA_ASYNC` | `false` | |
| `KAFKA_COMPRESSION` | `none` | `gzip` · `snappy` · `lz4` · `zstd` |

### OpenTelemetry
| Variable | Default | Notes |
|---|---|---|
| `OTEL_SERVICE_NAME` | `product-service` | Read by app and OTel SDK |
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
- **No raw `database/sql` or `pgx`** — use `pkg/databases.NewDB`; never bypass GORM
- **`*gorm.DB` stays in `repository/`** — never pass it to usecase, domain, or handler layers
- **`repository/` and `gateway/` never imported by `domain/` or `usecase/`** — only interfaces cross that boundary
- **`pkg/` has no `internal/` imports** — it must remain domain-agnostic
- **No `init()` functions** — explicit initialisation only
- **OTel spans belong in `delivery/`, `usecase/`, `repository/`, `cache/`, and `gateway/`** — not in `domain/`; always propagate `context.Context`
- **Always call the OTel `shutdown` function on graceful shutdown** — prevents data loss on flush
- **New Kafka topics must be added to the Topics table above**
- **New services follow the same layer structure as `internal/product/`** — domain → usecase → repository/cache/gateway → delivery
