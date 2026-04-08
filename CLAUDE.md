# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Raioz is a CLI tool (in Go) that orchestrates local microservice development environments. It reads a declarative `.raioz.json` config, clones repos, generates Docker Compose files, manages networks/volumes/env vars, and brings everything up with `raioz up`.

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

Integration tests (require Docker running):
```bash
go test -v -tags=integration ./cmd/...
```

## Code Quality Constraints (enforced in CI)

- **Max 400 lines per file** (excluding tests) — `make check-lines`
- **Max 120 characters per line** — `make check-length`
- **Test coverage >= 80%** — `make check-coverage`
- **i18n catalogs in sync** — `make check-i18n`

## Architecture

Clean Architecture: `cmd/` → `internal/app/` → `internal/domain/` → `internal/infra/`

- **cmd/**: Thin Cobra commands. Each creates `app.NewDependencies()` + use case, calls `Execute()`. No business logic. Descriptions are set via `i18n.T()` in `zzz_i18n_descriptions.go`.
- **internal/domain/interfaces/**: All port interfaces (DockerRunner, WorkspaceManager, StateManager, ConfigLoader, Validator, GitRepository, LockManager, HostRunner, EnvManager). Types reference `domain/models/` not infra packages.
- **internal/domain/models/**: Type aliases re-exporting domain types from config/, state/, workspace/, host/ — decouples domain layer from infra.
- **internal/app/**: Use cases. Each has an `Options` struct and `Execute()` method. Dependencies injected via `*Dependencies` struct from `dependencies.go`. Uses only domain interfaces, not concrete packages (except for type references).
- **internal/app/upcase/**: The `raioz up` orchestration flow (19 files). Has its own `Dependencies` struct mirroring the parent.
- **internal/infra/**: Adapters implementing domain interfaces, delegating to concrete packages (docker/, git/, workspace/, etc.).
- **internal/config/**: Config loading, JSON Schema, domain types (Deps, Service, SourceConfig, etc.).
- **internal/docker/**: Docker Compose generation, network/volume/port management.
- **internal/state/**: State persistence (`.state.json`), drift detection, preferences.
- **internal/env/**: Environment variable resolution and templating.
- **internal/errors/**: Structured `RaiozError` with codes, context, suggestions. Messages go through `i18n.T()`.
- **internal/i18n/**: Internationalization. `T(key, args...)` function, embedded JSON catalogs in `locales/`, auto-detection of system locale.
- **internal/mocks/**: Manual mock implementations for all 9 domain interfaces.

## Internationalization (i18n)

- **503 keys** in `internal/i18n/locales/en.json` and `es.json`
- All user-facing strings go through `i18n.T("key")` or `i18n.T("key", args...)`
- Fallback chain: saved preference → `RAIOZ_LANG` env → `LANG`/`LC_ALL` → `"en"`
- Preference persisted in `~/.raioz/config.json`
- **Adding a language**: copy `locales/en.json` to `locales/<code>.json`, translate values
- **Cobra descriptions**: set in `cmd/zzz_i18n_descriptions.go` init() (runs last, after i18n.Init)
- `--lang` flag detected early from `os.Args` so `--help` renders in correct language
- `make check-i18n` validates catalog completeness via `TestCatalogCompleteness`

## Key Domain Concepts

- **Source kinds**: `git` (clone repo), `image` (Docker image), `local` (local path), `command` (host execution)
- **Docker modes**: `dev` (with source mounts) vs `prod` (built image)
- **Polymorphic config types**: `EnvValue` (array OR object), `NetworkConfig` (string OR object)
- **State-based drift detection**: compares `.raioz.json` against saved `.state.json`

## Dependencies

- **CLI framework**: spf13/cobra
- **JSON Schema validation**: xeipuuv/gojsonschema
- **Go version**: 1.22

## Patterns

- Dependency injection via `Dependencies` struct (never create deps inline)
- All user messages through `i18n.T()` — never hardcode user-facing strings
- Structured errors: `errors.New(code, i18n.T("error.xxx")).WithSuggestion(i18n.T("error.xxx_suggestion"))`
- Tests co-located with source, table-driven with `t.Run`; mocks in `internal/mocks/`
- Logging via `log/slog` — internal only, not translated
- Commit messages follow `.claude/skills/commit/SKILL.md`: Conventional Commits, English, imperative, max 50 char subject
