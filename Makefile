.PHONY: help proto build test lint run-all docker-build deploy-local deploy-obs seed clean

# -- Config --
GO := go
BUF := buf
DOCKER := docker
KUBECTL := kubectl

SERVICES := api-gateway image-svc facts-svc worker-svc
BIN_DIR := bin

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# -- Protobuf --

proto: ## Generate Go code from .proto files
	$(BUF) generate

proto-lint: ## Lint protobuf files
	$(BUF) lint

proto-breaking: ## Check for breaking changes
	$(BUF) breaking --against '.git#branch=main'

# -- Build --

build: ## Build all service binaries
	@mkdir -p $(BIN_DIR)
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		$(GO) build -o $(BIN_DIR)/$$svc ./cmd/$$svc; \
	done

build-%: ## Build a specific service (e.g., make build-api-gateway)
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$* ./cmd/$*

# -- Test --

test: ## Run unit tests
	$(GO) test ./... -v -race -count=1

test-int: ## Run integration tests (requires docker-compose up)
	$(GO) test ./test/integration/... -v -tags=integration

test-coverage: ## Run tests with coverage
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html

# -- Lint --

lint: ## Run linter
	golangci-lint run ./...

# -- Run --

run-gateway: ## Run API gateway locally
	$(GO) run ./cmd/api-gateway

run-image: ## Run image service locally
	$(GO) run ./cmd/image-svc

run-facts: ## Run facts service locally
	$(GO) run ./cmd/facts-svc

run-worker: ## Run worker service locally
	$(GO) run ./cmd/worker-svc

run-all: ## Run all services (requires tmux or multiple terminals)
	@echo "Start infrastructure first: docker-compose up -d"
	@echo "Then run each service in a separate terminal:"
	@echo "  make run-gateway"
	@echo "  make run-image"
	@echo "  make run-facts"
	@echo "  make run-worker"

# -- Docker --

docker-build: ## Build Docker images for all services
	@for svc in $(SERVICES); do \
		echo "Building Docker image: voyager-$$svc"; \
		$(DOCKER) build -t voyager-$$svc --build-arg SERVICE=$$svc .; \
	done

docker-up: ## Start infrastructure (Postgres, NSQ, MinIO, Observability)
	docker-compose up -d

docker-down: ## Stop infrastructure
	docker-compose down

docker-logs: ## Follow infrastructure logs
	docker-compose logs -f

# -- Kubernetes --

deploy-local: ## Deploy all services to local k3s
	$(KUBECTL) apply -k deploy/overlays/local/

deploy-obs: ## Deploy observability stack (Prometheus + Grafana + Tempo)
	$(KUBECTL) apply -k deploy/observability/

undeploy: ## Remove all from k8s
	$(KUBECTL) delete -k deploy/overlays/local/ --ignore-not-found

# -- Database --

migrate-up: ## Run database migrations
	$(GO) run ./cmd/migrate up

migrate-down: ## Rollback last migration
	$(GO) run ./cmd/migrate down

seed: ## Seed database with sample data
	$(GO) run ./scripts/seed-data.go

# -- Utilities --

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out coverage.html

deps: ## Download Go dependencies
	$(GO) mod download
	$(GO) mod tidy

tools: ## Install development tools
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest
