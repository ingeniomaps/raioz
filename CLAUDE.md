# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Raioz is a **meta-orchestrator** CLI (Go) for local microservice development. It does NOT replace existing tools — it **complements** them. Raioz reads a minimal `raioz.yaml`, auto-detects how each service runs (Docker Compose, Dockerfile, npm, Go, Make, etc.), and orchestrates them all under a shared network with HTTPS proxy and automatic service discovery.

**Core principle**: the developer uses their preferred tools; Raioz just connects, starts, and stops everything.

### Two config modes (backward compatible)
- **New**: `raioz.yaml` — minimal YAML with services (local) + dependencies (images)
- **Legacy**: `.raioz.json` — full JSON config with compose generation (still supported)

## Build & Development Commands

```bash
make build          # Build binary with version info (ldflags)
make test           # Run all tests: go test -v ./...
make lint           # golangci-lint (5min timeout)
make format         # gofmt + goimports
make check          # All checks: format + lint + check-i18n + tests
make check-i18n     # Verify i18n catalogs are in sync
make ci             # Full CI: check + build
make install        # Build and install to /usr/local/bin
make security       # gosec + govulncheck
```

Run a single test:
```bash
go test -v -run TestFunctionName ./internal/package/...
```

## Code Quality Constraints (enforced in CI)

- **Max 400 lines per file** (excluding tests) — `make check-lines`
- **Max 120 characters per line** — `make check-length`
- **Test coverage >= 80%** — `make check-coverage`
- **i18n catalogs in sync** — `make check-i18n`

## Architecture

Clean Architecture: `cmd/` → `internal/cli/` → `internal/app/` → `internal/domain/` → `internal/infra/`

### Core layers
- **cmd/raioz/**: Entry point, delegates to `internal/cli/`
- **internal/cli/**: Cobra commands. Thin: create deps, call use case, return error.
- **internal/app/**: Use cases with `Options` + `Execute()`. DI via `*Dependencies` struct.
- **internal/app/upcase/**: The `raioz up` orchestration (detect → start deps → start services → proxy).
- **internal/domain/interfaces/**: Port interfaces (DockerRunner, Orchestrator, ProxyManager, DiscoveryManager, etc.)
- **internal/infra/**: Thin adapters implementing domain interfaces.

### New meta-orchestrator packages
- **internal/detect/**: Scans a path, detects runtime (compose, dockerfile, npm, go, make, python, rust).
- **internal/orchestrate/**: Dispatcher + runners per runtime. ComposeRunner uses overlay (never modifies user's compose). DockerfileRunner builds+runs. HostRunner starts host processes. ImageRunner generates minimal compose per dependency.
- **internal/proxy/**: Caddy management. Generates Caddyfile, manages container lifecycle, mkcert integration for local HTTPS. Routes support ws/sse/grpc.
- **internal/discovery/**: Generates service discovery env vars based on cross-runtime network context (container→container, container→host, host→container, host→host).
- **internal/watch/**: fsnotify-based file watcher with debounce. Restarts services on file changes.
- **internal/graph/**: Dependency graph visualization (ASCII, DOT/Graphviz, JSON).
- **internal/snapshot/**: Volume backup/restore via `docker run alpine tar`.
- **internal/tunnel/**: Expose services via cloudflared or bore.

### Existing packages (preserved)
- **internal/config/**: Config loading (YAML + JSON), types, filtering, dependency resolution.
- **internal/docker/**: Docker operations, network/volume/port management, compose reading.
- **internal/state/**: State persistence. `LocalState` in `.raioz.state.json` (project dir). Docker is source of truth for running state.
- **internal/env/**: Environment variable resolution and templating.
- **internal/errors/**: Structured `RaiozError` with codes, context, suggestions.
- **internal/i18n/**: Internationalization with embedded JSON catalogs.
- **internal/git/**, **internal/host/**, **internal/lock/**, **internal/mocks/**: Unchanged.

## Config format (raioz.yaml)

```yaml
workspace: acme-corp        # optional, groups projects on same Docker network
project: e-commerce          # required
proxy: true                  # optional, enables Caddy + HTTPS

pre: ./scripts/fetch-secrets.sh   # run before up (secrets, env setup)
post: rm -f .env.*.tmp            # run after up (cleanup)

services:                    # what I'm developing (always local)
  api:
    path: ./api
    dependsOn: [postgres, redis]
    health: /api/health
    watch: true

dependencies:                # what I need running (Docker images)
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres
  redis:
    image: redis:7
```

## Key Concepts

- **Services** = local code I'm developing. Raioz detects runtime and starts with native tool.
- **Dependencies** = Docker images I need running. Pulled and started as containers.
- **`raioz dev`** = promote a dependency from image to local (hot-swap). `raioz dev --reset` reverts.
- **Proxy** = Caddy reverse proxy. `https://<service>.localhost` for all services. DNS aliases in Docker network for container-to-container resolution.
- **Service discovery** = auto-injected env vars (`POSTGRES_HOST`, `REDIS_URL`, etc.) with correct hosts based on caller/target runtime.
- **State** = `.raioz.state.json` in project dir (gitignored). Only stores what Docker can't tell us (dev overrides, host PIDs, ignored services).
- **Networking** = one Docker network per workspace. `host.docker.internal` for container→host. Caddy eliminates port conflicts.

## CLI Commands (27 total)

### Core
`up`, `down`, `status`, `logs`, `restart`, `exec`, `check`, `clean`, `init`, `doctor`

### Development
`dev` (hot-swap dep→local), `graph` (visualize deps), `snapshot` (backup volumes), `tunnel` (expose to internet), `proxy` (manage Caddy)

### Management
`workspace`, `list`, `version`, `lang`, `ignore`, `volumes`, `compare`, `ci`, `health`, `migrate`

## Dependencies

- **CLI**: spf13/cobra
- **JSON Schema**: xeipuuv/gojsonschema
- **YAML**: gopkg.in/yaml.v3
- **File watching**: fsnotify/fsnotify
- **Go version**: 1.22

## Patterns

- Dependency injection via `Dependencies` struct (never create deps inline)
- All user messages through `i18n.T()` — never hardcode user-facing strings
- Structured errors: `errors.New(code, i18n.T("error.xxx")).WithSuggestion(...)`
- Tests co-located with source, table-driven with `t.Run`; mocks in `internal/mocks/`
- Compose overlay: never modify user's compose file, use `-f original.yml -f raioz-overlay.yml`
- Detection priority: compose > Dockerfile > package.json > go.mod > Makefile > pyproject.toml > Cargo.toml
- Commit messages: Conventional Commits, English, imperative, max 50 char subject
