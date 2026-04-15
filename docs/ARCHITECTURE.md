# Architecture

How Raioz works internally. For contributors and curious users.

## Design principles

1. **Don't replace, complement** — Raioz never modifies user files. It creates temporary overlays and throws them away.
2. **Native tools first** — Go projects run with `go run`, npm projects with `npm dev`. Docker only for dependencies.
3. **Zero lock-in** — Delete `raioz.yaml` and your project works the same.
4. **Detect, don't configure** — Prefer auto-detection over manual config.

## Flow: `raioz up`

```
raioz up
  │
  ├─ Load config
  │   ├─ raioz.yaml exists? → parse it
  │   └─ no config? → auto-detect (scan dirs + .env files)
  │
  ├─ Run pre-hooks
  │
  ├─ Create Docker network
  │   └─ {workspace}-net or raioz-{project}-net
  │
  ├─ Start dependencies (Docker images)
  │   ├─ Pull images if needed
  │   ├─ Generate minimal compose overlay per dependency
  │   ├─ Start containers on shared network
  │   └─ Health check with diagnostics
  │
  ├─ Detect service runtimes
  │   └─ For each service path: detect.Detect(path)
  │       → RuntimeGo, RuntimeNPM, RuntimeCompose, etc.
  │
  ├─ Start services (native runtimes)
  │   ├─ Inject service discovery env vars
  │   ├─ Dispatch to correct runner via orchestrate.Dispatcher
  │   │   ├─ HostRunner: go run, npm dev, cargo run, etc.
  │   │   ├─ ComposeRunner: docker compose with overlay
  │   │   └─ DockerfileRunner: docker build + run
  │   └─ Track PIDs in state file
  │
  ├─ Start proxy (if enabled)
  │   ├─ Generate mkcert certificates
  │   ├─ Generate Caddyfile with per-service routes
  │   └─ Start Caddy container on shared network
  │
  ├─ Start file watchers (if watch: true)
  │   └─ fsnotify with 500ms debounce → restart service
  │
  ├─ Run post-hooks
  │
  └─ Stream multiplexed logs (foreground mode)
```

## Layer map

```
cmd/raioz/main.go              Entry point (7 lines)
         │
internal/cli/                   Cobra commands (28 registered; 30 including
         │                      subcommands like `snapshot create/restore/…`)
         │                      Thin: parse flags → create deps → call use case
         │
internal/app/                   Use cases
         │                      Options + Execute() pattern
         │                      DI via *Dependencies struct
         │                      Only uses domain interfaces
         │
internal/domain/                Contracts
  interfaces/                   19 port interfaces
  models/                       Type aliases for decoupling
         │
internal/infra/                 Adapters
  docker/config/git/...         Implement domain interfaces
         │                      Delegate to concrete packages
         │
internal/                       Concrete packages
  detect/                       Runtime detection (24 runtimes)
  orchestrate/                  Dispatcher + runners
  proxy/                        Caddy management (per-project + workspace-shared)
  discovery/                    Service discovery env vars
  watch/                        File watcher
  naming/                       Docker resource naming + container labels
  runtime/                      Container runtime abstraction
  config/                       Config loading (YAML + JSON)
  docker/                       Docker operations
  state/                        State persistence
  env/                          Environment variable resolution
  errors/                       Structured errors
  i18n/                         Internationalization (en/es)
  tui/                          Interactive dashboard
  graph/                        Dependency visualization
  snapshot/                     Volume backup/restore
  tunnel/                       Tunnel exposure
  validate/                     Preflight and invariant validators
  resilience/                   Retry + circuit breakers
  notify/                       Desktop notifications
  audit/                        Audit log of user-facing operations
  logging/                      Structured logging with context
  output/                       Terminal output formatting (progress, tables)
  path/                         Path validation and normalization
  ignore/                       .raiozignore parsing
  production/                   Legacy migration helpers
  git/ host/ lock/ mocks/       Git, host processes, locks, test mocks
```

## Key packages

### detect

Scans a directory and returns a `DetectResult` with runtime, start
command, port, and notable files. Detection priority:

```
compose > Dockerfile > package.json > go.mod > Makefile >
pyproject.toml > Cargo.toml > composer.json > pom.xml >
build.gradle > *.csproj > Gemfile > mix.exs > ...
```

Package manager detected from lock files: `yarn.lock` → yarn,
`pnpm-lock.yaml` → pnpm, `bun.lockb` → bun.

### orchestrate

Dispatcher routes to the correct runner based on runtime:

| Runner | Runtimes | How |
|--------|----------|-----|
| `ComposeRunner` | compose | `docker compose -f original -f overlay up` |
| `DockerfileRunner` | dockerfile | `docker build` + `docker run` |
| `HostRunner` | npm, go, python, rust, php, java, ... (20 runtimes) | Native command, PID tracked |
| `ImageRunner` | image | Generate minimal compose, `docker compose up` |

### proxy

Manages Caddy as a Docker container on the shared network:
- Generates Caddyfile with per-service reverse proxy routes
- Uses mkcert for local TLS certificates
- Supports WebSocket, SSE, and gRPC routing
- DNS aliases in Docker network for container-to-container

### discovery

Generates environment variables for cross-runtime communication:

| Caller | Target | Host value |
|--------|--------|------------|
| Host process | Container | `localhost` |
| Container | Container | service name (Docker DNS) |
| Container | Host process | `host.docker.internal` |
| Host process | Host process | `localhost` |

### naming

Centralized naming for all Docker resources. Service containers are
per-project, but dependencies and proxy are workspace-shared when a
workspace is declared so sibling projects reuse them.

```
                        With workspace              Without workspace
Service container       {workspace}-{project}-{svc} raioz-{project}-{svc}
Dependency container    {workspace}-{dep}  (shared) raioz-{project}-{dep}
Network                 {workspace}-net             raioz-{project}-net
Proxy container         {workspace}-proxy  (shared) raioz-proxy-{project}
Caddy volume            {workspace}-caddy  (shared) raioz-caddy-{project}
```

Every raioz-managed container is stamped with labels
(`com.raioz.managed=true`, `com.raioz.kind`, `com.raioz.workspace`,
`com.raioz.project`, `com.raioz.service`). `raioz down` sweeps by label
— not by name prefix — so it can't take down containers owned by an
unrelated project that happens to share a prefix.

Shared dependencies intentionally **omit** `com.raioz.project` so a
`raioz down` on any one project doesn't tumba them; the last project
leaving the workspace does. See `internal/naming/labels.go`.

### host

Cross-platform host process management. `SetNewProcessGroup`,
`KillProcessTree`, `ForceKillProcessTree`, and `IsProcessAlive`
abstract the difference between Unix process groups
(`syscall.Setpgid` + signal to negative PID) and Windows tree kill
(`taskkill /T` + `tasklist`). Used by `host_runner.Stop`,
`host_lifecycle.killProcessGraceful`, and `down.killProcessGroup`
so all three sites share the same logic. Implementations live in
`proctree_unix.go` / `proctree_windows.go` behind build tags.

### state

Persists minimal state to `.raioz.state.json` in the project directory.
Only stores what Docker can't tell us:
- Dev overrides (which dependencies are swapped to local)
- Host PIDs (for process cleanup)
- Ignored services

Docker is the source of truth for container running state.

## Dependency injection

Use cases in `internal/app/` receive a `*Dependencies` struct:

```go
type Dependencies struct {
    Docker    interfaces.DockerRunner
    Git       interfaces.GitRepository
    State     interfaces.StateManager
    Config    interfaces.ConfigLoader
    Workspace interfaces.WorkspaceManager
    Host      interfaces.HostRunner
    Env       interfaces.EnvManager
    Lock      interfaces.LockManager
    Validator interfaces.Validator
}
```

CLI layer creates concrete implementations and injects them.
App layer only sees interfaces. This enables testing with mocks.

## Config bridge

`raioz.yaml` (RaiozConfig) is converted to the internal `Deps` struct
via a bridge layer. This keeps the user-facing config minimal while
the internal representation stays rich for backward compatibility
with `.raioz.json`.

## Architectural invariants

Rules that shipped in v0.1.0 and must be preserved. Breaking any of
them has caused real regressions in the past; each line below
corresponds to a bug that already happened once.

### 1. Identity is labels, not names
Every raioz-managed container is stamped with `com.raioz.managed=true`
plus kind/workspace/project/service labels. `raioz down` filters by
those labels. Any new runner (or wrapper) MUST stamp the full set — a
container without `com.raioz.managed=true` is invisible to teardown
and will leak. See `internal/naming/labels.go` and
`internal/orchestrate/image_runner.go`.

### 2. Shared deps omit `com.raioz.project`
Workspace-shared dependencies (`dependencies.<n>.name` set OR
`workspace:` set) are named `{workspace}-{dep}` and intentionally
omit the project label. That omission IS the signal: when `raioz
down` is invoked on one project, it skips any container without
`com.raioz.project=<me>`, so sibling projects keep running. The last
project leaving the workspace tears them down via
`otherProjectsActiveInWorkspace`. No ref-count state file.

`ImageRunner.Start` is idempotent for this reason: if a sibling
project already started `{workspace}-postgres`, `raioz up` in the
current project sees it and returns without re-creating.

### 3. Certs are per-domain with SAN validation
Certificates live in `~/.raioz/certs/<domain>/`. `EnsureCerts(domain)`
validates the existing SAN includes BOTH `<domain>` AND `*.<domain>`
before reusing. Without this, a cert issued for `acme.localhost`
could be reused for `hypixo.dev` — broken HTTPS with no obvious cause.

### 4. `auto_https off` for mkcert
The Caddyfile global block uses `auto_https off` when `tls: mkcert`
is active. Caddy's default behavior falls back to ACME for custom
domains without public DNS — which hangs forever. Do not replace this
with `disable_redirects` alone; that only silences the HTTP→HTTPS
redirect, not the ACME fallback.

### 5. Workspace-shared proxy routes are per-project
When `workspace:` is set, a single `{workspace}-proxy` Caddy fronts
every project in the workspace. Each project's routes are persisted
at `/tmp/<workspace>/proxy/routes/<project>.json`; the shared
Caddyfile is the union of every project's file. `raioz down` removes
only the current project's routes and reloads Caddy. Only the last
project leaving the workspace tumba the proxy.

`Reload` must NOT use `docker cp` to push the Caddyfile (the bind
mount is read-only and `cp` fails with "device or resource busy"). It
writes to the host path; the bind mount propagates the change into
the container.

### 6. Clone functions must stay in sync with config structs
`internal/app/upcase/workspace_project_conflict.go` has
`cloneService` and `cloneInfraEntry` used by the workspace-merge
path. Adding a field to `config.Service` or `config.Infra` that
affects orchestration REQUIRES updating these clones — missing
fields silently vanish on re-up after any workspace state mismatch.
Every past regression in this area traced to a missing field.

### 7. Image classification is shared and bare-name-matched
`proxy.IsNonHTTPImage(image)` in `internal/proxy/filter.go` is the
single source of truth for "does this dep speak HTTP?". Match on the
bare image name (last path segment before tag/digest), NOT substring
— substring matching tagged `redis/redisinsight` as Redis.
New blocklist entries go in `nonHTTPImageNames` in that file, never
inlined elsewhere.

### 8. Dial-first port probe
`isHostPortInUse` tries a TCP dial to `127.0.0.1:<port>` before
attempting to bind. Non-root raioz processes previously missed
privileged ports (`:80`) held by another process because `net.Listen`
returned `EACCES`, which was misread as "probe inconclusive". Dial
doesn't need privilege and answers the question we actually care
about: "is something listening?".
