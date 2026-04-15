# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Raioz is a **meta-orchestrator** CLI (Go) for local microservice development. It does NOT replace existing tools — it **complements** them. Raioz reads a minimal `raioz.yaml`, auto-detects how each service runs (Docker Compose, Dockerfile, npm, Go, Make, etc.), and orchestrates them all under a shared network with HTTPS proxy and automatic service discovery.

**Core principle**: the developer uses their preferred tools; Raioz just connects, starts, and stops everything.

### Config modes
- **Primary**: `raioz.yaml` — minimal YAML with services (local) + dependencies (images)
- **Legacy**: `.raioz.json` — deprecated with migration warning (`raioz migrate yaml`)

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

- **Max 400 lines per file** (excluding tests + `internal/config/schema.go` JSON blob) — `make check-lines`
- **Max 120 characters per line** — `make check-length`
- **Test coverage >= 73%** — `make check-coverage` (raised from 70% in v0.2.0; mocks/testing packages excluded from the metric. See [ROADMAP.md](ROADMAP.md) for the path back to 80%)
- **i18n catalogs in sync** — `make check-i18n`

### Lint baseline (reduced for v0.1.0)
`.golangci.yml` currently enables only: `govet`, `staticcheck`, `unused`,
`ineffassign`, `gofmt`, `goimports`, `misspell`, `whitespace`,
`copyloopvar`, `bodyclose`. The strict config previously in place fired
~2,500 opinionated issues on this codebase; re-introducing stricter
linters one at a time (errcheck → gosec → revive → wrapcheck) is a
tracked v0.2.0 task. See `ROADMAP.md`.

## Architecture

Clean Architecture: `cmd/` → `internal/cli/` → `internal/app/` → `internal/domain/` → `internal/infra/`

### Core layers
- **cmd/raioz/**: Entry point, delegates to `internal/cli/`
- **internal/cli/**: Cobra commands. Thin: create deps, call use case, return error.
- **internal/app/**: Use cases with `Options` + `Execute()`. DI via `*Dependencies` struct.
- **internal/app/upcase/**: The `raioz up` orchestration (detect → start deps → start services → proxy).
- **internal/domain/interfaces/**: Port interfaces (DockerRunner, Orchestrator, ProxyManager, DiscoveryManager, etc.)
- **internal/infra/**: Thin adapters implementing domain interfaces.

### Key packages
- **internal/detect/**: Scans a path, detects runtime (24 runtimes: compose, dockerfile, npm, go, python, rust, etc.).
- **internal/orchestrate/**: Dispatcher + runners per runtime. ComposeRunner uses overlay. DockerfileRunner builds+runs. HostRunner starts host processes. ImageRunner generates compose per dependency.
- **internal/proxy/**: Caddy management. Generates Caddyfile, mkcert integration for local HTTPS. Routes support ws/sse/grpc.
- **internal/discovery/**: Service discovery env vars based on cross-runtime network context.
- **internal/watch/**: fsnotify-based file watcher with debounce. Restarts services on file changes.
- **internal/config/**: Config loading (YAML + JSON), types, filtering, dependency resolution.
- **internal/docker/**: Docker operations, network/volume/port management, compose reading.
- **internal/state/**: State persistence. `LocalState` in `.raioz.state.json` (project dir).
- **internal/env/**: Environment variable resolution and templating.
- **internal/errors/**: Structured `RaiozError` with codes, context, suggestions.
- **internal/i18n/**: Internationalization with embedded JSON catalogs.
- **internal/naming/**: Centralized naming conventions for Docker resources.
- **internal/runtime/**: Docker/Podman/nerdctl runtime abstraction.
- **internal/tui/**: Interactive dashboard (bubbletea).
- **internal/resilience/**: Retry logic and circuit breakers.
- **internal/graph/**: Dependency graph visualization (ASCII, DOT/Graphviz, JSON).
- **internal/snapshot/**: Volume backup/restore via `docker run alpine tar`.
- **internal/tunnel/**: Expose services via cloudflared or bore.
- **internal/git/**, **internal/host/**, **internal/lock/**, **internal/mocks/**: Git ops, host processes, file locking, test mocks.

## Config format (raioz.yaml)

```yaml
workspace: acme-corp        # optional, groups projects on same Docker network
project: e-commerce         # required

# Caddy reverse proxy. Three shapes:
#   proxy: true                       # simple, binds host 80/443
#   proxy: { domain: acme.dev }       # custom domain
#   proxy:                            # full control
#     domain: acme.dev
#     ip: 172.28.1.1                  # optional; default <subnet>.1.1
#     publish: false                  # optional; don't bind host ports,
#                                     # reach via container IP + /etc/hosts
proxy:
  domain: acme.dev

network:                    # optional, string shorthand or object
  name: acme-net            # override the derived network name
  subnet: 172.28.0.0/16     # pin subnet so containers get deterministic IPs

pre: ./scripts/fetch-secrets.sh   # run before up (secrets, env setup)
post: rm -f .env.*.tmp            # run after up (cleanup)

services:                   # what I'm developing (always local)
  api:
    path: ./api
    dependsOn: [postgres, redis]
    health: /api/health
    watch: true
  keycloak:
    path: ./keycloak
    command: make start     # user-supplied launch
    stop: make stop
    proxy:                  # override when detection can't see the target
      target: hypixo-keycloak
      port: 8080

dependencies:               # what I need running (Docker images)
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres
    # name: my-pg           # optional literal container name override
  redis:
    image: redis:7
  adminer:
    image: adminer
    routing: {}             # opt-in HTTP proxy for a DB-heuristic image
```

### Optional fields recently added (all backward compatible)
- `network.name` / `network.subnet` — pin Docker network scope + IP range.
- `dependencies.<n>.name` — literal container name override (useful when external tooling expects a specific name).
- `dependencies.<n>.routing` — opt a dep whose image matches the DB/broker heuristic (postgres, redis, mysql, mongo, ...) into getting an HTTPS route from Caddy.
- `services.<n>.proxy.{target,port}` — bypass runtime detection for proxy routing. Needed when `command:` launches a compose stack raioz can't see.
- `proxy.ip` — pin the Caddy container's IP inside the Docker network. Default: `<subnet-base>.1.1` when `network.subnet` is declared, otherwise Docker auto-assigns.
- `proxy.publish` (default `true`) — when `false`, the proxy does NOT bind host ports 80/443. The proxy is reachable only through its container IP; users map hostnames via `/etc/hosts`. Enables running multiple workspaces in parallel without port contention. Requires a deterministic IP (subnet or explicit `proxy.ip`). Linux-only (macOS/Windows route Docker through a VM whose bridge IPs aren't reachable).
- `services.<n>.proxy.{target,port}` — bypass runtime detection for proxy routing. Needed when `command:` launches a compose stack raioz can't see.

## Key Concepts

- **Services** = local code I'm developing. Raioz detects runtime and starts with native tool.
- **Dependencies** = Docker images I need running. Pulled and started as containers.
- **`raioz dev`** = promote a dependency from image to local (hot-swap). `raioz dev --reset` reverts.
- **Proxy** = Caddy reverse proxy. `https://<service>.localhost` for all services. DNS aliases in Docker network for container-to-container resolution.
- **Service discovery** = auto-injected env vars (`POSTGRES_HOST`, `REDIS_URL`, etc.) with correct hosts based on caller/target runtime.
- **State** = `.raioz.state.json` in project dir (gitignored). Only stores what Docker can't tell us (dev overrides, host PIDs, ignored services).
- **Networking** = one Docker network per workspace. `host.docker.internal` for container→host. Caddy eliminates port conflicts.

### Container labels (BUG-2 fix — do not remove)
Every container raioz creates is stamped with:
- `com.raioz.managed=true`
- `com.raioz.kind=service|dependency|proxy`
- `com.raioz.workspace=<ws>` (when workspace is set)
- `com.raioz.project=<proj>` (**omitted on shared deps** — that's the signal)
- `com.raioz.service=<name>` (service / dep / "proxy")

`raioz down` sweeps by labels, not by name prefix. Any new runner MUST stamp
these labels or its containers will leak. See `internal/naming/labels.go`
and `orchestrate/image_runner.go`.

### Shared deps (workspace-scoped)
When `workspace:` is set OR the user writes `dependencies.<n>.name:`:
- Container name comes from `naming.DepContainer(project, dep, override)`.
- Without override: `{workspace}-{dep}` (e.g. `acme-postgres`), NOT per-project.
- Shared deps omit `com.raioz.project` so `raioz down` of any one project
  does NOT tumba them. Last project out tears them down via
  `otherProjectsActiveInWorkspace` — no ref-count state file.
- `ImageRunner.Start` is idempotent: if the container is already running
  (sibling project brought it up), it returns without re-creating.

### Proxy certs (per-domain namespace)
Certificates live in `~/.raioz/certs/<domain>/`. `EnsureCerts(domain)`
validates SAN includes both `<domain>` and `*.<domain>` before reusing —
prevents the "acme.localhost cert reused for hypixo.dev" class of bug.

### Caddyfile global options
`auto_https off` for `tls: mkcert` (ACME would hang on custom domains with
no public DNS). Do not revert to `disable_redirects` alone — that only
silences the HTTP→HTTPS redirect, not the ACME pipeline.

### Workspace-shared proxy lifecycle
When `workspace:` is set, a single `{workspace}-proxy` Caddy fronts every
project in the workspace. Routes are persisted per project at
`/tmp/<workspace>/proxy/routes/<project>.json`; the shared Caddyfile is the
union. `raioz down` removes only the current project's routes and reloads
Caddy; only the last project leaving the workspace tumba the proxy.

### Clone functions must stay in sync with config structs
`internal/app/upcase/workspace_project_conflict.go` has two clone functions
(`cloneService`, `cloneInfraEntry`) used by the workspace-merge path. When
adding a field to `config.Service` or `config.Infra` that affects
orchestration, you MUST also list it in the clone or it will silently vanish
on re-up after any workspace conflict. Every past regression in this area
traced to a missing field.

### Image classification shared via `internal/proxy/filter.go`
`proxy.IsNonHTTPImage(image)` is the source of truth for "does this dep
speak HTTP?". Bare-name match (last path segment before tag/digest), NOT
substring — substring matching falsely tagged `redis/redisinsight` as
Redis. New blocklist entries go in `nonHTTPImageNames`.

### `raioz hosts`
Prints the `/etc/hosts` line for the current project's proxy, ready for
`sudo tee -a /etc/hosts`. Only useful in practice with `proxy.publish:
false`, but works in any mode.

## CLI Commands (30 total)

### Core
`up`, `down`, `status`, `logs`, `restart`, `exec`, `check`, `clean`, `init`, `doctor`, `clone`

### Development
`dev` (hot-swap dep→local), `env` (show service env vars), `graph` (visualize deps), `snapshot` (backup volumes), `tunnel` (expose to internet), `proxy` (manage Caddy), `dashboard` (interactive TUI), `hosts` (print `/etc/hosts` line for `proxy.publish:false` setups)

### Management
`list`, `version`, `lang`, `ignore`, `volumes`, `compare`, `ci`, `health`, `migrate`, `ports`, `yaml` (migrate config)

## Dependencies

- **CLI**: spf13/cobra
- **TUI**: charmbracelet/bubbletea, charmbracelet/lipgloss
- **JSON Schema**: xeipuuv/gojsonschema
- **YAML**: gopkg.in/yaml.v3
- **File watching**: fsnotify/fsnotify
- **Go version**: 1.24

## Patterns

- Dependency injection via `Dependencies` struct (never create deps inline)
- All user messages through `i18n.T()` — never hardcode user-facing strings
- Structured errors: `errors.New(code, i18n.T("error.xxx")).WithSuggestion(...)`
- Tests co-located with source, table-driven with `t.Run`; mocks in `internal/mocks/`
- Compose overlay: never modify user's compose file, use `-f original.yml -f raioz-overlay.yml`
- Detection priority: compose > Dockerfile > package.json > go.mod > Makefile > pyproject.toml > Cargo.toml
- Commit messages: Conventional Commits, English, imperative, max 50 char subject
