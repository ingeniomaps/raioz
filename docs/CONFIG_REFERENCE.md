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
| `network` | string or object | no | auto-derived | Pin Docker network name and/or subnet. See [Network config](#network-config). |
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
| `proxy` | object | no | — | Override proxy target/port when detection can't see the service (e.g., `command:` launches its own compose stack). See [Service proxy override](#service-proxy-override). |
| `command` | string | no | — | User-supplied launch command. Overrides runtime auto-detection. |
| `stop` | string | no | — | User-supplied stop command, paired with `command`. Falls back to SIGTERM on the PID if absent. |
| `profiles` | string or list | no | — | Profile tags for selective startup (`raioz up --profile X`). |
| `git` | string | no | — | Git repository URL. Raioz clones it to `path`. |
| `branch` | string | no | — | Git branch to checkout. Used with `git`. |

---

## Dependency config

Dependencies are Docker images you need running (databases, caches, queues).

Exactly one of `image` or `compose` is required.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | string | one of | — | Docker image with tag (e.g., `postgres:16`). Mutually exclusive with `compose`. |
| `compose` | string or list | one of | — | Path(s) to existing docker-compose fragment(s). Use when the dep already has a production-grade compose file (healthchecks, volumes, custom entrypoints). Raioz adds a network + labels overlay; the user's compose controls everything else. Mutually exclusive with `image`. |
| `name` | string | no | auto-derived | Literal container name override. Use when external tooling (IDEs, backup scripts) expects a specific name. Without it, the name is `{workspace}-{dep}` (workspace mode) or `{prefix}-{project}-{dep}` (standalone). |
| `ports` | string or list | no | — | Port mappings (e.g., `"5432"`, `"5432:5432"`). |
| `env` | string or list | no | — | Env file paths for the container. |
| `volumes` | string or list | no | — | Volume mounts (e.g., `"pgdata:/var/lib/postgresql/data"`). |
| `hostname` | string | no | dependency name | Custom hostname in Docker network. |
| `routing` | object | no | — | Proxy routing options. Setting this (even to an empty object `{}`) opts the dep into getting an HTTPS route when its image would otherwise be skipped by the DB/broker heuristic (postgres, redis, mysql, etc.). See [Routing config](#routing-config). |
| `dev` | object | no | — | Local development override. See [Dev config](#dev-config). |

When neither `ports`, `expose`, nor `proxy.port` resolve a target port,
raioz reads the image's manifest via `docker image inspect` and uses
the lowest TCP port from `Config.ExposedPorts`. Most official images
declare `EXPOSE` (postgres → 5432, pgadmin4 → 80, redisinsight → 5540),
so explicit `ports:`/`expose:` is only needed for non-standard
deployments. Lookup runs after deps start (image is local by then) and
caches per `image:tag` for the process lifetime.

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
  ip: 172.28.1.1            # pin proxy container IP inside the Docker network
  publish: false            # skip binding host 80/443 (multi-workspace mode)
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `domain` | string | `localhost` | Base domain. Services get `https://{name}.{domain}`. |
| `mode` | string | `subdomain` | Routing mode: `subdomain` or `path`. |
| `tls` | string | `mkcert` | TLS provider: `mkcert` (local) or `letsencrypt`. |
| `ip` | string | `<subnet>.1.1` when `network.subnet` is set, else Docker-assigned | Pin the Caddy container's IP. Deterministic so scripts and `/etc/hosts` entries stay stable. Requires `network.subnet` — Docker won't honor `--ip` without a user-defined subnet. |
| `publish` | bool | `true` | Bind host ports 80/443 (default). Set `false` to reach the proxy only via its container IP — lets multiple workspaces run in parallel without port contention. Requires a deterministic `ip` or `network.subnet`. **Linux-only**; on macOS/Windows, Docker routes through a VM whose bridge IPs aren't reachable from the host. |

Result: each service gets `https://{service}.{domain}` (e.g., `https://api.acme.localhost`).

When `publish: false`, use `raioz hosts` to print the `/etc/hosts` line mapping every proxied hostname to the proxy container IP.

---

## Network config

Pin the Docker network raioz manages. Omit to let raioz derive the
name (`{workspace}-net` or `{project}-net`) and let Docker pick any
subnet.

### String shorthand

```yaml
network: my-existing-net
```

### Full object

```yaml
network:
  name: acme-net              # override the derived network name
  subnet: 172.28.0.0/16       # pin subnet so containers get deterministic IPs
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `{workspace}-net` or `{project}-net` | Network name. Useful when joining an external network managed outside raioz. |
| `subnet` | string (CIDR) | Docker auto-assigned | Explicit subnet. Required when using `proxy.ip` or `proxy.publish: false` so container IPs stay deterministic across `up`/`down` cycles. |

---

## Service proxy override

Tell the proxy exactly where to forward traffic for a service, bypassing
auto-detection. Populated from `services.<n>.proxy:` in `raioz.yaml`.

```yaml
services:
  keycloak:
    path: ./keycloak
    command: make start       # launches its own compose stack
    stop: make stop
    proxy:
      target: hypixo-keycloak  # container name on the shared network
      port: 8080
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `target` | string | auto-detected | DNS name or IP the proxy should `reverse_proxy` to. Use a container name on the shared network, or `host.docker.internal` to reach a host-running service. |
| `port` | int | auto-detected | Port on `target` to dial. |

Needed when `command:` launches a compose stack whose containers raioz
can't introspect. Without the override, raioz classifies the service as
"host" and the proxy ends up pointing at `host.docker.internal` with no
port.

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

Service containers use a per-project name; dependencies in a workspace
are shared across sibling projects (so sibling projects reuse the same
postgres container instead of one per project).

| Resource | With workspace | Without workspace |
|----------|----------------|-------------------|
| Service container | `{workspace}-{project}-{service}` | `raioz-{project}-{service}` |
| Dependency container | `{workspace}-{dep}` (shared) | `raioz-{project}-{dep}` |
| Network | `{workspace}-net` | `raioz-{project}-net` |
| Proxy container | `{workspace}-proxy` (shared) | `raioz-proxy-{project}` |

Shared dependencies survive individual `raioz down` calls while any
sibling project still runs in the workspace. The last project to leave
the workspace tears them down.

Override the dependency container name with `dependencies.<n>.name`
when external tooling expects a specific literal name.

### Labels

Every raioz-managed container carries these labels so `raioz down` can
sweep by label instead of name prefix:

| Label | Value |
|-------|-------|
| `com.raioz.managed` | `true` |
| `com.raioz.kind` | `service` \| `dependency` \| `proxy` |
| `com.raioz.workspace` | workspace name (when set) |
| `com.raioz.project` | project name (**omitted** on shared deps — that's the signal they outlive any single project) |
| `com.raioz.service` | service / dep / "proxy" name |

---

## Environment variable injection

Raioz injects service discovery env vars automatically:

| Scenario | Example |
|----------|---------|
| Host → container | `POSTGRES_HOST=localhost` |
| Container → container | `POSTGRES_HOST=postgres` |
| With proxy | `API_URL=https://api.acme.localhost` |
