# Raioz

[![Release](https://img.shields.io/github/v/release/ingeniomaps/raioz?sort=semver)](https://github.com/ingeniomaps/raioz/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/ingeniomaps/raioz/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ingeniomaps/raioz/actions/workflows/ci.yml)

**One command to run your entire architecture.**

Raioz is a meta-orchestrator for local development. It doesn't replace your tools — it connects them. Bring your Docker Compose files, Dockerfiles, npm projects, Go services, Makefiles — Raioz detects what you have and runs everything together with shared networking, HTTPS, and automatic service discovery.

```bash
raioz up       # everything running, connected, HTTPS ready
```

## Why Raioz?

**The problem:** You have 5 microservices. One uses Docker Compose, another is a Go binary, the frontend is npm, and you need Postgres and Redis. Getting them all to talk to each other locally is a mess of ports, env vars, and manual Docker commands.

**The solution:** Raioz reads each service's native config (compose, Dockerfile, package.json, go.mod), starts them all, puts them on the same network, and injects the right environment variables so they find each other automatically.

```
raioz up

  ✓ postgres (image)      :5432
  ✓ redis (image)          :6379
  ✓ api (go)               https://api.acme.localhost
  ✓ frontend (npm)         https://frontend.acme.localhost

  Watching 1 service(s): [api]
  Press Ctrl+C to stop

  api      │ API starting on :8080
  frontend │ Frontend running on :3000
  redis    │ Ready to accept connections
  postgres │ database system is ready
```

## Install

Pre-built binaries are shipped for **Linux** and **macOS** on both
`amd64` and `arm64`. Windows users run raioz through WSL2 or build
from source with `go install`.

### Recommended (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
```

No-sudo variant (installs to `~/.local/bin`):

```bash
INSTALL_DIR=~/.local/bin bash -c "$(curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh)"
```

### Any platform (Go toolchain)

```bash
go install github.com/ingeniomaps/raioz/cmd/raioz@latest
```

### From source

```bash
git clone https://github.com/ingeniomaps/raioz.git
cd raioz && make install
```

Verify the install:

```bash
raioz version
raioz doctor   # checks Docker, mkcert, Git, etc.
```

## Quick Start

### Option A: Zero-config (just run it)

```bash
cd your-project/
raioz up          # auto-detects services and dependencies
```

Raioz scans subdirectories for project files (`go.mod`, `package.json`, `Dockerfile`, etc.) and reads `.env` files to infer infrastructure needs (`DATABASE_URL=postgres://...` adds postgres automatically).

### Option B: Generate config

```bash
cd your-project/
raioz init        # scans and generates raioz.yaml
raioz up          # done
```

### Option C: Write config manually

```yaml
# raioz.yaml
project: my-app

services:
  api:
    path: ./api
    dependsOn: [postgres, redis]
    watch: true

  frontend:
    path: ./frontend
    watch: native

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres

  redis:
    image: redis:7
```

## Supported Runtimes (24)

Raioz auto-detects the runtime from project files:

| Runtime | Trigger file | Start command |
|---------|-------------|---------------|
| Docker Compose | `compose.yml` | `docker compose up` |
| Dockerfile | `Dockerfile` | `docker build + run` |
| **Node.js** | `package.json` | `npm run dev` / `yarn` / `pnpm` / `bun` |
| **Go** | `go.mod` | `go run .` (or `air` if `.air.toml` exists) |
| **Python** | `pyproject.toml` | `python -m flask run` |
| **Rust** | `Cargo.toml` | `cargo run` |
| **PHP** | `composer.json` | `php artisan serve` (Laravel) / `php -S` |
| **Java/Kotlin** | `pom.xml` / `build.gradle` | `./mvnw spring-boot:run` / `./gradlew bootRun` |
| **C#/.NET** | `*.csproj` | `dotnet watch run` |
| **Ruby** | `Gemfile` | `bundle exec rails server` |
| **Elixir** | `mix.exs` | `mix phx.server` |
| **Scala** | `build.sbt` | `sbt run` |
| **Swift** | `Package.swift` | `swift run` |
| **Dart** | `pubspec.yaml` | `dart run` |
| **Clojure** | `deps.edn` / `project.clj` | `clj -M:dev` / `lein run` |
| **Haskell** | `stack.yaml` / `*.cabal` | `stack run` / `cabal run` |
| **Zig** | `build.zig` | `zig build run` |
| **Gleam** | `gleam.toml` | `gleam run` |
| **Deno** | `deno.json` | `deno task dev` |
| **Bun** | `bunfig.toml` | `bun run dev` |
| Make | `Makefile` | `make dev` |
| Just | `justfile` | `just dev` |
| Task | `Taskfile.yml` | `task dev` |

Package managers are auto-detected from lock files: `yarn.lock` → yarn, `pnpm-lock.yaml` → pnpm, `bun.lockb` → bun.

## Container Runtimes

Raioz defaults to Docker but supports any compatible CLI:

```bash
# Docker (default)
raioz up

# Podman
RAIOZ_RUNTIME=podman raioz up

# nerdctl (containerd)
RAIOZ_RUNTIME=nerdctl raioz up
```

## Key Features

### HTTPS everywhere

```yaml
proxy: true                    # simple
proxy:
  domain: acme.localhost       # custom domain
```

Every service gets `https://<name>.acme.localhost`. Uses Caddy + mkcert. The domain structure mirrors production: `api.acme.localhost` → `api.acme.com`.

### Workspace-shared proxy

When `workspace:` is declared, a single Caddy instance fronts every project in the workspace. Each project adds its own routes on `raioz up`; `raioz down` removes only that project's routes. The last project leaving tumba the proxy.

```yaml
workspace: acme-corp
project: e-commerce
proxy:
  domain: acme.localhost
```

Run two sibling projects at the same time — both reachable through the same proxy, no port contention:

```bash
cd ~/work/e-commerce && raioz up
cd ~/work/admin-panel && raioz up   # joins the shared acme-corp proxy
```

### Multi-workspace parallelism (no port contention)

Set `proxy.publish: false` to skip binding host ports 80/443. The proxy becomes reachable only via its container IP, so multiple workspaces run in parallel, each on its own subnet. Users map hostnames via `/etc/hosts`:

```yaml
workspace: acme-corp
project: e-commerce
network:
  subnet: 172.28.0.0/16   # pinned so IPs are deterministic
proxy:
  domain: acme.localhost
  publish: false          # don't bind host 80/443
```

```bash
raioz hosts | sudo tee -a /etc/hosts
# 172.28.1.1 api.acme.localhost frontend.acme.localhost   # raioz:acme-corp
```

Linux-only (macOS and Windows route Docker through a VM whose bridge IPs aren't reachable from the host).

### Three modes of operation

```bash
raioz up                 # detach: start everything and exit
raioz up --attach        # foreground: stream all logs, Ctrl+C to stop
raioz up --watch api     # hot-reload mode: auto-restart 'api' on file change
```

### Multiplexed logs

When running in foreground mode (`--attach` or `watch: true`), logs from all services are shown with colored prefixes:

```
  api      │ GET /health 200 2ms
  frontend │ compiled successfully
  postgres │ database system is ready
  redis    │ Ready to accept connections
```

### File watching

```yaml
services:
  api:
    watch: true       # Raioz restarts on file changes (debounced)
  frontend:
    watch: native     # Service has its own hot-reload (Next.js, Vite)
```

### Automatic service discovery

Each service gets environment variables with the correct hosts:

```bash
# Host process calling a Docker container:
POSTGRES_HOST=localhost
REDIS_URL=http://localhost:6379

# Docker container calling another container:
POSTGRES_HOST=postgres
REDIS_URL=http://redis:6379

# With proxy:
API_URL=https://api.acme.localhost
```

### Smart init

`raioz init` scans your project and infers everything:

- Detects services from subdirectory structure.
- Reads `.env` files to find infrastructure needs.
- Auto-detects `.env.postgres` files and wires them to dependencies.
- Wires `dependsOn` automatically.

### Workspace naming

```yaml
workspace: acme          # optional
project: e-commerce
```

With workspace set, Docker resources use it as prefix instead of `raioz-`:

```
acme-e-commerce-api        # container (service)
acme-postgres              # container (shared dependency)
acme-net                   # network
acme-proxy                 # Caddy (workspace-shared)
```

### Reliable teardown via container labels

Every container raioz creates is stamped with labels (`com.raioz.managed=true`, `com.raioz.project=<p>`, `com.raioz.service=<s>`, `com.raioz.kind=service|dependency|proxy`). `raioz down` sweeps by label, never by name prefix — so it can't tumba containers from an unrelated project that happens to share a name.

### Process lifecycle

Host processes (Go, npm, etc.) are tracked with PIDs:

```bash
raioz status
  Dependencies (2)
    postgres    running    0.02%    66MB    postgres:16
    redis       running    0.23%    16MB    redis:7

  Services (2)
    api         go         running    pid:12345
    frontend    npm        running    pid:12346
```

`raioz down` kills all processes. `raioz up` cleans stale processes before starting.

### More commands

```bash
raioz list                     # all active projects across workspaces
raioz graph                    # visualize dependency graph (ASCII/DOT/JSON)
raioz dev postgres ./local-pg  # hot-swap dependency to local code
raioz snapshot create backup   # backup Docker volumes
raioz tunnel api               # expose to internet via cloudflared
raioz env api                  # show resolved env vars for a service
raioz hosts                    # print /etc/hosts line for proxy.publish:false setups
raioz doctor                   # check system requirements
raioz clean                    # remove stopped containers
raioz ports                    # list port mappings across projects
raioz dashboard                # interactive TUI
```

See `raioz --help` for the full command list (30 total).

## Configuration

Full reference lives in [docs/CONFIG_REFERENCE.md](docs/CONFIG_REFERENCE.md). Common fields:

```yaml
# raioz.yaml
workspace: acme-corp            # optional: groups projects on same network
project: e-commerce             # required

network:                        # optional: pin network name and/or subnet
  name: acme-net                # override derived name
  subnet: 172.28.0.0/16         # deterministic container IPs

proxy:                          # optional: Caddy + HTTPS
  domain: acme.localhost
  ip: 172.28.1.1                # pin proxy container IP (default: <subnet>.1.1)
  publish: false                # skip host 80/443 binding (multi-workspace mode)

pre: ./scripts/fetch-secrets.sh # run before starting
post: rm -f .env.*.tmp          # run after starting

services:                       # local code you edit
  api:
    path: ./api
    dependsOn: [postgres, redis]
    watch: true
    health: /api/health
    ports: ["3000"]
    env: .env.api
    routing:
      ws: true                  # WebSocket support
      stream: true              # SSE / streaming
      grpc: true                # gRPC (h2c)
    proxy:                      # override when detection can't see the target
      target: hypixo-keycloak   # container name on the shared network
      port: 8080

  frontend:
    path: ./frontend
    watch: native               # service has its own hot-reload

dependencies:                   # Docker images
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres
    # name: my-pg               # optional literal container name override

  adminer:
    image: adminer
    routing: {}                 # opt-in HTTPS route for a DB-heuristic image

  legacy-stack:
    compose: ./infra/legacy.yml # bring an existing compose fragment
    env: .env.legacy
```

## How It Works

```
raioz up
  │
  ├── Load raioz.yaml (or auto-detect)
  ├── Run pre-hook
  ├── Create Docker network (pinned subnet if declared)
  │
  ├── Dependencies (Docker images):
  │   ├── Pull images
  │   ├── Start containers on shared network
  │   ├── Stamp raioz labels for safe teardown
  │   └── Health check with diagnostics
  │
  ├── Services (native runtimes):
  │   ├── Detect runtime from project files
  │   ├── Inject service discovery env vars
  │   ├── Start with native tool (go run, npm dev, etc.)
  │   └── Save PIDs to state file
  │
  ├── Proxy (if enabled):
  │   ├── Ensure mkcert certificates (SAN-validated per domain)
  │   ├── Start Caddy (per-project or workspace-shared)
  │   ├── Persist this project's routes to /tmp/<ws>/proxy/routes/<project>.json
  │   └── https://<service>.<domain> for each service
  │
  ├── File watcher (if watch: true):
  │   ├── Monitor service directories
  │   ├── Debounce file changes (500ms)
  │   └── Auto-restart changed services
  │
  ├── Run post-hook
  └── Stream multiplexed logs (foreground mode)
```

## Zero Lock-in

Delete `raioz.yaml` and your project works exactly the same. Raioz never modifies your files — it creates temporary overlays for networking and throws them away on `raioz down`.

## Requirements

- **OS**: Linux or macOS (amd64 / arm64). Windows via WSL2 or `go install`.
- **Docker** (or Podman / nerdctl) with Compose plugin.
- **Git** — used by `raioz clone` and git-sourced services.
- **Optional**: mkcert (local HTTPS), cloudflared (tunnels).

## Documentation

- [CHANGELOG.md](CHANGELOG.md) — version history.
- [ROADMAP.md](ROADMAP.md) — planned work for upcoming releases.
- [docs/CONFIG_REFERENCE.md](docs/CONFIG_REFERENCE.md) — complete `raioz.yaml` field reference.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — internal design overview.
- [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) — common issues and fixes.

## Development

```bash
make build     # build binary with version ldflags
make test      # run all tests with coverage
make lint      # golangci-lint
make check     # format + lint + i18n sync + tests
make ci        # full CI pipeline (matches GitHub Actions)
```

## License

MIT
