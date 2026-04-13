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
internal/cli/                   Cobra commands (27 total)
         │                      Thin: parse flags → create deps → call use case
         │
internal/app/                   Use cases
         │                      Options + Execute() pattern
         │                      DI via *Dependencies struct
         │                      Only uses domain interfaces
         │
internal/domain/                Contracts
  interfaces/                   27 port interfaces
  models/                       Type aliases for decoupling
         │
internal/infra/                 Adapters
  docker/config/git/...         Implement domain interfaces
         │                      Delegate to concrete packages
         │
internal/                       Concrete packages
  detect/                       Runtime detection (24 runtimes)
  orchestrate/                  Dispatcher + runners
  proxy/                        Caddy management
  discovery/                    Service discovery env vars
  watch/                        File watcher
  naming/                       Docker resource naming
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

Centralized naming for all Docker resources:

```
With workspace:    {workspace}-{project}-{service}
Without workspace: raioz-{project}-{service}
Network:           {workspace}-net or raioz-{project}-net
```

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
