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

### Lint baseline
`.golangci.yml` enables: `govet`, `staticcheck`, `unused`,
`ineffassign`, `gofmt`, `goimports`, `misspell`, `whitespace`,
`copyloopvar`, `bodyclose`, `errcheck`, `gosec`, `revive` (curated
ruleset), `wrapcheck` (scoped to errors from outside `raioz/internal/**`,
infra adapter layer + tests exempted). `_test.go` is excluded from
errcheck/gosec/revive/wrapcheck — tests get their signal from
assertions, not pedantic style rules. Gosec G204 (subprocess with
variable) and G306 (WriteFile permissions) are excluded globally
because raioz orchestrates docker by design and writes user-readable
configs. See `.golangci.yml` itself for the per-rule rationale.

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
- **internal/logging/**, **internal/audit/**, **internal/notify/**, **internal/output/**: Four observability channels with non-overlapping jobs. Channel rules + event matrix in [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) — consult before adding a new event.

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
  # Sibling raioz project as a dep (issue #26)
  keycloak:
    project: ../../keycloak # mode A — sibling IS the dep
    requiredHostname: sso   # optional: assert sibling exposes 'sso'
  kafka:
    image: bitnami/kafka:3
    siblingProject: ../../kafka  # mode B — fallback to image if sibling down
```

### Optional fields recently added (all backward compatible)
- `network.name` / `network.subnet` — pin Docker network scope + IP range.
- `dependencies.<n>.name` — literal container name override (useful when external tooling expects a specific name).
- `dependencies.<n>.routing` — opt a dep whose image matches the DB/broker heuristic (postgres, redis, mysql, mongo, ...) into getting an HTTPS route from Caddy.
- `services.<n>.proxy.{target,port}` — bypass runtime detection for proxy routing. Needed when `command:` launches a compose stack raioz can't see.
- `proxy.ip` — pin the Caddy container's IP inside the Docker network. Default: `<subnet-base>.1.1` when `network.subnet` is declared, otherwise Docker auto-assigns.
- `proxy.publish` (default `true`) — when `false`, the proxy does NOT bind host ports 80/443. The proxy is reachable only through its container IP; users map hostnames via `/etc/hosts`. Enables running multiple workspaces in parallel without port contention. Requires a deterministic IP (subnet or explicit `proxy.ip`). Linux-only (macOS/Windows route Docker through a VM whose bridge IPs aren't reachable).
- `services.<n>.proxy.{target,port}` — bypass runtime detection for proxy routing. Needed when `command:` launches a compose stack raioz can't see.
- `dependencies.<n>.project` — depend on a sibling raioz project (mode A). Empty image/compose; raioz spawns `raioz up` recursively in the sibling cwd when needed. `down` of the consumer never tumba al hermano.
- `dependencies.<n>.siblingProject` — fallback variant: pair with `image:`/`compose:` and raioz skips the local declaration only when the sibling is currently active. Useful for CI / contributors without the sibling cloned.
- `dependencies.<n>.requiredHostname` — assert the sibling's raioz.yaml declares this hostname before deferring to it. Empty = no validation.
- `services.<n>.auth` — opt-in selector for cloning private git repos. Empty (default) keeps the v0.1 strict hardening (public-only); `inherit` delegates to the dev's global git config (credential helper, ssh-agent, OS keychain); `gh` and `ssh` ship as placeholders today (issue 067 fase 1) and become functional in fase 2/3. **The yaml never carries the credential** — only the selector (ADR-036 § secrets-never-in-yaml).

## Key Concepts

- **Services** = local code I'm developing. Raioz detects runtime and starts with native tool.
- **Dependencies** = Docker images I need running. Pulled and started as containers.
- **`raioz dev`** = promote a dependency from image to local (hot-swap). `raioz dev --reset` reverts.
- **Proxy** = Caddy reverse proxy. `https://<service>.localhost` for all services. DNS aliases in Docker network for container-to-container resolution.
- **Service discovery** = auto-injected env vars (`POSTGRES_HOST`, `REDIS_URL`, etc.) with correct hosts based on caller/target runtime.
- **State** = `.raioz.state.json` in project dir (gitignored). Only stores what Docker can't tell us (dev overrides, host PIDs, ignored services).
- **Networking** = one Docker network per workspace. `host.docker.internal` for container→host. Caddy eliminates port conflicts.

## Architectural invariants

These rules are load-bearing — breaking one tends to create a class of
bug. Each links to its ADR for the full rationale (problem, decision,
alternatives). See [docs/decisions/](docs/decisions/) for the format
and how to add new ADRs. For the interaction between locks and
mutexes (project lock vs workspace lock vs in-process map mutexes),
see [docs/LOCKS.md](docs/LOCKS.md). For where raioz writes state
on disk (LocalState, `raioz.root.json`, audit log, routes, certs),
see [docs/STATE.md](docs/STATE.md). For the threat model and what
raioz does NOT protect against, see [docs/SECURITY.md](docs/SECURITY.md).

- **[ADR-001](docs/decisions/001-container-identity-labels.md)** — Containers identified by `com.raioz.*` labels, never by name prefix. New runners MUST stamp the labels via `naming.Labels()`; `make check-labels` enforces literal-free call sites. Files: `internal/naming/labels.go`, `internal/orchestrate/image_runner.go`.
- **[ADR-002](docs/decisions/002-shared-deps-workspace-scoped.md)** — Workspace-shared deps omit `com.raioz.project`; lifecycle uses `otherProjectsActiveInWorkspace`, no refcount file. `ImageRunner.Start` is idempotent.
- **[ADR-003](docs/decisions/003-cert-namespacing.md)** — TLS certs live in `~/.raioz/certs/<domain>/`; `EnsureCerts` validates SANs include both `<domain>` and `*.<domain>` before reuse.
- **[ADR-004](docs/decisions/004-caddy-auto-https-off.md)** — Caddyfile uses `auto_https off` in mkcert mode (ACME would hang on custom domains without public DNS). Do **not** revert to `disable_redirects` alone — that only silences the redirect, not ACME.
- **[ADR-005](docs/decisions/005-workspace-shared-proxy.md)** — One `{workspace}-proxy` Caddy per workspace. Routes persist per project under `${WorkspaceProxyDir()}/<ws>/routes/<project>.json`; Caddyfile is the union. Only the last project to leave the workspace tumba the proxy.
- **[ADR-006](docs/decisions/006-clone-functions-sync.md)** — `cloneService` / `cloneInfraEntry` in `internal/app/upcase/workspace_project_conflict.go` must list every new field on `config.Service` / `config.Infra` or it silently vanishes on re-up. Grep `config.Infra{` / `config.Service{` after struct changes.
- **[ADR-007](docs/decisions/007-image-classification-bare-name.md)** — `proxy.IsNonHTTPImage` matches by **bare image name** (last path segment minus tag/digest), NOT substring (substring tagged `redis/redisinsight` as Redis). New entries go in `nonHTTPImageNames`.
- **[ADR-008](docs/decisions/008-sibling-projects-as-deps.md)** — Sibling raioz projects as deps via `project:` (mode A) or `siblingProject:` + `image:` (mode B). `raioz down` never touches the sibling. Cycle detection via `RAIOZ_SIBLING_STACK`. Mode A spawn uses `os.Executable()`. Workspace coherence required.
- **[ADR-022](docs/decisions/022-unified-state-paths.md)** — `naming.RaiozStateDir()` is the single source of truth for runtime state location. Resolution: `RAIOZ_HOME` → `$XDG_STATE_HOME/raioz` → `~/.local/state/raioz`. `audit`, `ignore`, `workspace` all delegate to it; do NOT add a fourth private copy. `MigrateLegacyStateDirs` runs once from `rootCmd.PersistentPreRun` to lift `~/.raioz` and `/opt/raioz-proyecto` into the unified root on upgrade.
- **[ADR-023](docs/decisions/023-state-mirrors-reality.md)** — State files mirror reality. `raioz down` deletes `raioz.root.json` via `root.Delete(ws)` after a successful full teardown (skipped when leftover containers survive, and when only a subset of services was downed). New state files under `RaiozStateDir()` inherit this contract: `up` writes, `down` deletes.
- **[ADR-024](docs/decisions/024-pre-up-hook.md)** — Two-phase pre-hook. `pre:` runs before infra/sibling-spawn (env/secrets/local FS). `preUp:` runs AFTER infra+sibling-spawn but BEFORE services (`make createdb` against a sibling-spawn'd postgres). Both abort the run on failure. YAML-mode only. The hook runs on the host — reach deps via published `localhost:port`, not container DNS.
- **[ADR-025](docs/decisions/025-launcher-pattern-container-wait.md)** — HostRunner launcher-pattern wait. When a host `command:` exits cleanly in the settle window AND `proxy.target:` names a container, raioz polls Docker until the container appears (up to `RAIOZ_LAUNCHER_TIMEOUT`, default 60s) before reporting ready. `raioz down` waits for in-progress builds (`RAIOZ_LAUNCHER_DRAIN_TIMEOUT`, default 30s) before running `stop:` to avoid orphans. Skipped automatically when `proxy.target:` is host-shaped (dotted name/IP/`host.docker.internal`).
- **[ADR-026](docs/decisions/026-signal-handling-and-pdeathsig.md)** — `cli.Execute` wraps the cobra root in `signal.NotifyContext(SIGINT, SIGTERM)` so every `cmd.Context()` cancels on Ctrl+C. `spawnSibling` (ADR-008 mode A) also sets `Pdeathsig = SIGTERM` on Linux so a kernel-level reap kills the child tree when the parent exits — covers SIGKILL of the parent, which ctx cancellation alone can't. macOS/Windows fall back to the portable half (ctx cancel).
- **[ADR-027](docs/decisions/027-i18n-source-discipline.md)** — `output.Print*` calls must wrap user-visible strings in `i18n.T()`. `scripts/check-i18n-source.sh` + `scripts/i18n-source-baseline.txt` enforce a shrinking-baseline ratchet: new files fail outright; existing files can only shrink. Pattern matches static English literals; dynamic concatenation passes — accepted trade-off. Wired into `make check-i18n-source` and CI alongside `check-i18n`.
- **[ADR-028](docs/decisions/028-shared-map-mutexes.md)** — `Manager.routes` (proxy) and `HostRunner.{processes,launchers}` (orchestrate) gain mutexes + helper methods (`recordPID`/`markLauncher`/`takePID`/`snapshotRoutes` etc). Iteration sites snapshot before doing slow work (docker exec, file writes) so the lock isn't held for seconds. Purely defensive — no current race, prevents the next concurrency feature from corrupting the maps.
- **[ADR-029](docs/decisions/029-app-infra-imports-ratchet.md)** — Baseline ratchet for app-layer direct imports of `internal/{docker,proxy,orchestrate}`. `scripts/lint-app-infra-imports.sh` + `scripts/app-infra-imports-baseline.txt` keep ADR-012 Plan B honest: new files fail outright; existing 22 entries can leave the list when a feature migrates them through the segregated port. `make check-app-infra-imports` wired into `check` and CI.
- **[ADR-030](docs/decisions/030-windows-ci-on-push.md)** — Windows runtime CI gate, push-only. New `Unit tests (Windows)` job on `windows-latest` runs `go test` against OS-sensitive packages (`naming`, `host`, `audit`, `workspace`, `proxy`, `lock`, `ignore`). Catches the runtime forks the cross-compile gate (v0.5.1) cannot: `os.Rename` semantics, `LockFileEx` vs `Flock`, `taskkill` vs `Setpgid`, XDG path fallbacks. Gated on push to develop/main so PRs don't pay the slower runner. See [docs/CI.md](docs/CI.md) for the matrix.
- **[ADR-031](docs/decisions/031-version-field-warning-gate.md)** — `raioz.yaml`'s `version:` field becomes a warning-level gate in v0.6: missing / newer / older / malformed all emit distinct warnings instead of silent acceptance. Comparison is integer-only (`"1"`, `"2"` — not `"1.0"`/`"v1"`). Escalation plan published: v0.7 hard-errors past-version configs, v1.0 hard-errors any mismatch.
- **[ADR-032](docs/decisions/032-proxy-interface-cleanup.md)** — `ProxyManager` port keeps only `Configure(cfg ProxyConfig)`; the 8 per-field setters (`SetDomain`, `SetTLSMode`, …) introduced in ADR-013 are gone. `ProxyConfig.TLSMode` is the typed `interfaces.TLSMode` enum (`TLSModeLocal` / `TLSModeACME` / `TLSModeManual`), not a free-form string. Caddy vocabulary (`"mkcert"`/`"letsencrypt"`) lives only inside the proxy adapter via `caddyTLSValue`; user YAML still accepts the legacy aliases through `ParseTLSMode`.
- **[ADR-033](docs/decisions/033-goreleaser-pr-dry-run.md)** — `release-dry-run` job in `ci.yml` runs `goreleaser release --snapshot --clean --skip=publish,sign` on every PR/push. Catches packaging-time regressions (archive templates, changelog regex, `verify-stamp.sh` script) that the cross-compile gate can't see. On failure uploads `dist/` as a 7-day artifact for inspection. ~1-2 min added to CI per PR.
- **[ADR-034](docs/decisions/034-host-runner-log-fd-cleanup.md)** — `HostRunner.Start`'s wait goroutine owns the parent-side log fd. After `cmd.Wait()` returns, the goroutine closes `logFile` so every exit path (clean-in-window / error / survived-window) releases exactly once. Watch-mode sessions no longer accumulate fds until GC. Regression test `host_runner_fd_linux_test.go` polls `/proc/self/fd`.
- **[ADR-035](docs/decisions/035-env-parse-loud-fallback.md)** — Malformed duration env vars (`RAIOZ_LAUNCHER_TIMEOUT=60` without the `s`) used to fall back silently. `durationFromEnv` now warns once per (process, var) via `sync.Map` dedup; `host.KnownDurationEnvs()` enumerates every duration env var raioz reads, and `raioz doctor::checkEnvironment` surfaces overrides + typos. New duration env vars MUST be appended to `KnownDurationEnvs` or they inherit the silent-fallback bug.
- **[ADR-036](docs/decisions/036-trust-model-yaml.md)** — `raioz.yaml` hygiene policy with three preflight rules: H1 rejects yaml containing embedded credential patterns (regex scan in `internal/config/secret_scan.go`, runs before `yaml.Unmarshal`, no override); H2 rejects paths that escape the project dir or target system locations like `/etc`/`/root` (`internal/config/path_safety.go`); H3 warns on dependency images without an explicit tag or using `:latest` (`internal/config/image_pinning.go`). Sibling project paths (ADR-008) are exempt from H2 containment. The ADR's "won't do" section preserves the rationale for the broader trust-pass scope (URL classification, host allowlist, script heuristics, signing, sandboxing) that was evaluated and rejected — read it before re-proposing any of those features.
- **[ADR-037](docs/decisions/037-replaceable-edge-router.md)** — Workspace `router.project:` (meta config) replaces the bundled Caddy with a sibling raioz project that owns edge routing. The meta runner brings the router up first, sets `RAIOZ_ROUTER_ACTIVE=1` for every consumer sub-up (suppresses their Caddy via `internal/app/upcase/router_env.go`), and tears the router down last. `--router-off` on `raioz up` bypasses the branch for debugging. V1 ships no service-discovery contract — the router owns its own templates.

Every env var raioz reads (RAIOZ_HOME, RAIOZ_RUNTIME, RAIOZ_LANG, RAIOZ_LOG_LEVEL/_JSON, the launcher pair, RAIOZ_SIBLING_STACK, RAIOZ_CORRELATION_ID, plus the XDG bases) is listed in [docs/CONFIG_REFERENCE.md § Environment variables (read by raioz)](docs/CONFIG_REFERENCE.md#environment-variables-read-by-raioz). New env-driven knobs must land in that table — issue 063 closed the discoverability gap.

### `raioz hosts`
Prints the `/etc/hosts` line for the current project's proxy, ready for
`sudo tee -a /etc/hosts`. Only useful in practice with `proxy.publish:
false`, but works in any mode.

## CLI Commands (31 total)

### Core
`up`, `down`, `status`, `logs`, `restart`, `exec`, `check`, `clean`, `init`, `doctor`, `clone`

### Development
`dev` (hot-swap dep→local), `env` (show service env vars), `graph` (visualize deps), `snapshot` (backup volumes), `tunnel` (expose to internet), `proxy` (manage Caddy), `dashboard` (interactive TUI), `hosts` (print `/etc/hosts` line for `proxy.publish:false` setups), `switch` (stop colliding sibling projects + up cwd, with confirmation; `--yes` skips prompt, `--keep` excludes projects)

### Management
`list`, `version`, `lang`, `ignore`, `volumes`, `compare`, `ci`, `health`, `migrate`, `ports`, `yaml` (migrate config)

## Dependencies

- **CLI**: spf13/cobra
- **TUI**: charmbracelet/bubbletea, charmbracelet/lipgloss
- **JSON Schema**: xeipuuv/gojsonschema
- **YAML**: gopkg.in/yaml.v3
- **File watching**: fsnotify/fsnotify
- **Go version**: 1.26

## Patterns

- Dependency injection via `Dependencies` struct (never create deps inline)
- All user messages through `i18n.T()` — never hardcode user-facing strings
- Structured errors: `errors.New(code, i18n.T("error.xxx")).WithSuggestion(...)`
- Tests co-located with source, table-driven with `t.Run`; mocks in `internal/mocks/`
- Compose overlay: never modify user's compose file, use `-f original.yml -f raioz-overlay.yml`
- Detection priority: compose > Dockerfile > package.json > go.mod > Makefile > pyproject.toml > Cargo.toml
- Commit messages: Conventional Commits, English, imperative, max 50 char subject
