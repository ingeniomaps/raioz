.PHONY: lint format test build install clean check-lines check-length test-coverage check-coverage security

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

lint: ## Run golangci-lint
	@echo "Running linter..."
	@golangci-lint run --build-tags=""

format: ## Format code with gofmt and goimports
	@echo "Formatting code..."
	@gofmt -s -w .
	@goimports -w .

test: ## Run all tests
	@echo "Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@echo ""
	@go tool cover -func=coverage.out | grep -E "^total:"

check-coverage: test-coverage ## Check coverage against threshold (default: 80%)
	@./scripts/check-coverage.sh $(COVERAGE_THRESHOLD)

build: ## Build the binary
	@echo "Building binary..."
	@VERSION=$$(git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	DATE=$$(date +%Y-%m-%dT%H:%M:%S 2>/dev/null || echo "unknown"); \
	go build -buildvcs=false \
		-ldflags "-X 'raioz/cmd.Version=$$VERSION' -X 'raioz/cmd.Commit=$$COMMIT' -X 'raioz/cmd.BuildDate=$$DATE'" \
		-o raioz ./cmd/raioz

install: build ## Build and install the binary to /usr/local/bin (development mode - uses local binary)
	@echo "Installing binary to $${INSTALL_DIR:-/usr/local/bin}..."
	@if [ ! -f "./raioz" ]; then \
		echo "Error: Binary not found after build. Run 'make build' first."; \
		exit 1; \
	fi
	@echo "Using install.sh in development mode (will use local binary)..."
	@./install.sh

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f raioz coverage.out coverage.html
	@go clean ./...

check-lines: ## Check for files exceeding 400 lines
	@echo "Checking for files exceeding 400 lines..."
	@files=$$(find . -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" ! -path "*/.ia/*" -exec sh -c 'lines=$$(wc -l < "$$1"); if [ "$$lines" -gt 400 ]; then echo "$$1: $$lines lines"; fi' _ {} \;); \
	if [ -n "$$files" ]; then \
		echo "❌ Files exceeding 400 lines found:"; \
		echo "$$files"; \
		exit 1; \
	else \
		echo "✅ All files are under 400 lines"; \
	fi

check-length: ## Check for lines exceeding 120 characters
	@echo "Checking for lines exceeding 120 characters..."
	@lines=$$(find . -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" ! -path "*/.ia/*" -exec awk 'length > 120 {print FILENAME":"NR": "$$0}' {} \;); \
	if [ -n "$$lines" ]; then \
		echo "❌ Lines exceeding 120 characters found:"; \
		echo "$$lines"; \
		exit 1; \
	else \
		echo "✅ All lines are under 120 characters"; \
	fi

check: format check-lines check-length lint test ## Run all checks (format, lint, tests, line checks)

generate: ## Generate code (mocks, etc.)
	@echo "Generating code..."
	@go generate ./...

mock: ## Generate mocks using mockery
	@if ! command -v mockery > /dev/null; then \
		echo "❌ mockery not found. Install with: go install github.com/vektra/mockery/v2@latest"; \
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
		echo "⚠️  gosec not installed. Run: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		echo "⚠️  Note: gosec is also available via golangci-lint (make lint)"; \
	fi
	@echo ""
	@echo "=== Running govulncheck ==="
	@if command -v govulncheck > /dev/null; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck not installed. Run: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
		echo "⚠️  Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
		govulncheck ./...; \
	fi

ci: check build ## Run CI pipeline (all checks + build)

release-installer: build ## Prepare installer script (for GitHub release)
	@echo "Preparing installer script for release..."
	@chmod +x install.sh
	@echo "✓ Installer script is ready"
	@echo ""
	@echo "The installer script has dual behavior:"
	@echo "  - Development mode: Uses local binary if found (for developers)"
	@echo "  - Production mode: Downloads from GitHub releases (for users)"
	@echo ""
	@echo "For developers (local installation):"
	@echo "  make install"
	@echo ""
	@echo "For users (GitHub installation):"
	@echo "  curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/YOUR_REPO/main/install.sh | sh"
	@echo ""
	@echo "Note: Make sure to upload the built binary to GitHub releases for users"

.PHONY: release-installer
