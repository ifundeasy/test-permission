# cedar-vs-spicedb benchmark — Go (hexagonal) task runner.
BINARY  := authz-service
PKG     := ./cmd/authz-service
BIN_DIR := bin
# load .env for every target that talks to the stack from the host
ENV     := set -a; [ -f .env ] && . ./.env || true; set +a;

.PHONY: help build run test cover fmt vet tidy docker-build up logs down reset clean \
        seed seed-cedar seed-spicedb seed-tuples seed-test verify bench

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2}'

build: ## Build a static binary into bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY) $(PKG)

run: ## Run the facade locally (loads .env if present)
	@$(ENV) go run $(PKG)

seed: ## Seed BOTH engines + ground-truth tuples (full scale, batch 1000, resumable)
	@$(ENV) go run ./cmd/authz-seed -engine all

seed-cedar: ## Seed only the Cedar side (Postgres schema "cedar")
	@$(ENV) go run ./cmd/authz-seed -engine cedar

seed-spicedb: ## Seed only SpiceDB (schema + relationships)
	@$(ENV) go run ./cmd/authz-seed -engine spicedb

seed-tuples: ## Regenerate only the ground-truth tuple files
	@$(ENV) go run ./cmd/authz-seed -engine tuples

seed-test: ## Seed a miniature dataset (fast end-to-end smoke test)
	@$(ENV) go run ./cmd/authz-seed -engine all -scale test -tuples-out bench/tuples-test

verify: ## Equivalence gate: Cedar == SpiceDB == expected on all sampled tuples
	@$(ENV) go run ./cmd/authz-bench -mode verify

bench: ## Run the benchmark (all models × engines × consistency modes)
	@$(ENV) go run ./cmd/authz-bench -mode run

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
