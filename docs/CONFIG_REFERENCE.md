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
| `version` | string | no | — | Schema version this file targets. Currently `"1"`. A warning is emitted when absent. See [Versioning](#versioning). |
| `project` | string | yes | — | Project name. Used for Docker resource naming. Lowercase, hyphens, max 63 chars. |
| `workspace` | string | no | — | Groups projects on same Docker network. When set, resources use `{workspace}-` prefix instead of `raioz-`. |
| `network` | string or object | no | auto-derived | Pin Docker network name and/or subnet. See [Network config](#network-config). |
| `proxy` | bool or object | no | `false` | Enable Caddy reverse proxy with HTTPS. See [Proxy config](#proxy-config). |
| `pre` | string or list | no | — | Commands to run before anything else (env rendering, secrets fetch). Failure aborts `up`. |
| `preUp` | string or list | no | — | Commands to run after dependencies/sibling-spawn are up, but before this project's services start. Use when bootstrap (`make createdb`, schema seeding) needs a workspace dep already reachable. Failure aborts `up`. See [Pre-up vs pre](#pre-up-vs-pre). |
| `post` | string or list | no | — | Commands to run after `raioz up` (e.g., cleanup). |
| `services` | map | no | — | Local services to develop. Keys are service names. See [Service config](#service-config). |
| `dependencies` | map | no | — | Docker images to run. Keys are dependency names. See [Dependency config](#dependency-config). |

---

## Pre-up vs pre

`pre:` and `preUp:` look similar but fire at different phases of
`raioz up`:

```
pre:    →  (env rendering, secrets fetch, local file generation)
           ↓
        infra + sibling-spawn  ←  workspace dependencies come up here
           ↓
preUp:  →  (bootstrap that talks to those deps — createdb, migrate)
           ↓
        this project's services start
           ↓
post:   →  (cleanup)
```

Use `pre:` when the hook only touches the local filesystem (write a
file, decrypt a secret, render a template). Use `preUp:` when the
hook needs the workspace's dependencies — typically a sibling
raioz project's database — to already be reachable.

A common case is `preUp: make createdb` for a service that depends
on a workspace-shared postgres declared as a sibling project (see
[Sibling raioz projects as deps](#sibling-raioz-projects-as-deps)).
The hook runs on the host, so reach the dep via its published host
port (e.g. `PG_HOST=localhost PG_PORT=5432 make createdb`) rather
than the container DNS name (which the host can't resolve).

A `preUp:` failure aborts the run before any service starts; this
is intentional. If the bootstrap can't succeed, the services that
depend on it shouldn't come up.

## Versioning

`version:` declares which raioz.yaml schema your file targets. Today the
only valid value is `"1"`. Declaring it locks the expected shape so future
raioz binaries can either accept it or emit an actionable migration error
instead of silently misinterpreting your config.

```yaml
version: "1"
project: my-app
# ...
```

### Current behavior

| Field state | Behavior |
| ----------- | -------- |
| `version: "1"` | Loads silently. Recommended. |
| Field absent | Loads with a soft warning ("consider adding"). Still works. |
| `version: "2"` (newer) | Loads with a **loud warning**: this binary supports "1", fields introduced in newer schemas are ignored. Update raioz. |
| `version: "0"` (older) | Loads with a **loud warning**: semantics may have changed since this binary's expected version. Run `raioz migrate yaml`. |
| `version: "v1"`, `"1.0"`, `"abc"`, `"-1"` | Loads with a malformed-value warning. Treats the config as if `version: "1"` was declared. |

`raioz init` and `raioz migrate yaml` both write `version: "1"` into newly
generated files.

The warnings are advisory in v0.6 (issue 054 / ADR-031). Future
releases may upgrade specific cases to hard errors:

- **v0.7** is the target for hard-erroring on past-version configs
  (forces a `raioz migrate yaml` before load).
- **v1.0** is the target for hard-erroring on any mismatch.

The escalation lets configs in real-world repos see one minor
release of warning before they become breaking.

### Field evolution policy

Every public field in `raioz.yaml` carries a `// since: vX.Y.Z` marker
next to its declaration in `internal/config/yaml_types.go` (and
`internal/domain/models/config_proxy.go` for the proxy fields). The
marker is enforced by `make check-since` — adding a field without one
fails CI.

- **Adding** a field to the schema does not require bumping the version
  (optional fields are backward compatible). Add the `since:` marker
  AND add a fixture in `internal/config/testdata/configs/` exercising
  the new field (`make check-configs` ensures the schema diff comes
  with a corresponding fixture diff).
- **Removing or changing semantics** of a field requires a version bump
  AND one minor release of grace where the change emits a deprecation
  warning.
- A future major release may **require** `version:` and refuse to load
  configs that don't declare it. The current warning gives the field
  time to land in real-world configs.

#### Inspecting your config

`raioz yaml lint` reports, for every field your `raioz.yaml` populates,
the raioz version that introduced it. Useful when you inherit a config
from a teammate and want to know whether your binary supports it:

```text
$ raioz yaml lint
raioz yaml lint: raioz.yaml
  declared version: 1
  fields in use:    14

  [ok]   project (since v0.1.0)
  [ok]   proxy.publish (since v0.1.0)
  [ok]   dependencies.keycloak.project (since v0.4.0)
  ...
```

When `version:` is missing, every used field is flagged as `[warn]`
with a suggestion to declare it. Add `version: "1"` to silence the
warnings and lock the expected schema.

> Limitation: only fields declared in `internal/config/yaml_types.go`
> are scanned at runtime today; fields in nested types from
> `internal/domain/models/` (ProxyConfig, RoutingConfig) are validated
> at build time by `make check-since` but skipped silently by `raioz
> yaml lint`. Acceptable while those types are stable; revisit if a new
> proxy / routing field needs end-user-visible lint coverage.

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

Exactly one of `image`, `compose`, or `project` is required.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | string | one of | — | Docker image with tag (e.g., `postgres:16`). Mutually exclusive with `compose` and `project`. |
| `compose` | string or list | one of | — | Path(s) to existing docker-compose fragment(s). Use when the dep already has a production-grade compose file (healthchecks, volumes, custom entrypoints). Raioz adds a network + labels overlay; the user's compose controls everything else. Mutually exclusive with `image` and `project`. |
| `project` | string | one of | — | Path to a sibling raioz project's directory. The sibling IS this dep — raioz brings it up via `raioz up` recursively when not already running, and never tumba it on `raioz down`. Mutually exclusive with `image`/`compose`. See [Sibling raioz projects](#sibling-raioz-projects-as-deps). |
| `siblingProject` | string | no | — | Fallback marker: pair with `image:`/`compose:` and raioz skips the local declaration when the sibling project is active. Mutually exclusive with `project`. Useful for CI or contributors without the sibling repo cloned. |
| `requiredHostname` | string | no | — | Assert the sibling's raioz.yaml declares this hostname before deferring to it. Only valid alongside `project:` or `siblingProject:`. |
| `name` | string | no | auto-derived | Literal container name override. Use when external tooling (IDEs, backup scripts) expects a specific name. Without it, the name is `{workspace}-{dep}` (workspace mode) or `{prefix}-{project}-{dep}` (standalone). |
| `ports` | string or list | no | — | Port mappings (e.g., `"5432"`, `"5432:5432"`). |
| `env` | string or list | no | — | Env file paths for the container. |
| `volumes` | string or list | no | — | Volume mounts (e.g., `"pgdata:/var/lib/postgresql/data"`). |
| `hostname` | string | no | dependency name | Custom hostname in Docker network. |
| `routing` | object | no | — | Proxy routing options. Setting this (even to an empty object `{}`) opts the dep into getting an HTTPS route when its image would otherwise be skipped by the DB/broker heuristic (postgres, redis, mysql, etc.). See [Routing config](#routing-config). |
| `dev` | object | no | — | Local development override. See [Dev config](#dev-config). |

### Sibling raioz projects as deps

When a stack spans multiple raioz projects in the same workspace,
declare the relationship explicitly:

```yaml
# Mode A — the sibling IS this dep (no image fallback)
dependencies:
  keycloak:
    project: ../../keycloak     # path to the sibling's raioz.yaml dir
    requiredHostname: sso       # optional, asserted pre-spawn

# Mode B — image fallback when the sibling isn't up
dependencies:
  kafka:
    image: bitnami/kafka:3
    siblingProject: ../../kafka
    requiredHostname: broker    # only checked when deferring to sibling
```

Behavior:

- **`raioz up`** for mode A: reads the sibling's raioz.yaml, runs
  `raioz up` recursively in its dir if not already running, streams
  the output prefixed with `[sibling: <depName>]`. For mode B: when
  the sibling is active, skips the local image and stamps the dep in
  `.raioz.state.json` so `down` matches.
- **`raioz down`** never tumba al hermano. Mode A is identified via
  `project:`; mode B via the deferred-to-sibling stamp from up.
- **Workspace coherence is required** — both sides must declare the
  same `workspace:`. Cross-workspace siblings fail fast with a
  descriptive error before any spawn.
- **Cycles fail fast.** A → B → A is detected via the
  `RAIOZ_SIBLING_STACK` env var threaded through recursive spawns;
  the chain is printed in the error.
- **`requiredHostname:`** is checked against the sibling's declared
  hostnames (`hostname:` on services + `hostnameAliases:`), not the
  live Caddyfile. Skipped when mode B falls back to the image.

#### Sibling lifecycle expectations

After raioz decides to defer to a sibling (mode A active or mode B
with sibling up), the sibling is re-probed once just before consumer
services start. If the sibling was torn down in another terminal in
that window, raioz fails fast with `SIBLING_DOWN` and a suggestion to
bring it back up:

```text
[error] [SIBLING_DOWN] sibling project 'keycloak' for dep 'keycloak' appears to be down ...
  Suggestion: Bring the sibling back up: `cd /path/to/keycloak && raioz up`, then re-run this project's up.
```

raioz does **not** auto-recover by re-spawning the sibling. That would
violate the "down never touches the sibling" invariant (ADR-008) and
hide which project owns the lifecycle. The recovery is explicit.

If the sibling dies *after* the consumer is already running, raioz
won't notice until the next `raioz up` — connections from inside
consumer services will see DNS failures. Re-run `raioz up` on the
sibling and the consumer to recover.

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

### Launcher-pattern container wait

When `command:` is a launcher (a script that detaches a container or
daemon and exits 0 quickly — for example `make dev-docker` invoking
`docker compose up -d --build`), raioz used to report "ready" the
instant the launcher returned. For a first-time build that can leave
"ready" claimed while the container is still being built. ADR-025
fixes this:

- On `raioz up`, if `proxy.target:` points at a container name (not
  `host.docker.internal` or an IP/FQDN), raioz polls Docker for that
  container after the launcher exits. The user sees `Waiting for
  launcher container 'X' to appear (up to <T>)...` and a
  `Launcher container 'X' ready` confirmation. On timeout, raioz
  warns and continues; the run is not aborted.
- On `raioz down`, if the container hasn't appeared yet (build still
  running), raioz waits before invoking `stop:` so the build doesn't
  finish post-stop and leave an orphan.

Tunable via `RAIOZ_LAUNCHER_TIMEOUT` (up-time) and
`RAIOZ_LAUNCHER_DRAIN_TIMEOUT` (down-time) — see [Environment
variables (read by raioz)](#environment-variables-read-by-raioz)
for defaults and the malformed-value behavior.

Skipped automatically when `proxy.target:` is empty or
host-shaped (a dotted name, IP literal, `localhost`, or
`host.docker.internal`) — there's no container to wait for.

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

## Environment variables (read by raioz)

These environment variables override raioz defaults. Most have
sensible defaults — set one only when the default doesn't fit
your setup. `raioz doctor` reports the resolved state of the
duration-typed vars and exits non-zero if any are malformed
(ADR-035 / issue 062).

### State and configuration paths

| Var | Default | Effect |
|-----|---------|--------|
| `RAIOZ_HOME` | (unset) | Override the runtime state root entirely. Takes precedence over XDG. |
| `XDG_STATE_HOME` | `~/.local/state` | Base for `<XDG>/raioz/` (state). Used when `RAIOZ_HOME` is unset (ADR-022). |
| `XDG_CONFIG_HOME` | `~/.config` | Base for `<XDG>/raioz/` (user prefs). |

### Runtime selection

| Var | Default | Effect |
|-----|---------|--------|
| `RAIOZ_RUNTIME` | `docker` | Container runtime: `docker`, `podman`, or `nerdctl`. |
| `RAIOZ_LANG` | system locale (`LC_ALL` → `LANG`) | UI language. Currently `en` / `es`. |

### Logging

| Var | Default | Effect |
|-----|---------|--------|
| `RAIOZ_LOG_LEVEL` | `error` | slog level: `debug`, `info`, `warn`, `error`. |
| `RAIOZ_LOG_JSON` | `false` (auto-`true` in CI) | Emit structured JSON logs instead of text. CI detection looks at `CI`, `GITHUB_ACTIONS`, `GITLAB_CI`, `JENKINS_URL`, `TRAVIS`, `CIRCLECI`, `CONTINUOUS_INTEGRATION`. |

### Launcher pattern (ADR-025)

Go duration strings (e.g. `90s`, `2m`). `0s` opts out. Malformed
values warn once and fall back to the default — surfaced by
`raioz doctor` (ADR-035).

| Var | Default | Effect |
|-----|---------|--------|
| `RAIOZ_LAUNCHER_TIMEOUT` | `60s` | Up-time wait for the launcher's container to appear. |
| `RAIOZ_LAUNCHER_DRAIN_TIMEOUT` | `30s` | Down-time wait for an in-progress build to produce the container before invoking `stop:`. |

Adding a new duration env var? Append it to
`host.KnownDurationEnvs()` so `raioz doctor` picks it up — same
contract as the table above (ADR-035).

### Internal (don't set manually)

These are managed by raioz itself; don't override them.

| Var | Use |
|-----|-----|
| `RAIOZ_SIBLING_STACK` | Recursion cycle detection in mode A sibling spawn (ADR-008). |
| `RAIOZ_CORRELATION_ID` | Request/correlation ID propagated across recursive `raioz up` so audit logs from sibling spawns share an ID. |

---

## Environment variables (injected by raioz)

Raioz injects service discovery env vars automatically:

| Scenario | Example |
|----------|---------|
| Host → container | `POSTGRES_HOST=localhost` |
| Container → container | `POSTGRES_HOST=postgres` |
| With proxy | `API_URL=https://api.acme.localhost` |
