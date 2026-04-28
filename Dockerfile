# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

# swag is needed to regenerate Swagger docs from source annotations.
RUN go install github.com/swaggo/swag/cmd/swag@latest

WORKDIR /app

# Download dependencies separately so this layer is cached when only source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Regenerate Swagger docs from the canonical entry point.
RUN swag init \
    --generalInfo cmd/web/main.go \
    --output ./docs \
    --parseDependency \
    --parseInternal

# Build a fully static binary.
# CGO_ENABLED=0 is safe because all DB drivers used here are pure-Go.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /bin/web ./cmd/web

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# distroless/static has no shell, no package manager, and no libc — minimal attack surface.
# It includes CA certificates so OTLP/TLS connections work out of the box.
FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/web /web

# Document the default port; override APP_PORT env var if needed.
EXPOSE 8003

# Run as non-root (distroless ships uid 65532 as "nonroot").
USER nonroot:nonroot

ENTRYPOINT ["/web"]
