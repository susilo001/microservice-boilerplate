# Architecture

## Implemented baseline

The repository implements product CRUD over HTTP with a GORM repository,
Redis cache and best-effort Kafka publishing. The scheduler is a placeholder.
The BFF, gateway, gRPC, search index, other domains and worker described later
are an optional reference design, not shipped capabilities.

```mermaid
flowchart LR
    HTTP[HTTP client] --> Router[Gin and middleware]
    Router --> Handler[Product handlers]
    Handler --> Usecase[Product use cases]
    Usecase --> Repository[GORM repository]
    Usecase --> Cache[Redis adapter]
    Usecase --> Publisher[Kafka adapter]
    Repository --> Primary[(Primary DB)]
    Repository -->|list reads only| Replicas[(Optional replicas)]
    Cache --> Redis[(Redis)]
    Publisher --> Kafka[Kafka]
```

Domain contracts separate use cases from adapters. Shared infrastructure in
pkg imports no internal domains. Composition roots select dependencies and
own lifecycle. Add providers behind domain-specific contracts when needed;
avoid forcing transactions or business operations into generic CRUD.

Updates use explicit columns and optimistic version checks. A competing update
returns 409, and update cannot insert a deleted product. Single-product reads
use the primary; list reads may be eventually consistent through replicas.

Cache misses obtain a generation token before reading the primary. Invalidation
atomically rotates the token and deletes data. A fill only succeeds if its
token is still current, preventing stale fills after successful invalidation.
Tokens expire after 30s; expired tokens cannot fill. Cache data expires after 5m.
Failed invalidation can still leave stale values until TTL.

Events carry event ID, schema version and aggregate version. Product ID is
the Kafka key; consumers deduplicate IDs and reject stale versions because
concurrent writers may publish out of order. Publishing is synchronous but
best-effort after commit. A crash or exhausted attempts can lose events.
This implementation has no durable queue or outbox.

The cooperative request deadline defaults to 10s; operation budgets are 200ms
for cache, 2s for persistence and 2s for publishing. Adapters own retries:
three transient-error attempts with jittered waits bounded by 50ms and 100ms.
Client-level retries are disabled. Parent deadlines take precedence.

/health is liveness; /ready verifies primary access and product schema.
Redis/Kafka remain optional for readiness. Internal errors are logged with
correlation IDs and safe public responses. Panic recovery protects handlers.
HTTP drains before Kafka, Redis, primary/replica pools and telemetry close.

Production deployments apply reviewed migrations before startup; AutoMigrate
is opt-in for local development. CI exercises PostgreSQL/Redis, race detection,
vet, build and Swagger drift checks. Other providers need equivalent integration
verification and migrations. Authentication/authorization remain deployment
requirements.

## Optional reference design

The target architecture is a set of domain-oriented services behind a
Backend-for-Frontend (BFF). Clients talk only to the BFF over REST/GraphQL; the
BFF fans out to the domain services over gRPC, merges the results into
screen-shaped payloads, and shapes them per client. Each domain service follows
Clean Architecture internally (handlers → usecase → repository/cache/gateway) and
**owns its own data** (Postgres primary + read replicas). Writes emit domain
events to Kafka; a worker consumes them to keep a shared search index in sync and
run other side effects. OpenTelemetry spans and trace-correlated logs flow through
the gateway, the BFF, every gRPC hop, and across Kafka.

## System diagram

```mermaid
flowchart TB
    web[Web app]:::client
    mobile[Mobile app]:::client
    api[API consumer]:::client

    gw["API Gateway<br/>TLS · auth · rate-limit"]:::edge

    subgraph bff_layer[BFF layer]
        bff["BFF<br/>REST/GraphQL out · gRPC in<br/>fan-out · merge · per-client shaping"]
    end

    subgraph services[Domain services · gRPC · each Clean Architecture]
        direction LR
        product["Product service"]
        customer["Customer service"]
        datamaster["DataMaster service<br/>(Fee Settings)"]
        order["Order service"]
        portfolio["Portfolio service"]
    end

    subgraph data[Per-service data ownership]
        direction LR
        productdb[("Product DB<br/>primary + replicas")]
        customerdb[("Customer DB<br/>primary + replicas")]
        dmdb[("DataMaster DB<br/>primary + replicas")]
        orderdb[("Order DB<br/>primary + replicas")]
        portdb[("Portfolio DB<br/>primary + replicas")]
    end

    redis[(Redis cache<br/>per-service + BFF)]:::infra
    search[("Search index<br/>OpenSearch")]:::infra

    subgraph events[Event backbone]
        kafka[[Kafka]]
        worker["Worker / Indexer<br/>Kafka consumer"]
    end

    otel[/"OTel → Jaeger / Tempo"/]:::obs

    web --> gw
    mobile --> gw
    api --> gw
    gw --> bff
    bff -->|gRPC| product
    bff -->|gRPC| customer
    bff -->|gRPC| datamaster
    bff -->|gRPC| order
    bff -->|gRPC| portfolio
    bff -.->|cache merged results| redis
    bff -.->|advanced search read| search

    product --> productdb
    customer --> customerdb
    datamaster --> dmdb
    order --> orderdb
    portfolio --> portdb
    services -.->|per-service cache| redis
    services -->|publish events| kafka

    kafka --> worker
    worker -->|index sync| search

    bff -.->|spans + logs| otel
    services -.->|spans + logs| otel
    worker -.->|spans| otel

    classDef client fill:#E6F1FB,stroke:#185FA5,color:#0C447C;
    classDef edge fill:#EEEDFE,stroke:#534AB7,color:#26215C;
    classDef infra fill:#E1F5EE,stroke:#0F6E56,color:#04342C;
    classDef obs fill:#EAF3DE,stroke:#3B6D11,color:#173404;
```

## Design principles

- **BFF is opinionated, not generic.** It exposes endpoints shaped to specific
  screens and is owned by the frontend team. It is not a general-purpose API; if
  it drifts toward "handle anything," that's a smell to split it.
- **REST/GraphQL out, gRPC in.** Browser-friendly, debuggable transport on the
  outer edge; binary, strongly-typed, HTTP/2 contracts on the hot internal paths.
- **Each service owns its data.** No service reaches into another's tables; data
  is shared only via gRPC calls or events.
- **Fan out in parallel, degrade gracefully.** The BFF calls services
  concurrently under a shared deadline and decides per source whether a failure
  fails the whole response or returns partial data.
- **Async where the caller need not wait.** State changes commit synchronously and
  publish events; downstream effects (search index sync, notifications) run in the
  worker.
- **Traceable by default.** A trace context flows through gateway → BFF → every
  gRPC hop → Kafka → worker, so one `trace_id` reconstructs the full path.
- **Clean Architecture inside each service.** Dependencies point inward toward
  `domain`; inner layers define interfaces, outer layers implement them.

## Components

| Component         | Responsibility                                                              |
|-------------------|-----------------------------------------------------------------------------|
| API Gateway       | TLS termination, authentication, global rate limiting, routing              |
| BFF               | Per-client REST/GraphQL endpoints; parallel gRPC fan-out; merge/reshape; aggregated-result cache |
| Domain services   | One bounded context each (Product, Customer, DataMaster/Fee Settings, Order, Portfolio); gRPC server; own DB |
| Worker / Indexer  | Consumes Kafka events; keeps the search index in sync; other side effects |
| Search index      | Read model for advanced filter / full-text / faceted queries                |
| Redis             | Hot-path cache (per-service namespaces; BFF caches merged payloads)         |
| Kafka             | Event backbone between services and the worker                              |

### Clean Architecture layers (within each domain service)

| Layer            | Responsibility                                                        | May import           |
|------------------|-----------------------------------------------------------------------|----------------------|
| `domain`         | Entities, value objects, repository/cache/gateway **interfaces**      | nothing internal     |
| `usecase`        | Business orchestration; depends only on domain interfaces             | `domain`             |
| `repository`     | GORM persistence over DBResolver (primary/replica)                    | `domain`             |
| `cache`          | Redis read-through / write-invalidate                                 | `domain`             |
| `gateway`        | Kafka publisher; trace-context injection into message headers         | `domain`             |
| `delivery/grpc`  | gRPC handlers, request binding, response shaping                      | `usecase`, `domain`  |
| `cmd/*`          | Composition root (wiring only)                                        | all layers           |
| `pkg/*`          | Shared utilities                                                      | nothing internal     |

## Request flows

### Aggregated read — BFF fan-out (e.g. `GET /api/v1/order-form-data`)

A screen that needs data from several services makes **one** call; the BFF fans
out concurrently and returns exactly the shape the UI renders.

```
Client → API Gateway → BFF (span: bff.handler.OrderFormData)
  → errgroup fan-out under a shared context deadline:
        ├─ gRPC Product service    → ProductDB    (read replica)
        ├─ gRPC Customer service   → CustomerDB   (read replica)
        ├─ gRPC DataMaster service → DataMasterDB (read replica)   // fee settings
        └─ gRPC Portfolio service  → PortfolioDB  (read replica)
  → merge into { products[], customer, feeSettings, eligibility } + cache (Redis, short TTL)
  → APIResponse[OrderFormResponse]
```

Degradation policy is per source: e.g. if the DataMaster service is down, return
products with `feeSettings: null` rather than failing the whole screen.

### Advanced search — `GET /api/v1/products` (filter / full-text)

Complex multi-field, full-text, or faceted queries are served by the search
index, **not** by Postgres.

```
Client → API Gateway → BFF (span: bff.handler.SearchProducts)
  → SearchGateway.Query (search index)
        → keyset / search_after pagination (never deep from/offset)
        → optional short-TTL cache keyed by normalized filter set
  → APIResponse[[]ProductResponse] + page cursor
```

Guardrails at the BFF: allow-listed sort fields (never raw client column names),
a cap on filter fields, and a hard result cap.

### Write — `POST /api/v1/orders`

```
Client → API Gateway → BFF (span: bff.handler.CreateOrder)
  → gRPC Order service (span: order.usecase.CreateOrder)
        → Repository.Create → DBResolver → Primary  (INSERT)
        → otelgorm auto-span for the SQL query
  → Publisher.Publish → Kafka (order.created + W3C traceparent; bounded in-memory retry)
  → 201 APIResponse[OrderResponse]
```

### Update / Delete — cache invalidation

```
Order service UpdateOrder:
  1. Repository.FindByID → Primary (read without locking)
  2. Repository.Update    → Primary  (explicit write with version check)
  3. Cache.Delete         → Redis    (immediate invalidation)
  → Publisher.Publish → Kafka (order.updated; bounded in-memory retry)
The BFF also evicts any aggregated cache entries that contained this order.
```

### Event consumption — Worker / Indexer

```
Kafka topic: order.created / product.updated / feesettings.updated / customer.updated / …
  → Consumer extracts W3C traceparent header (continues the originating trace)
  → Idempotent handler (search index upsert, notifications, audit log, …)
  → Writes back to a service's Primary only via that service's API, never directly
```

## Trace propagation across services

```mermaid
flowchart LR
    client[Client] -->|HTTP + traceparent| gw[API Gateway]
    gw --> bff["BFF · otelgin/otelgrpc"]
    bff -->|gRPC + trace ctx| svc["Domain service<br/>handler→usecase→repo (child spans)"]
    svc -->|publish · inject traceparent| kafka[[Kafka]]
    kafka -->|extract| wk[Worker · trace continues]
```

- `otelgin`/`otelgrpc` extract an upstream `traceparent` (joining the trace) or
  start a new root span. Each layer opens a child span → full waterfall in
  Jaeger/Tempo.
- The publisher injects `traceparent` into Kafka headers; the worker extracts it
  so the trace continues across the bus.
- A TraceContext middleware stores `trace_id` + `span_id` in context; the logger
  reads them, so every log line is correlatable to a span.

## Data strategy

- **Each service owns its store.** Writes to that service's primary, reads to its
  replicas, hot reads to its Redis namespace.
- **BFF cache:** merged/aggregated payloads are cached keyed by the normalized
  request (plus tenant, e.g. `cimb`); invalidated when a contributing service
  emits a relevant event.
- **Cache invalidation:** delete-on-write for entities; the BFF evicts aggregated
  entries on related events.
- **Search index:** a shared read model maintained asynchronously by the worker
  from Kafka events. Eventually consistent (typically sub-second), so
  read-your-writes cases read the owning service directly, not the index.

## Consistency & failure handling

- **Deadlines.** A `context.Context` deadline set at the BFF propagates through
  every gRPC and DB call so a slow downstream can't exhaust connection pools.
- **Connection pools.** Safe defaults with saturation monitoring; replica pools
  configured consistently with the primary in each service.
- **Partial failure.** The BFF decides per source whether to fail or degrade;
  circuit breakers fail fast on a struggling downstream.
- **In-memory retries.** Redis and Kafka operations retry with bounded
  exponential backoff. This is not durable: a process restart or exhausted
  attempts can lose a product event; use a transactional outbox when delivery
  must be guaranteed.
- **Idempotent consumers.** The worker handles redeliveries idempotently
  (search upsert by id, deduped side effects).

## Scaling notes

- Gateway, BFF, and each domain service are stateless → independent horizontal
  scale behind HPAs on RPS/CPU.
- Separate BFFs per client type (web vs mobile) when payload needs diverge enough
  to justify it.
- Use **batch gRPC endpoints** (`GetByIDs([]id)`) rather than per-id loops to
  avoid network-level N+1 in the fan-out.

## Dependency rule (within each domain service)

```
domain          ← no internal imports
usecase         ← domain (interfaces only)
repository      ← domain (interfaces only)
cache           ← domain (interfaces only)
gateway         ← domain (interfaces only)
delivery/grpc   ← usecase + domain
cmd/*           ← all layers (wiring only)
pkg/*           ← no internal imports
```
