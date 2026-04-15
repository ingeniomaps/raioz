# Config Reference — raioz.yaml

Complete reference for the `raioz.yaml` configuration file.

## Minimal example

```yaml
project: my-app

services:
  api:
    path: ./api
```

## Full example

```yaml
workspace: acme-corp
project: e-commerce
proxy:
  domain: acme.localhost

pre: ./scripts/fetch-secrets.sh
post: rm -f .env.*.tmp

services:
  api:
    path: ./api
    dependsOn: [postgres, redis]
    watch: true
    health: /api/health
    ports: ["8080"]
    env: .env.api
    hostname: api
    routing:
      ws: true
      stream: true
      grpc: true

  frontend:
    path: ./frontend
    watch: native
    profiles: [frontend]

  auth:
    git: git@github.com:org/auth-service.git
    branch: main
    path: ./auth

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres
    volumes: ["pgdata:/var/lib/postgresql/data"]

  redis:
    image: redis:7
```

---

## Top-level fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `project` | string | yes | — | Project name. Used for Docker resource naming. Lowercase, hyphens, max 63 chars. |
| `workspace` | string | no | — | Groups projects on same Docker network. When set, resources use `{workspace}-` prefix instead of `raioz-`. |
| `proxy` | bool or object | no | `false` | Enable Caddy reverse proxy with HTTPS. See [Proxy config](#proxy-config). |
| `pre` | string or list | no | — | Commands to run before `raioz up` (e.g., fetch secrets). |
| `post` | string or list | no | — | Commands to run after `raioz up` (e.g., cleanup). |
| `services` | map | no | — | Local services to develop. Keys are service names. See [Service config](#service-config). |
| `dependencies` | map | no | — | Docker images to run. Keys are dependency names. See [Dependency config](#dependency-config). |

---

## Service config

Services are local code you're developing. Raioz detects the runtime
and starts with the native tool (go run, npm dev, etc.).

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | yes | — | Relative path to service directory. |
| `dependsOn` | string or list | no | — | Dependencies or services that must start first. |
| `env` | string or list | no | — | Env file paths (relative to project root). |
| `ports` | string or list | no | auto-detected | Port mappings (e.g., `"3000"`, `"3000:8080"`). |
| `watch` | bool or string | no | `false` | File watching mode. See [Watch config](#watch-config). |
| `health` | string | no | — | Health check endpoint path (e.g., `/api/health`). |
| `hostname` | string | no | service name | Custom hostname for proxy routing. |
| `routing` | object | no | — | Proxy routing options. See [Routing config](#routing-config). |
| `profiles` | string or list | no | — | Profile tags for selective startup (`raioz up --profile X`). |
| `git` | string | no | — | Git repository URL. Raioz clones it to `path`. |
| `branch` | string | no | — | Git branch to checkout. Used with `git`. |

---

## Dependency config

Dependencies are Docker images you need running (databases, caches, queues).

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | string | yes | — | Docker image with tag (e.g., `postgres:16`). |
| `ports` | string or list | no | — | Port mappings (e.g., `"5432"`, `"5432:5432"`). |
| `env` | string or list | no | — | Env file paths for the container. |
| `volumes` | string or list | no | — | Volume mounts (e.g., `"pgdata:/var/lib/postgresql/data"`). |
| `hostname` | string | no | dependency name | Custom hostname in Docker network. |
| `routing` | object | no | — | Proxy routing options. See [Routing config](#routing-config). |
| `dev` | object | no | — | Local development override. See [Dev config](#dev-config). |

---

## Proxy config

The `proxy` field can be a simple boolean or a detailed object.

### Simple (boolean)

```yaml
proxy: true
```

Enables Caddy with defaults: subdomain mode, localhost domain, mkcert TLS.

### Detailed (object)

```yaml
proxy:
  domain: acme.localhost    # custom domain
  mode: subdomain           # "subdomain" (default) or "path"
  tls: mkcert               # "mkcert" (default) or "letsencrypt"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `domain` | string | `localhost` | Base domain. Services get `https://{name}.{domain}`. |
| `mode` | string | `subdomain` | Routing mode: `subdomain` or `path`. |
| `tls` | string | `mkcert` | TLS provider: `mkcert` (local) or `letsencrypt`. |

Result: each service gets `https://{service}.{domain}` (e.g., `https://api.acme.localhost`).

---

## Routing config

Per-service proxy routing behavior. Only relevant when `proxy` is enabled.

```yaml
routing:
  ws: true       # WebSocket support
  stream: true   # SSE / streaming responses
  grpc: true     # gRPC (h2c proxying)
  tunnel: true   # Expose via tunnel (cloudflared/bore)
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ws` | bool | `false` | Enable WebSocket proxying. |
| `stream` | bool | `false` | Enable SSE / streaming response support. |
| `grpc` | bool | `false` | Enable gRPC (HTTP/2 cleartext) proxying. |
| `tunnel` | bool | `false` | Mark service for tunnel exposure. |

---

## Watch config

The `watch` field controls file watching and auto-restart.

| Value | Behavior |
|-------|----------|
| `false` (default) | No watching. Service runs once. |
| `true` | Raioz watches files and restarts the service on changes (debounced 500ms). |
| `"native"` | Service has its own hot-reload (e.g., Next.js, Vite). Raioz streams logs but doesn't restart. |

---

## Dev config

Allows promoting a dependency to local development code.

```yaml
dependencies:
  auth:
    image: org/auth:latest
    dev:
      path: ./local-auth    # local directory with source code
```

Activate with: `raioz dev auth ./local-auth`
Revert with: `raioz dev --reset auth`

---

## Pre/post hooks

Run shell commands before or after starting services.

```yaml
# Single command
pre: ./scripts/fetch-secrets.sh
post: rm -f .env.*.tmp

# Multiple commands
pre:
  - ./scripts/fetch-secrets.sh
  - ./scripts/generate-certs.sh
post:
  - rm -f .env.*.tmp
  - echo "Ready"
```

---

## Zero-config mode

If no `raioz.yaml` exists, `raioz up` auto-detects:
- **Services** from subdirectories (by project files like go.mod, package.json, etc.)
- **Dependencies** from `.env` files (DATABASE_URL → postgres, REDIS_URL → redis)
- **dependsOn** wiring from which service needs which dependency

Generate a config from auto-detection: `raioz init`

---

## Docker resource naming

| With workspace | Without workspace |
|----------------|-------------------|
| `{workspace}-{project}-{service}` | `raioz-{project}-{service}` |
| `{workspace}-net` | `raioz-{project}-net` |

---

## Environment variable injection

Raioz injects service discovery env vars automatically:

| Scenario | Example |
|----------|---------|
| Host → container | `POSTGRES_HOST=localhost` |
| Container → container | `POSTGRES_HOST=postgres` |
| With proxy | `API_URL=https://api.acme.localhost` |
