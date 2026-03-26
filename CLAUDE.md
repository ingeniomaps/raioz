# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Raioz is a CLI tool (in Go) that orchestrates local microservice development environments. It reads a declarative `.raioz.json` config, clones repos, generates Docker Compose files, manages networks/volumes/env vars, and brings everything up with `raioz up`. The project is written in Spanish (comments, docs, commit messages).

## Build & Development Commands

```bash
make build          # Build binary with version info (ldflags)
make test           # Run all tests: go test -v ./...
make lint           # golangci-lint (5min timeout)
make format         # gofmt + goimports
make check          # All checks: format + lint + tests
make ci             # Full CI: check + build
make install        # Build and install to /usr/local/bin
make security       # gosec + govulncheck
make mock           # Generate mocks via mockery
```

Run a single test:
```bash
go test -v -run TestFunctionName ./internal/package/...
```

Integration tests (require Docker running):
```bash
go test -v -tags=integration ./cmd/...
```

Skip integration tests: `go test -short ./...`

## Code Quality Constraints (enforced in CI)

- **Max 400 lines per file** (excluding tests) — `make check-lines`
- **Max 120 characters per line** — `make check-length`
- **Test coverage >= 80%** — `make check-coverage`

## Architecture

Follows Clean Architecture: `cmd/` → `internal/app/` → `internal/domain/` → `internal/infra/`

- **cmd/**: Cobra command definitions. Each command loads config, builds dependencies, calls a use case.
- **internal/domain/interfaces/**: Port interfaces (DockerRunner, GitRepository, WorkspaceManager, StateManager, LockManager, etc.). Always program against these.
- **internal/app/**: Use cases orchestrating business logic. Key: `upcase/usecase.go` (the `raioz up` flow), `down.go`, `status.go`, `check.go`.
- **internal/infra/**: Interface implementations wrapping packages in internal/docker/, internal/git/, etc.
- **internal/config/**: Config loading and schema. `schema.go` defines JSON Schema, `deps.go` is the main Deps struct loaded from `.raioz.json`.
- **internal/docker/**: Docker Compose generation (`compose.go`), network/volume/port management. This is the heaviest package.
- **internal/validate/**: JSON Schema validation + business rules + preflight checks (Docker installed, disk space, etc.).
- **internal/state/**: Persists resolved config to `.state.json` for drift detection.
- **internal/env/**: Environment variable resolution and templating.
- **internal/errors/**: Structured `RaiozError` with error codes, context, and user-facing suggestions.
- **internal/workspace/**: Workspace resolution (`~/.raioz/workspaces/{name}/`).
- **internal/root/**: `raioz.root.json` — resolved config with metadata tracking service origins.

## Key Domain Concepts

- **Source kinds**: `git` (clone repo), `image` (Docker image), `local` (local path), `command` (host execution)
- **Docker modes**: `dev` (with source mounts) vs `prod` (built image)
- **Polymorphic config types**: `EnvValue` (array of file paths OR object with vars), `NetworkConfig` (string OR object with name+subnet)
- **State-based drift detection**: compares current `.raioz.json` against saved `.state.json`

## Dependencies

- **CLI framework**: spf13/cobra
- **JSON Schema validation**: xeipuuv/gojsonschema
- **Go version**: 1.22

## Patterns

- Dependency injection via constructor structs (never create dependencies inline)
- Structured errors: use `errors.New(code, msg)` with `.WithContext()` and `.WithSuggestion()`
- Tests co-located with source, table-driven style with `t.Run` subtests
- Logging via Go's `log/slog` (configured by `--log-level` flag or `RAIOZ_LOG_LEVEL` env)
