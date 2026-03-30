.PHONY: help dev-up dev-down test test-unit test-standard ci-smoke ci-unit ci-web ci-cli-smoke ci-local lint seed logs build clean demo workbench fmt fmt-check

CLUSTER_NAME := ottoplus-dev
K3D_CONFIG := deploy/k3d-config.yaml

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Development Environment ---

dev-up: ## Create k3d cluster + LocalStack + install CRDs (does not start API or operator)
	@./scripts/dev-up.sh

dev-down: ## Teardown k3d cluster and all resources
	@./scripts/dev-down.sh

seed: ## Seed test data into the local environment
	@./scripts/seed.sh

logs: ## Tail logs from all components in the cluster
	@kubectl logs -n ottoplus --all-containers=true -l app.kubernetes.io/part-of=ottoplus -f --tail=100

# --- Build ---

build: ## Build API server and operator binaries
	go build -o bin/api-server ./cmd/api
	go build -o bin/operator ./cmd/operator

clean: ## Remove build artifacts
	rm -rf bin/

# --- Demo ---

demo: build ## Run the API server locally (no K8s needed) and print sample curl commands
	@echo ""
	@echo "Starting ottoplus API server on :8080 ..."
	@echo "Try these commands in another terminal:"
	@echo ""
	@echo "  curl -s http://localhost:8080/healthz | jq ."
	@echo "  curl -s http://localhost:8080/v1/blocks | jq ."
	@echo "  curl -s -X POST http://localhost:8080/v1/compositions/topology \\"
	@echo "    -H 'Content-Type: application/json' \\"
	@echo "    -d @deploy/examples/sample-composition.json | jq ."
	@echo ""
	@./bin/api-server -addr :8080

workbench: build ## Start API server + workbench together (credential-source surfaces available by default)
	@cd web && npm install --silent
	@echo ""
	@echo "Starting ottoplus API server on :8080 and workbench on :5173 ..."
	@echo "Open http://localhost:5173 in your browser."
	@echo ""
	@./bin/api-server -addr :8080 & API_PID=$$!; \
		cd web && npx vite --host 2>/dev/null & WEB_PID=$$!; \
		trap 'kill $$API_PID $$WEB_PID 2>/dev/null' EXIT INT TERM; \
		wait

# --- Testing ---

test: test-unit ## Run all tests (default: unit only)

test-unit: ## Run unit tests (no infra required)
	go test ./src/core/... ./src/api/... ./src/operator/blocks/... ./src/operator/reconciler/... -v -count=1

test-standard: ## Run 4-block standard credential path tests only
	go test ./deploy/examples/... ./src/core/compiler/... ./src/api/... ./src/operator/reconciler/... \
		-run 'TestStandard|TestCompile_CredentialPath|TestValidateComposition_StandardPath|TestTopology_StandardPath|TestAutoWire_StandardPath|TestOperatorCompiler_StandardPath' \
		-v -count=1

# --- CI (local mirror of GitHub Actions) ---

ci-smoke: build test-standard ## Local mirror of CI smoke workflow (build + standard path tests)

ci-unit: test ## Local mirror of CI unit workflow (full unit tests)

ci-web: ## Run frontend checks (test + build)
	cd web && npm install --silent && npm run ci

ci-cli-smoke: ## Run CLI smoke (8 commands from README/QUICKSTART: table + JSON)
	go run ./cmd/ottoplus blocks list
	go run ./cmd/ottoplus compose validate --file deploy/examples/sample-composition.json
	go run ./cmd/ottoplus compose auto-wire --file deploy/examples/sample-composition.json
	go run ./cmd/ottoplus compose topology --file deploy/examples/sample-composition.json
	go run ./cmd/ottoplus blocks list --format json
	go run ./cmd/ottoplus compose validate --file deploy/examples/sample-composition.json --format json
	go run ./cmd/ottoplus compose auto-wire --file deploy/examples/sample-composition.json --format json
	go run ./cmd/ottoplus compose topology --file deploy/examples/sample-composition.json --format json

ci-local: ci-smoke ci-unit ci-web ci-cli-smoke ## Run all CI checks locally in sequence

# --- Code Quality ---

lint: ## Run linters and static analysis
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

fmt: ## Format Go code
	gofmt -w -s .

fmt-check: ## Check if code is formatted
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)
