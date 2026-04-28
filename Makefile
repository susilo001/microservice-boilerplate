.PHONY: build test test-race run lint tidy \
        swagger swagger-install \
        infra-up infra-down \
        docker-build docker-run docker-push

IMAGE_NAME ?= microservice-boilerplate
IMAGE_TAG  ?= latest

# ── Development ───────────────────────────────────────────────────────────────

swagger-install:
	go install github.com/swaggo/swag/cmd/swag@latest

swagger:
	swag init \
		--generalInfo cmd/web/main.go \
		--output ./docs \
		--parseDependency \
		--parseInternal

build: swagger
	go build ./...

test:
	go test ./...

test-race:
	go test -race ./...

run:
	go run ./cmd/web

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

# ── Infrastructure ────────────────────────────────────────────────────────────

infra-up:
	docker-compose up -d

infra-down:
	docker-compose down

# ── Docker ────────────────────────────────────────────────────────────────────

docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run:
	docker run --rm --env-file .env -p 8003:8003 $(IMAGE_NAME):$(IMAGE_TAG)

docker-push:
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
