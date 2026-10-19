# microservice-boilerplate

A production-ready Go microservice boilerplate built on Clean Architecture, Domain-Driven Design, and Event-Driven Architecture. Clone this repo as the starting point for any new Go service.

The product domain is the reference implementation: standard CRUD with Redis caching and Kafka event publishing on writes (GORM).

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
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Maximum time to read request headers |
| `HTTP_READ_TIMEOUT` | `15s` | Maximum time to read a request body |
| `HTTP_WRITE_TIMEOUT` | `15s` | Maximum time to write a response |
| `HTTP_IDLE_TIMEOUT` | `60s` | Maximum idle keep-alive duration |

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
| `REDIS_MAX_RETRIES` | `3` | Retries on transient errors |
| `REDIS_DIAL_TIMEOUT` | `5s` | Connection timeout |
| `REDIS_READ_TIMEOUT` | `3s` | Read timeout |
| `REDIS_WRITE_TIMEOUT` | `3s` | Write timeout |

### Kafka

| Variable | Default | Description |
|---|---|---|
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated broker list |
| `KAFKA_BATCH_SIZE` | `100` | Messages per batch |
| `KAFKA_BATCH_TIMEOUT` | `1s` | Max wait before flushing a partial batch |
| `KAFKA_REQUIRED_ACKS` | `all` | `all` · `leader` · `none` |
| `KAFKA_ASYNC` | `false` | Fire-and-forget (no delivery guarantee) |
| `KAFKA_COMPRESSION` | `none` | `none` · `gzip` · `snappy` · `lz4` · `zstd` |

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
| `GET` | `/swagger/*any` | Swagger UI |
| `GET` | `/api/v1/products` | List products (paginated `?page=1&page_size=20`, searchable `?search=widget`, sortable `?sort=price:desc`, filterable `?filter=price:gte:10&filter=price:lte:100`, preloadable `?preload=relation`) |
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
