.PHONY: test test-unit test-integration test-verbose test-coverage test-race fmt lint vet build help

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help:
	@echo "$(GREEN)Platform-Go Make Commands:$(NC)"
	@echo "  make test              - Run all tests"
	@echo "  make test-unit         - Run unit tests only"
	@echo "  make test-verbose      - Run tests with verbose output"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make test-race         - Run tests with race detector"
	@echo "  make coverage-html     - Generate HTML coverage report"
	@echo "  make fmt               - Format code with gofmt"
	@echo "  make lint              - Run linter (golangci-lint)"
	@echo "  make vet               - Run go vet"
	@echo "  make build             - Build API and scheduler binaries"
	@echo "  make build-api         - Build API binary only"
	@echo "  make build-scheduler   - Build scheduler binary only"
	@echo "  make clean             - Remove build artifacts"
	@echo "  make deps              - Download and verify dependencies"
	@echo "  make k8s-deploy        - Deploy all resources to Kubernetes"
	@echo "  make k8s-delete        - Delete all resources from Kubernetes"
	@echo "  make k8s-status        - Check Kubernetes resource status"
	@echo "  make k8s-logs-api      - Stream API server logs"
	@echo "  make k8s-logs-scheduler - Stream scheduler logs"
	@echo "  make ci                - Run CI pipeline (format check, lint, vet, test, build)"
	@echo "  make local-test        - Run local tests with coverage report"
	@echo "  make all               - Run full pipeline (CI + K8s deploy)"
	@echo "  make test-integration  - Run integration tests"
	@echo "  make test-integration-quick - Run quick integration tests"
	@echo "  make test-clean        - Clean test environment"

## Testing targets
test:
	@echo "$(YELLOW)Running all tests...$(NC)"
	@go test ./... -v

test-unit:
	@echo "$(YELLOW)Running unit tests (excluding integration)...$(NC)"
	@go test ./pkg/... ./internal/... -v -short

test-verbose:
	@echo "$(YELLOW)Running tests with verbose output...$(NC)"
	@go test ./pkg/... ./internal/... -v -count=1

test-coverage:
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	@go test ./... -v -coverprofile=coverage.out -covermode=atomic
	@echo "$(GREEN)Coverage report: coverage.out$(NC)"
	@go tool cover -func=coverage.out | tail -1

coverage-html: test-coverage
	@echo "$(YELLOW)Generating HTML coverage report...$(NC)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Open coverage.html in your browser$(NC)"

test-race:
	@echo "$(YELLOW)Running tests with race detector...$(NC)"
	@go test ./... -v -race

test-integration:
	@echo "$(YELLOW)Running integration tests...$(NC)"
	@echo "$(YELLOW)Note: Requires PostgreSQL and Kubernetes cluster$(NC)"
	@cd test/integration && cp .env.test ../../.env || true
	@go test -v -timeout 30m ./test/integration/...
	@echo "$(GREEN)Integration tests complete$(NC)"

test-integration-quick:
	@echo "$(YELLOW)Running quick integration tests (skipping slow tests)...$(NC)"
	@cd test/integration && cp .env.test ../../.env || true
	@go test -v -timeout 15m -short ./test/integration/...
	@echo "$(GREEN)Quick integration tests complete$(NC)"

test-integration-k8s:
	@echo "$(YELLOW)Running K8s integration tests only...$(NC)"
	@cd test/integration && cp .env.test ../../.env || true
	@go test -v -timeout 20m ./test/integration/ -run K8s
	@echo "$(GREEN)K8s integration tests complete$(NC)"

test-clean:
	@echo "$(YELLOW)Cleaning test environment...$(NC)"
	@kubectl get ns | grep test-integration | awk '{print $$1}' | xargs -r kubectl delete ns || true
	@dropdb platform_test 2>/dev/null || true
	@createdb platform_test 2>/dev/null || true
	@echo "$(GREEN)Test environment cleaned$(NC)"

## Code quality targets
fmt:
	@echo "$(YELLOW)Formatting code...$(NC)"
	@gofmt -w .
	@echo "$(GREEN)Code formatted$(NC)"

fmt-check:
	@echo "$(YELLOW)Checking code format...$(NC)"
	@if gofmt -l . | grep -q .; then \
		echo "$(RED)Format issues found:$(NC)"; \
		gofmt -l .; \
		exit 1; \
	else \
		echo "$(GREEN)Code is properly formatted$(NC)"; \
	fi

lint:
	@echo "$(YELLOW)Running golangci-lint...$(NC)"
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./... --timeout=5m; \
	elif [ -f $(HOME)/go/bin/golangci-lint ]; then \
		$(HOME)/go/bin/golangci-lint run ./... --timeout=5m; \
	elif [ -f $(HOME)/bin/golangci-lint ]; then \
		$(HOME)/bin/golangci-lint run ./... --timeout=5m; \
	else \
		echo "$(RED)golangci-lint not found. Installing...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		$(HOME)/go/bin/golangci-lint run ./... --timeout=5m; \
	fi

vet:
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet ./...

## Build targets
build: build-api build-scheduler
	@echo "$(GREEN)Build complete$(NC)"

build-api:
	@echo "$(YELLOW)Building API server...$(NC)"
	@go build -o platform-api ./cmd/api
	@echo "$(GREEN)Built: platform-api$(NC)"

build-scheduler:
	@echo "$(YELLOW)Building scheduler...$(NC)"
	@go build -o platform-scheduler ./cmd/scheduler
	@echo "$(GREEN)Built: platform-scheduler$(NC)"

## Dependency targets
deps:
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	@go mod download
	@go mod verify
	@echo "$(GREEN)Dependencies verified$(NC)"

## Cleanup
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -f platform-api platform-scheduler
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)Clean complete$(NC)"

## Kubernetes targets
k8s-deploy:
	@echo "$(YELLOW)Deploying to Kubernetes...$(NC)"
	@kubectl apply -f k8s/secret.yaml
	@kubectl apply -f k8s/postgres.yaml
	@kubectl apply -f k8s/ca.yaml
	@kubectl apply -f k8s/go-api.yaml
	@kubectl apply -f k8s/go-scheduler.yaml
	@echo "$(GREEN)Kubernetes deployment complete$(NC)"

k8s-delete:
	@echo "$(YELLOW)Deleting Kubernetes resources...$(NC)"
	@kubectl delete -f k8s/go-scheduler.yaml || true
	@kubectl delete -f k8s/go-api.yaml || true
	@kubectl delete -f k8s/ca.yaml || true
	@kubectl delete -f k8s/postgres.yaml || true
	@kubectl delete -f k8s/storage.yaml || true
	@kubectl delete -f k8s/secret.yaml || true
	@echo "$(GREEN)Kubernetes resources deleted$(NC)"

k8s-status:
	@echo "$(YELLOW)Checking Kubernetes resources...$(NC)"
	@kubectl get deployments
	@echo ""
	@kubectl get pods
	@echo ""
	@kubectl get svc

k8s-logs-api:
	@kubectl logs -f deployment/go-api --tail=100

k8s-logs-scheduler:
	@kubectl logs -f deployment/go-scheduler --tail=100

## Combined targets
ci: fmt-check lint vet test-unit build
	@echo "$(GREEN)CI checks passed$(NC)"

local-test: clean deps test coverage-html
	@echo "$(GREEN)Local testing complete. See coverage.html for details$(NC)"

all: ci k8s-deploy
	@echo "$(GREEN)Full build and deploy pipeline complete$(NC)"
