# cedar-authz-service — Go (hexagonal) task runner.
BINARY  := authz-service
PKG     := ./cmd/authz-service
BIN_DIR := bin

.PHONY: help build run test cover fmt vet tidy docker-build up logs down reset clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2}'

build: ## Build a static binary into bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY) $(PKG)

run: ## Run locally (loads .env if present)
	@set -a; [ -f .env ] && . ./.env || true; set +a; go run $(PKG)

test: ## Run all tests
	go test ./...

cover: ## Run tests with a coverage summary
	go test -cover ./...

fmt: ## Format all Go code
	gofmt -w .

vet: ## Run go vet static analysis
	go vet ./...

tidy: ## Sync go.mod / go.sum
	go mod tidy

docker-build: ## Build the container image
	docker build -t $(BINARY) .

up: ## Start the full stack (postgres + service)
	docker compose up --build

logs: ## Tail the service logs
	docker compose logs -f authz-service

down: ## Stop the stack (keeps the data volume)
	docker compose down

reset: ## Stop the stack AND delete the data volume (DESTRUCTIVE — data loss)
	docker compose down -v

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
