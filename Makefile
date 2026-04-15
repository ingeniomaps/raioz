.PHONY: help lint format test test-coverage check-coverage build install clean
.PHONY: check-lines check-length check-i18n check ci
.PHONY: integration-test generate mock security

# Find flags shared by check-lines and check-length
FIND_GO = find . -name "*.go" ! -name "*_test.go" \
	! -path "*/vendor/*" ! -path "*/.context/*"

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  %-25s %s\n", $$1, $$2}'

lint: ## Run golangci-lint
	@echo "Running linter..."
	@golangci-lint run --build-tags=""

format: ## Format code with gofmt and goimports
	@echo "Formatting code..."
	@gofmt -s -w .
	@goimports -w .

test: ## Run all tests
	@echo "Running tests..."
	@GOGC=50 GOMEMLIMIT=4GiB go test -v -p 1 -parallel 2 -timeout 120s ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@GOGC=50 GOMEMLIMIT=4GiB go test -v -p 1 -parallel 2 -timeout 120s \
		-coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@echo ""
	@go tool cover -func=coverage.out | grep -E "^total:"

COVERAGE_THRESHOLD ?= 70

check-coverage: test-coverage ## Check coverage against threshold (default: 70%)
	@./scripts/check-coverage.sh $(COVERAGE_THRESHOLD)

build: ## Build the binary
	@echo "Building binary..."
	@VERSION=$$(git describe --tags --exact-match 2>/dev/null \
		|| git describe --tags 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	DATE=$$(date +%Y-%m-%dT%H:%M:%S 2>/dev/null || echo "unknown"); \
	go build -buildvcs=false \
		-ldflags "\
			-X 'raioz/internal/cli.Version=$$VERSION' \
			-X 'raioz/internal/cli.Commit=$$COMMIT' \
			-X 'raioz/internal/cli.BuildDate=$$DATE'" \
		-o raioz ./cmd/raioz

install: build ## Build and install to /usr/local/bin
	@echo "Installing binary..."
	@./install.sh

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f raioz coverage.out coverage.html
	@go clean ./...

check-lines: ## Check for files exceeding 400 lines
	@echo "Checking for files exceeding 400 lines..."
	@files=$$($(FIND_GO) ! -name "schema.go" \
		-exec sh -c \
		'lines=$$(wc -l < "$$1"); \
		if [ "$$lines" -gt 400 ]; then echo "$$1: $$lines lines"; fi' \
		_ {} \;); \
	if [ -n "$$files" ]; then \
		echo "❌ Files exceeding 400 lines found:"; \
		echo "$$files"; \
		exit 1; \
	else \
		echo "✅ All files are under 400 lines"; \
	fi

check-length: ## Check for lines exceeding 120 characters
	@echo "Checking for lines exceeding 120 characters..."
	@lines=$$($(FIND_GO) \
		-exec awk 'length > 120 {print FILENAME":"NR": "$$0}' {} \;); \
	if [ -n "$$lines" ]; then \
		echo "❌ Lines exceeding 120 characters found:"; \
		echo "$$lines"; \
		exit 1; \
	else \
		echo "✅ All lines are under 120 characters"; \
	fi

check-i18n: ## Verify all i18n catalogs have the same keys
	@echo "Checking i18n catalog completeness..."
	@go test -run TestCatalogCompleteness -count=1 ./internal/i18n/ \
		&& echo "All catalogs in sync"

check: format check-lines check-length check-i18n lint test ## Run all checks

integration-test: build ## Run E2E integration tests (requires Docker)
	@echo "Running integration tests..."
	@./scripts/integration-test.sh ./raioz

generate: ## Generate code (mocks, etc.)
	@echo "Generating code..."
	@go generate ./...

mock: ## Generate mocks using mockery
	@if ! command -v mockery > /dev/null; then \
		echo "❌ mockery not found."; \
		echo "Install: go install github.com/vektra/mockery/v2@latest"; \
		exit 1; \
	fi
	@echo "Generating mocks..."
	@mockery

security: ## Run security scans (gosec + govulncheck)
	@echo "Running security scans..."
	@echo "=== Running gosec ==="
	@if command -v gosec > /dev/null; then \
		gosec ./... || true; \
	else \
		echo "⚠️  gosec not installed."; \
		echo "Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi
	@echo ""
	@echo "=== Running govulncheck ==="
	@if command -v govulncheck > /dev/null; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck not installed."; \
		echo "Install: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

ci: check build ## Run CI pipeline (all checks + build)
