# Raioz

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

## Quick Start

### Install

```bash
# From source
go install github.com/ingeniomaps/raioz/cmd/raioz@latest

# Or build locally
git clone https://github.com/ingeniomaps/raioz.git
cd raioz && make install
```

### Option A: Zero-config (just run it)

```bash
cd your-project/
raioz up          # auto-detects services and dependencies
```

Raioz scans subdirectories for project files (go.mod, package.json, Dockerfile, etc.) and reads `.env` files to infer infrastructure needs (`DATABASE_URL=postgres://...` adds postgres automatically).

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

### Three modes of operation

```bash
raioz up                # detach: start everything and exit
raioz up --attach       # foreground: stream all logs, Ctrl+C to stop
raioz up                # with watch: true — logs + auto-restart on file changes
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

- Detects services from subdirectory structure
- Reads `.env` files to find infrastructure needs
- Auto-detects `.env.postgres` files and wires them to dependencies
- Wires `dependsOn` automatically

### Workspace naming

```yaml
workspace: acme          # optional
project: e-commerce
```

With workspace set, Docker resources use it as prefix instead of "raioz":

```
acme-e-commerce-api        # container
acme-e-commerce-postgres   # container
acme-net                   # network
```

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

### More features

```bash
raioz list                    # all active projects across workspaces
raioz graph                   # visualize dependency graph (ASCII/DOT/JSON)
raioz dev postgres ./local-pg # hot-swap dependency to local code
raioz snapshot create backup  # backup Docker volumes
raioz tunnel api              # expose to internet via cloudflared
raioz doctor                  # check system requirements
raioz clean                   # remove stopped containers
```

## Config Reference

```yaml
# raioz.yaml — full reference
workspace: acme-corp          # optional: prefix for Docker resources
project: e-commerce           # required: project name
proxy:                        # optional: enables Caddy + HTTPS
  domain: acme.localhost      # custom domain (default: localhost)

pre: ./scripts/fetch-secrets.sh   # run before starting
post: rm -f .env.*.tmp            # run after starting

services:
  api:
    path: ./api                   # local directory (auto-detected runtime)
    dependsOn: [postgres, redis]  # start these first
    watch: true                   # restart on file changes
    health: /api/health           # health check endpoint
    ports: ["3000"]               # port mappings
    env: .env.api                 # service-specific env file
    hostname: api                 # custom proxy hostname
    routing:
      ws: true                    # WebSocket support
      stream: true                # SSE/streaming
      grpc: true                  # gRPC (h2c)

  frontend:
    path: ./frontend
    watch: native                 # has its own hot-reload

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres

  redis:
    image: redis:7
```

## How It Works

```
raioz up
  │
  ├── Load raioz.yaml (or auto-detect)
  ├── Run pre-hook
  ├── Create Docker network
  │
  ├── Dependencies (Docker images):
  │   ├── Pull images
  │   ├── Start containers on shared network
  │   └── Health check with diagnostics
  │
  ├── Services (native runtimes):
  │   ├── Detect runtime from project files
  │   ├── Inject service discovery env vars
  │   ├── Start with native tool (go run, npm dev, etc.)
  │   └── Save PIDs to state file
  │
  ├── Proxy (if enabled):
  │   ├── Generate mkcert certificates
  │   ├── Start Caddy with per-service routes
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

- Docker (or Podman/nerdctl) + Compose plugin
- Optional: mkcert (for local HTTPS), cloudflared (for tunnels)

## Development

```bash
make build     # build binary
make test      # run all tests
make lint      # golangci-lint
make check     # format + lint + i18n + tests
make ci        # full CI pipeline
```

## License

MIT
