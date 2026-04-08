# Pending Migrations

Two gradual, pragmatic migrations are in progress. Both follow the same approach: migrate file-by-file, prioritize high-impact areas first.

---

## 1. Structured Logging (`fmt.Printf` -> `log/slog`)

**Goal**: Replace raw `fmt.Printf` calls with structured logging or the `output` package.

**Pattern**:

| Call type | Migration target |
|-----------|-----------------|
| Warnings/errors | `logging.Warn()` / `logging.Error()` |
| User info messages | `output.PrintInfo()` |
| User success messages | `output.PrintSuccess()` |
| Debug/internal logs | `logging.Debug()` / `logging.Info()` |

**Note**: `internal/output` keeps its special formatting (emojis, colors). Only internal diagnostic prints move to `slog`.

**Status**: ~10 of 146+ occurrences migrated.

| File | Occurrences | Status |
|------|-------------|--------|
| `cmd/up.go` | 3 | Done |
| `cmd/down.go` | 7 | Done |
| `cmd/dependency_assist.go` | 41 | Pending (interactive, complex) |
| `internal/output/format.go` | 15 | Pending (keep user format) |
| `cmd/clean.go` | 13 | Pending |
| `cmd/status.go` | 10 | Pending |
| `cmd/override.go` | 9 | Pending |
| `cmd/list.go` | 9 | Pending |
| `cmd/workspace.go` | 8 | Pending |
| `cmd/version.go` | 5 | Pending |
| `cmd/ports.go`, `link.go`, `ignore.go`, `check.go` | 3 each | Pending |
| Other `internal/` files | ~27 | Pending |

---

## 2. Context with Timeouts (`exec.Command` -> `exec.CommandContext`)

**Goal**: Add `context.Context` with predefined timeouts to all subprocess calls.

**Pattern**:

```go
// Before
cmd := exec.Command("docker", "compose", "up", "-d")

// After
ctx, cancel := exectimeout.WithTimeout(exectimeout.DockerComposeUpTimeout)
defer cancel()
cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
```

**Predefined timeouts**: git clone 10m, git pull 5m, git checkout 2m, docker compose up 5m, docker compose down 2m, docker pull 15m, docker inspect 30s, default 5m.

**Status**: Git layer done, Docker layer pending (~50+ occurrences).

| Package | Status |
|---------|--------|
| `git/branch.go` | Done |
| `git/remote.go` | Done |
| `git/version.go` | Done |
| `git/clone.go` | Done |
| `git/readonly.go` | Done |
| `docker/inspect.go` (git-related) | Done |
| `state/check.go` | Done |
| `cmd/up.go` | Done |
| `docker/images.go` | Pending |
| `docker/network.go` | Pending |
| `docker/volumes.go` | Pending |
| `docker/status.go` | Pending |
| `docker/clean.go` | Pending |
| `docker/logs.go` | Pending |
| `docker/inspect.go` (internal) | Pending |
| `validate/preflight.go` | Pending |
