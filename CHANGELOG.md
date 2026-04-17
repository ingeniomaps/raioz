# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Pays back the technical-debt items the v0.1.0 release deferred:
linters, Windows binaries, dependency tracking, image-port detection,
and a coverage push. No breaking changes to `raioz.yaml` or CLI flags.

### Added

- `services.<n>.hostnameAliases` and `dependencies.<n>.hostnameAliases`
  — expose the same upstream under extra subdomains without duplicating
  the declaration. Aliases share one Caddy site block (single
  `reverse_proxy`, one TLS directive under mkcert) and each one gets its
  own `--network-alias` so container→container DNS works for every name.
  Unblocks the Keycloak admin/user split (`sso.example.dev` +
  `accounts.example.dev`) and any multi-hostname API pattern. Empty list
  = prior behavior.
- `raioz down --conflicting` and `--all-projects`. The first stops
  sibling projects (cross-workspace) whose published host ports collide
  with the cwd's `raioz.yaml`, freeing the ports so `raioz up` can
  proceed without a manual `cd` to the other repo. The second stops
  every active raioz project except the cwd's. Detection uses the
  `com.raioz.project` container label; teardown uses `docker rm -f`
  per label. Bypasses the other project's `post:` hook and leaves its
  `.raioz.state.json` stale — the next `raioz up` in that repo
  reconciles via state-vs-docker diff. Flags are mutually exclusive
  and never touch the cwd project itself.
- Windows binaries (`windows/amd64`, `windows/arm64`) ship from
  goreleaser. Process-tree management (Setpgid + group kill on Unix)
  and disk-space probes (`syscall.Statfs`) split behind `_unix.go` /
  `_windows.go` build tags. New `internal/host` exports
  `KillProcessTree`, `ForceKillProcessTree`, `SetNewProcessGroup`, and
  `IsProcessAlive` so the three sites that needed Unix-only signals
  (host runner, host lifecycle, down) share a single cross-platform
  abstraction. Windows uses `taskkill /T` for tree kill, `tasklist`
  for liveness, and `golang.org/x/sys/windows.GetDiskFreeSpaceEx` for
  disk space.
- Proxy routes for image-based dependencies now read `EXPOSE` from the
  image manifest. After deps start, raioz runs
  `docker image inspect --format '{{json .Config.ExposedPorts}}'` for
  any dep whose `detection.Port` is still 0, picks the lowest TCP
  port, and writes it back so the proxy reaches `postgres:5432`,
  `pgadmin4:80`, etc. without the user copying the port into `ports:`
  or `expose:`. Results cache per `image:tag` for the process
  lifetime; lookup failure preserves the existing `Port: 0` fallback
  chain.
- Dependabot now tracks GitHub Actions versions
  (`actions/checkout`, `setup-go`, `golangci-lint-action`, etc.)
  alongside Go modules. Weekly Monday schedule, separate commit
  prefix (`ci`), 5-PR cap.

### Changed

- Lint baseline tightened in four atomic PRs:
  - `errcheck` enabled; `_test.go` excluded. 37 production sites
    addressed (best-effort cleanup gets explicit discards with
    why-comments; real errors propagate or log; Cobra flag boilerplate
    discards).
  - `gosec` enabled; G204 (subprocess with variable) and G306
    (WriteFile permissions) excluded globally with rationale — raioz
    orchestrates docker by design and writes user-readable configs.
    G115 suppressed inline at the one safe site (filesystem block
    size cast).
  - `revive` enabled with a curated 17-rule set (default fires
    ~980 issues mostly from `unused-parameter` and `exported`, which
    don't fit this codebase's conventions). Fixed 5 production hits:
    `copy`/`max` builtin shadowing, empty blocks, var-declaration
    redundancy, if-return collapses.
  - `wrapcheck` enabled scoped to errors from outside raioz
    (`ignorePackageGlobs: raioz/internal/**`); `internal/infra/`
    (hexagonal adapter layer) and `_test.go` exempted. 58 stdlib /
    third-party error sites wrapped with `fmt.Errorf("…: %w", err)`.
- Coverage threshold raised from 70% to 73%, with `internal/mocks`
  and `internal/testing` excluded from the metric (test
  infrastructure, not production code). Real total now ~74%. New
  unit tests cover pure helpers in `compose_spec`, `hosts`,
  `update_port`, `infer_deps`, `naming`, `host/proctree`,
  `production`, and `output`. Path to 80% needs integration tests
  under a live Docker daemon — see ROADMAP.

### Fixed

- `dependencies.<n>.hostname:` and `dependencies.<n>.proxy.port:` are
  now honored by the proxy. Both fields parsed cleanly but were dropped
  by the YAML→`Infra` bridge before reaching `buildProxyRoute`, so
  deps fell back to the entry name as the subdomain and to the
  detection-picked port as the upstream. Multi-port images (e.g.
  `mailhog/mailhog` exposing 1025 SMTP + 8025 UI) ended up routed to
  the wrong port. Added `Hostname` to `Infra`, propagated both fields
  through `yaml_bridge`, and made `proxy.port` standalone (no
  `proxy.target`) override the detected port. Also stops emitting the
  `legacy ports:` warning when a dep declares `proxy:` or `hostname:`
  — the recommended migration to `publish:` + `expose:` would break
  routing in those cases.
- `dependencies.<n>.hostname:` is now honored in runtime under
  workspace-shared mode. The YAML bridge already populated
  `Infra.Hostname` (v0.2.0 fix for issue #001), but `cloneInfraEntry` in
  the workspace-merge path dropped it on every re-up, so the persisted
  route and Caddyfile kept falling back to the entry name. Root cause
  was the same class of silent-field-drop previously fixed for
  `ProxyOverride`: the clone functions reinstantiated the struct field
  by field and new fields weren't listed. `cloneInfraEntry` now copies
  `Hostname`, `HostnameAliases`, and `Seed` (latent bug — the seed file
  list for `/docker-entrypoint-initdb.d/` was also being dropped).
  Regression guarded by a generative test that reflects over every
  exported field on `config.Service` and `config.Infra` and fails if
  the clone returns a zero value — next time someone adds a field, CI
  rejects with a pointer to the clone function to update.
- `host_runner.Restart` no longer ignores the error from `Stop`
  ahead of `Start` — silenced explicitly with a comment so the next
  reader knows the intent.
- `cleanStaleHostProcesses` no longer silently drops the
  `state.SaveLocalState` error after clearing PIDs — the call is
  best-effort but documented.

## [0.1.1] - 2026-04-15

Patch-level fixes for configuration parsing, surfaced by the keycloak
pilot user configuring `raioz.yaml` against v0.1.0 (2026-04-14).

### Added

- `dependencies.<n>.proxy: {target, port}` — mirror of the existing
  `services.<n>.proxy:` escape hatch. Overrides proxy detection for a
  dependency whose runtime raioz can't fully introspect (e.g. a
  `compose:`-backed dep whose target container name or port doesn't
  match the defaults). Bridges into `Infra.ProxyOverride` and is read
  by `buildProxyRoute` via the same `proxyTargetOverride` path used for
  services.
- Advisory warnings for unknown YAML fields at config load. Typos
  (e.g. `whtch:` instead of `watch:`) or fields introduced by a newer
  raioz version now surface as `<file>: line N: field <name> not found
  in type …` on stderr instead of being silently dropped. Warning-only
  by design to preserve forward compatibility; a `--strict` flag for
  hard fail is tracked for a future release.

### Fixed

- `dependencies.<n>.proxy:` was accepted by the YAML parser but
  silently dropped — Caddy then routed the dependency through its
  image's default port (typically 80) regardless of what the user
  declared. The bridge layer now populates `Infra.ProxyOverride` and
  `cloneInfraEntry` propagates it through the workspace-merge path.
- Proxy port fallback for dependencies now consults
  `dependencies.<n>.expose[0]` when detection couldn't resolve a port
  and the legacy `ports:` field is empty. Previously a dep that only
  declared `expose:` would get a proxy route with port 0. `ports:`
  still wins when both are set, preserving existing behavior.

## [0.1.0] - 2026-04-14

First stable release. Complete rewrite from Docker Compose generator
to meta-orchestrator: raioz no longer generates compose files — it
detects runtimes and runs services natively under a shared network
with HTTPS via Caddy.

### Added

#### Core
- `raioz.yaml` as primary config format (services + dependencies).
- Auto-detection of 24 runtimes (Go, Node, Python, Rust, PHP, Java, .NET, Ruby, Elixir, Dart, Swift, Scala, Clojure, Zig, Gleam, Haskell, Deno, Bun, Make, Just, Task, Compose, Dockerfile).
- Zero-config mode: `raioz up` without any config file.
- `raioz init` auto-scans project and generates `raioz.yaml`.
- Host process lifecycle management (PID tracking, cleanup).
- Container runtime abstraction (Docker, Podman, nerdctl).
- Container labels `com.raioz.managed`, `com.raioz.workspace`, `com.raioz.project`, `com.raioz.service`, `com.raioz.kind` stamped on every raioz-created container. Shared deps omit `com.raioz.project` to signal workspace ownership.

#### Proxy & networking
- Caddy reverse proxy with automatic HTTPS via mkcert.
- `https://<service>.<domain>` for every service.
- Custom domain support (`proxy.domain`).
- WebSocket, SSE, and gRPC routing.
- Automatic service discovery with injected env vars.
- Workspace-shared proxy mode — when `workspace:` is declared, a single `{workspace}-proxy` Caddy fronts every project in the workspace. Routes persisted per project at `/tmp/<workspace>/proxy/routes/<project>.json`; Caddyfile is the union of every project's contribution. `raioz down` removes only the current project's routes and reloads; only the last project leaving tumba the proxy.
- `proxy.ip` optional field — pin the Caddy container's IP inside the Docker network. Default: `<subnet-base>.1.1` when `network.subnet` is declared.
- `proxy.publish` optional field (default `true`) — when set to `false`, the proxy does NOT bind host ports 80/443. Reachable only via its container IP, so multiple workspaces can run in parallel without port contention. Requires `network.subnet` or explicit `proxy.ip`. Linux-only.
- `raioz hosts` command — prints an `/etc/hosts` entry for the current project's proxy (container IP + every proxied hostname). Ready for `sudo tee -a /etc/hosts`. Trailing `# raioz:<workspace>` comment makes entries grep-findable.
- Interactive recovery menu when the proxy fails to start on an interactive tty (Retry / Skip / Cancel). Non-interactive stdin still hard-fails.

#### Configuration
- `network.name` and `network.subnet` optional fields in `raioz.yaml` — pin the Docker network name and subnet explicitly. String shorthand (`network: my-net`) also accepted.
- `dependencies.<n>.name` — literal container name override for a dep.
- `dependencies.<n>.routing` — opt-in HTTPS proxy route for a dep whose image matches the DB/broker heuristic.
- `services.<n>.proxy.{target, port}` — override detection when `command:` launches a compose stack raioz can't see.

#### Developer experience
- Multiplexed logs from all services with colored prefixes.
- File watching with debounced auto-restart (`watch: true`).
- `--attach` flag for foreground mode.
- Interactive TUI dashboard (`raioz dashboard`).
- Dependency graph visualization (`raioz graph` — ASCII, DOT, JSON).
- Volume snapshots (`raioz snapshot create/restore/list/delete`).
- Tunnel support (`raioz tunnel` — cloudflared, bore).
- `raioz dev` to hot-swap a dependency from image to local code.
- Package manager auto-detection from lock files (yarn, pnpm, bun).
- Air integration for Go projects with `.air.toml`.
- Workspace naming for Docker resource prefixes.

#### Operations
- Infra health checks with diagnostics.
- `raioz doctor` for system diagnostics.
- `raioz ports` to list port mappings.
- Pre/post hooks (`pre:`, `post:` in config).
- Dependency inference from `.env` files (DATABASE_URL → postgres).

#### Build & CI
- GitHub Actions pipeline with lint, test, and build (consolidated into a single `ci.yml`).
- goreleaser config for cross-platform releases (Linux + macOS; Windows planned).
- Integration test script.

### Changed
- `raioz init` replaced wizard with auto-scan.
- `raioz status` shows runtime type and PID for host services, and reports the correct state for shared/dependency containers (previously always showed `stopped`).
- `raioz list` shows live status for host services.
- Resource naming centralized in `naming` package.
- `.raioz.json` config deprecated with migration warning (`raioz migrate yaml`).
- Install script rewritten for goreleaser compatibility.
- Dependencies in a workspace are now container-shared (`{workspace}-{dep}`), not per-project. First `up` creates; subsequent `up`s reuse. `down` keeps shared deps alive while any sibling project still runs in the workspace; last project out tumba them.
- Certificates are namespaced per domain (`~/.raioz/certs/<domain>/`) and their SAN is verified before reuse. Prevents silent cross-domain cert reuse.
- Caddyfile global block uses `auto_https off` in mkcert mode (was `disable_redirects`). Stops Caddy from falling back to ACME on custom domains without public DNS.
- `raioz.yaml` now fails fast when a name appears in both `services:` and `dependencies:`.
- Proxy startup now pre-flights host ports `80`/`443` and distinguishes `EADDRINUSE` (real conflict) from `EACCES` (privileged port as non-root — not our concern).
- Proxy skips HTTPS route creation for deps whose image matches a well-known non-HTTP list (postgres, redis, mysql, mariadb, mongo, rabbitmq, kafka, etc.). Use `routing: {}` to opt in.
- `.raioz.state.json` is now always written on `up` (even for projects without host services) with project, workspace, `networkName`, and `lastUp` populated. Removed on `down` if project is empty.

### Fixed
- Resolve project name for proxy status and stop.
- `raioz down` no longer sweeps containers belonging to other projects that happen to share a name prefix on the same Docker daemon.
- Service containers with a user-supplied `command:` (e.g. `make start`) are now caught by the down flow via exact-name fallback (`<prefix>-<project>`).
- Caddy proxy no longer gets stuck in `Created` state after a port conflict — stale containers are removed before retry, and the failure is surfaced as an actual error instead of a passable warning.
- `DepComposeProjectName` now uses the active naming prefix instead of hardcoded `raioz-`, so `docker compose ls` matches the real container names.
- Errors from `docker stop` / `rm` during teardown are logged with stderr instead of silently swallowed.
- Proxy port preflight no longer emits false-positive `port in use` for privileged ports when running as non-root.
- Proxy port preflight now uses a TCP dial probe before attempting a bind — unprivileged raioz processes could previously miss privileged ports (e.g. :80) actually held by another process because `net.Listen` returned `EACCES`, which was mistaken for "probe inconclusive".
- `cloneService` / `cloneInfraEntry` in the workspace-conflict merge path now copy ALL orchestration-relevant fields (`ProxyOverride`, `Port`, `HealthEndpoint`, `Name`, `Routing`, `Expose`, `Publish`). Missing fields silently vanished on re-up after a workspace state mismatch.
- `proxy.IsNonHTTPImage` classifier moved to shared `internal/proxy/filter.go` and rewritten to match on the bare image name (last path segment before tag/digest) instead of substrings. `redis/redisinsight`, `dpage/pgadmin4`, `mongo-express`, and similar HTTP UIs that share a substring with their binary-protocol namesake are now correctly proxied.
- Workspace-shared proxy: `Reload` no longer runs `docker cp` (the bind-mount target is read-only and `cp` failed with "device or resource busy"). It writes the Caddyfile on the host and calls `caddy reload` — the bind mount propagates the file into the container automatically.

### Removed
- `raioz workspace` command (replaced by workspace config field).
- `raioz link` command.
- Override system.
- `docker-compose.generated.yml` generation.

---

## Pre-pivot releases

Earlier versions used `.raioz.json` to generate Docker Compose files.
That model is deprecated. Use `raioz migrate yaml` to convert.
