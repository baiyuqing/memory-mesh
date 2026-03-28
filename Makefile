.PHONY: help dev-up dev-down test test-unit test-integration test-e2e lint seed logs build clean

CLUSTER_NAME := ottoplus-dev
K3D_CONFIG := deploy/k3d-config.yaml

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Development Environment ---

dev-up: ## Create k3d cluster + LocalStack + deploy all components
	@./scripts/dev-up.sh

dev-down: ## Teardown k3d cluster and all resources
	@./scripts/dev-down.sh

seed: ## Seed test data into the local environment
	@./scripts/seed.sh

logs: ## Tail logs from all components in the cluster
	@kubectl logs -n ottoplus --all-containers=true -l app.kubernetes.io/part-of=ottoplus -f --tail=100

# --- Build ---

build: ## Build all Go binaries
	go build -o bin/api-server ./src/api/...
	go build -o bin/operator ./src/operator/...

clean: ## Remove build artifacts
	rm -rf bin/

# --- Testing ---

test: test-unit ## Run all tests (default: unit only)

test-unit: ## Run unit tests (no infra required)
	go test ./src/core/... ./src/shared/... -v -count=1

test-integration: ## Run integration tests (requires dev-up)
	go test ./tests/integration/... -v -count=1 -timeout 120s

test-e2e: ## Run end-to-end tests (requires dev-up)
	go test ./tests/e2e/... -v -count=1 -timeout 300s

# --- Code Quality ---

lint: ## Run linters and static analysis
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

fmt: ## Format Go code
	gofmt -w -s .

fmt-check: ## Check if code is formatted
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)
