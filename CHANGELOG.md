# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed
- Deprecate `.raioz.json` config with migration warning
- Rewrite install script for goreleaser compatibility

### Fixed
- Resolve project name for proxy status and stop

## [0.1.0] — Meta-orchestrator

Complete rewrite from Docker Compose generator to meta-orchestrator.
Raioz no longer generates compose files — it detects runtimes and
runs services natively.

### Added

#### Core
- `raioz.yaml` as primary config format (services + dependencies)
- Auto-detection of 24 runtimes (Go, Node, Python, Rust, PHP, Java, .NET, Ruby, Elixir, Dart, Swift, Scala, Clojure, Zig, Gleam, Haskell, Deno, Bun, Make, Just, Task, Compose, Dockerfile)
- Zero-config mode: `raioz up` without any config file
- `raioz init` auto-scans project and generates `raioz.yaml`
- Host process lifecycle management (PID tracking, cleanup)
- Container runtime abstraction (Docker, Podman, nerdctl)

#### Proxy & networking
- Caddy reverse proxy with automatic HTTPS via mkcert
- `https://<service>.<domain>` for every service
- Custom domain support (`proxy.domain`)
- WebSocket, SSE, and gRPC routing
- Automatic service discovery with injected env vars

#### Developer experience
- Multiplexed logs from all services with colored prefixes
- File watching with debounced auto-restart (`watch: true`)
- `--attach` flag for foreground mode
- Interactive TUI dashboard (`raioz dashboard`)
- Dependency graph visualization (`raioz graph` — ASCII, DOT, JSON)
- Volume snapshots (`raioz snapshot create/restore/list/delete`)
- Tunnel support (`raioz tunnel` — cloudflared, bore)
- `raioz dev` to hot-swap a dependency from image to local code
- Package manager auto-detection from lock files (yarn, pnpm, bun)
- Air integration for Go projects with `.air.toml`
- Workspace naming for Docker resource prefixes

#### Operations
- Infra health checks with diagnostics
- `raioz doctor` for system diagnostics
- `raioz ports` to list port mappings
- Pre/post hooks (`pre:`, `post:` in config)
- Dependency inference from `.env` files (DATABASE_URL → postgres)

#### Build & CI
- GitHub Actions pipeline with lint, test, and build
- goreleaser config for cross-platform releases
- Integration test script

### Changed
- `raioz init` replaced wizard with auto-scan
- `raioz status` shows runtime type and PID for host services
- `raioz list` shows live status for host services
- Resource naming centralized in `naming` package

### Removed
- `raioz workspace` command (replaced by workspace config field)
- `raioz link` command
- Override system
- `docker-compose.generated.yml` generation

---

## Pre-pivot releases

Earlier versions used `.raioz.json` to generate Docker Compose files.
That model is deprecated. Use `raioz migrate yaml` to convert.
