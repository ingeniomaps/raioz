# Raioz

**One command to run your entire architecture.**

Raioz is a meta-orchestrator for local development. It doesn't replace your tools — it connects them. Bring your Docker Compose files, Dockerfiles, npm projects, Go services, Makefiles — Raioz detects what you have and runs everything together with shared networking, HTTPS, and automatic service discovery.

```bash
raioz init     # scans your project, generates config
raioz up       # everything running, connected, HTTPS ready
```

## Why Raioz?

**The problem:** You have 5 microservices. One uses Docker Compose, another is a Go binary, the frontend is npm, and you need Postgres and Redis. Getting them all to talk to each other locally is a mess of ports, env vars, and manual Docker commands.

**The solution:** Raioz reads each service's native config (compose, Dockerfile, package.json, go.mod), starts them all, puts them on the same network, and injects the right environment variables so they find each other automatically.

```
raioz up

  DEPENDENCIES
  postgres     image     ready    :5432
  redis        image     ready    :6379

  SERVICES
  api          go/host   ready    https://api.localhost
  frontend     npm/host  ready    https://frontend.localhost
  worker       docker    ready    https://worker.localhost
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

### Option A: Auto-scan (recommended)

```bash
cd your-project/
raioz init        # scans directories, detects services, infers dependencies from .env files
raioz up          # done
```

### Option B: Write config manually

Create `raioz.yaml`:

```yaml
project: my-app

services:
  api:
    path: ./api
    dependsOn: [postgres, redis]

  frontend:
    path: ./frontend
    dependsOn: [api]

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]

  redis:
    image: redis:7
```

```bash
raioz up
```

That's it. Raioz detects that `./api` has a `go.mod` (runs with `go run .`), `./frontend` has a `package.json` (runs with `npm run dev`), and postgres/redis are Docker images. Everything shares a network and can reach each other.

## Key Features

### Zero learning curve

Raioz reads what you already have. Your docker-compose.yml, your Dockerfile, your package.json — they stay exactly as they are. Raioz just connects them.

### Automatic service discovery

Each service gets environment variables with the correct hosts for its dependencies:

```bash
# Inside a Docker container calling another container:
POSTGRES_HOST=postgres          # Docker DNS name
REDIS_URL=http://redis:6379

# Inside a host process (npm/go) calling a Docker container:
POSTGRES_HOST=localhost          # Port mapped to host
REDIS_URL=http://localhost:6379

# With proxy enabled:
API_HTTPS_URL=https://api.localhost
```

Raioz calculates the right host automatically based on where each service runs.

### HTTPS everywhere with Caddy proxy

```yaml
project: my-app
proxy: true       # that's it
```

Every service gets `https://<name>.localhost` — works from the browser AND from inside Docker containers. No port conflicts, no certificate errors. Uses Caddy + mkcert under the hood.

### Hot-swap dependencies

```bash
# Postgres runs as Docker image by default
raioz up
# ✓ postgres (image: postgres:16) :5432

# Need to modify postgres config? Clone it locally:
git clone git@github.com:org/custom-pg.git ./local-pg
raioz dev postgres ./local-pg
# ✓ postgres: image → local (./local-pg)
# All services still see it as "postgres" — zero config change

# Done? Go back to the image:
raioz dev --reset postgres
```

### Smart init

`raioz init` scans your project and infers everything:

- Detects services from subdirectory structure (Dockerfile, package.json, go.mod, Makefile)
- Reads `.env` files to find infrastructure needs (`DATABASE_URL=postgres://...` → adds postgres dependency)
- Wires `dependsOn` automatically based on what each service references
- Reads existing `docker-compose.yml` to extract infra services

### Interactive dashboard

```bash
raioz dashboard   # or: raioz tui
```

Live terminal UI showing all services, their status, CPU/memory, proxy URLs, and streaming logs. Navigate with keyboard, restart/stop services, open shells — all from one terminal.

### File watching

```yaml
services:
  api:
    path: ./api
    watch: true       # Raioz restarts on file changes

  frontend:
    path: ./frontend
    watch: native     # Next.js/Vite has its own hot-reload, Raioz stays out
```

### Dependency graph

```bash
raioz graph
#   frontend ──> api ──> postgres, redis
#   worker ──> postgres
#
#   Dependencies:
#     [postgres]
#     [redis]

raioz graph --format dot | dot -Tpng -o architecture.png
```

### Volume snapshots

```bash
raioz snapshot create before-migration
# run your migration...
# something broke?
raioz snapshot restore before-migration
```

### Tunnel to internet

```bash
raioz tunnel api
# ✓ Tunnel active: https://abc123.trycloudflare.com → localhost:3000
# Share with teammates, test webhooks, mobile development
```

## All Commands

| Command | Description |
|---------|-------------|
| `raioz up` | Start everything |
| `raioz down` | Stop everything |
| `raioz status` | Show service status, runtime, URLs |
| `raioz logs [service]` | View logs (supports --follow) |
| `raioz restart [service]` | Restart services |
| `raioz exec <service> [cmd]` | Run command in container / service dir |
| `raioz dashboard` | Interactive TUI (alias: `tui`) |
| `raioz dev <dep> <path>` | Hot-swap dependency to local |
| `raioz graph` | Visualize dependencies (ASCII/DOT/JSON) |
| `raioz snapshot` | Backup/restore Docker volumes |
| `raioz tunnel <service>` | Expose service to internet |
| `raioz proxy status` | Manage Caddy reverse proxy |
| `raioz init` | Auto-scan and generate raioz.yaml |
| `raioz doctor` | Check system requirements |
| `raioz check` | Validate config |
| `raioz clean` | Remove stopped containers/volumes |
| `raioz migrate yaml` | Convert .raioz.json to raioz.yaml |

## Config Reference

```yaml
# raioz.yaml — full reference
workspace: acme-corp          # optional: groups projects on same network
project: e-commerce           # required: project name
proxy: true                   # optional: enables Caddy + HTTPS

pre: ./scripts/fetch-secrets.sh   # run before starting (secrets, env)
post: rm -f .env.*.tmp            # run after starting (cleanup)

services:
  api:
    path: ./api                   # local directory (auto-detected runtime)
    dependsOn: [postgres, redis]  # start these first
    health: /api/health           # health check endpoint
    watch: true                   # restart on file changes
    ports: ["3000"]               # port mappings
    env: .env.api                 # service-specific env file
    hostname: api                 # custom proxy hostname
    routing:
      ws: true                    # WebSocket support
      stream: true                # SSE/streaming (disable buffering)
      grpc: true                  # gRPC (h2c protocol)
      tunnel: true                # auto-expose via cloudflared

  frontend:
    path: ./frontend
    watch: native                 # has its own hot-reload (Vite, Next.js)

  auth-service:
    git: git@github.com:org/auth.git   # clone from git
    branch: develop

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres            # env file for this dependency

  redis:
    image: redis:7
```

## How It Works

```
raioz up
  │
  ├── Load raioz.yaml
  ├── Run pre-hook (fetch secrets, etc.)
  ├── Create Docker network
  │
  ├── For each dependency:
  │   └── docker pull + run on shared network
  │
  ├── For each service:
  │   ├── Detect runtime (compose? Dockerfile? npm? go? make?)
  │   ├── Inject service discovery env vars
  │   └── Start with native tool:
  │       ├── docker compose -f <theirs> up (for compose projects)
  │       ├── docker build + run (for Dockerfiles)
  │       ├── npm run dev (for Node.js)
  │       ├── go run . (for Go)
  │       └── make dev (for Makefiles)
  │
  ├── Start Caddy proxy (if proxy: true)
  │   └── https://<service>.localhost for each service
  │
  ├── Run post-hook (cleanup)
  └── Save state
```

## Zero Lock-in

Delete `raioz.yaml` and your project works exactly the same. Raioz never modifies your files — it creates temporary overlays for networking and throws them away on `raioz down`.

## Requirements

- Go 1.22+
- Docker + Docker Compose
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
