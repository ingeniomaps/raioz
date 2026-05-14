# CI matrix

Reference for what raioz tests where, and why. Lives next to
`docs/ARCHITECTURE.md` because the matrix is part of the
project's architecture — a contributor changing OS-sensitive code
needs to know what coverage they get from CI.

## What runs where

| Job | Runner | Trigger | Scope |
|-----|--------|---------|-------|
| `Lint` | `ubuntu-latest` | every push + PR | gofmt, golangci-lint, file/line caps, i18n catalogs + source discipline, label discipline, schema fixtures + `since:` markers, CLI thin-viz, app-layer infra imports baseline |
| `Unit tests` | `ubuntu-latest` | every push + PR | `go test -race ./...` + cross-compile to windows/darwin × amd64/arm64 (build-only, smoke for goreleaser) |
| `Integration E2E` | `ubuntu-latest` | every push + PR (needs unit tests) | bring up a real toy service via the installed `raioz` binary, exercise init/up/status/logs/down + restart cycle |
| `Goreleaser dry-run` | `ubuntu-latest` | every push + PR | `goreleaser release --snapshot --clean --skip=publish,sign` — full packaging pipeline minus the publish step (ADR-033) |
| `Unit tests (Windows)` | `windows-latest` | push to main/develop only | OS-sensitive packages: `naming`, `host`, `audit`, `workspace`, `proxy`, `lock`, `ignore` |

Triggers in detail:

- **PR** → Lint + Unit tests + Integration E2E + Goreleaser dry-run.
- **Push to develop or main** → all of the above plus
  `Unit tests (Windows)`.
- **Tag** (release.yml) → goreleaser builds + publishes assets.

The Windows job is push-only to keep PR wall-clock low. The
cross-compile build in the Linux `Unit tests` job catches
build-time regressions on every PR; the Windows job catches
runtime-behavior regressions when develop / main are updated.

## What the Windows job tests

The package list is intentionally narrow — packages that touch
OS-sensitive surfaces. Anything CPU-bound + pure-Go (parsers,
config, i18n, errors, naming utility) is already covered by the
Linux job, which runs on the same Go binary and produces
identical results across OSes for pure-Go code.

| Package | Why on Windows |
|---------|----------------|
| `internal/naming` | `RaiozStateDir()` / `RaiozConfigDir()` XDG fallback. Path separators. |
| `internal/host` | `proctree_windows.go::taskkill` + `isProcessAlive` parsing tasklist. Settle window timing on Windows clock. |
| `internal/audit` | `os.Rename` rotation semantics (Windows fails on existing destination). |
| `internal/workspace` | Base dir resolution (`workspaces/<project>` path joins). |
| `internal/proxy` | `workspace_lock_windows.go::LockFileEx` per-handle semantics. |
| `internal/lock` | PID lockfile + process-liveness via `syscall.Signal(0)` (mapped on Windows). |
| `internal/ignore` | Same state path resolution as audit / workspace. |

## What the Windows job does NOT test

By design:

- **Docker integration.** Windows runners on GitHub don't ship
  Docker by default and the workspace-shared proxy is documented
  Linux-only (proxy.publish:false relies on bridge IPs that Docker
  Desktop on Windows routes through a VM the host can't reach).
- **POSIX shell helpers** in tests. The fake-binary pattern used
  in `internal/app/upcase/sibling_spawn_test.go` writes `#!/bin/sh`
  scripts; those tests `t.Skip` on Windows.
- **End-to-end orchestration** (`raioz up` of a real service).
  Lives in the `Integration E2E` job on Linux only.

## Cross-compile gate

`Unit tests` also cross-builds the goreleaser targets:

- `windows/amd64`
- `windows/arm64`
- `darwin/amd64`
- `darwin/arm64`

`go build` only — no tests. This was added in v0.5.1 after
goreleaser failed to build Windows binaries because a Unix-only
syscall slipped in (issue 047). The gate catches the same class
of regression on every PR without paying for a Windows runner.

## Goreleaser dry-run

`Goreleaser dry-run` runs `goreleaser release --snapshot --clean
--skip=publish,sign` on every PR/push. The cross-compile gate
above only validates `go build`; this job exercises the full
packaging pipeline — archive templates, checksum generation,
changelog regex, and the `builds.hooks.post` script
(`scripts/verify-stamp.sh`, ADR-019) — without publishing to
GitHub Releases.

On failure the job uploads `dist/` as a 7-day-retention
artifact (`goreleaser-snapshot-dist`) so a contributor can
inspect the partial output without re-running locally.

Rationale and design notes: [ADR-033](decisions/033-goreleaser-pr-dry-run.md).

## Adding new coverage

- **A new package with OS-sensitive code** → add it to the
  Windows job's package list and update this matrix.
- **A new package that's pure Go** → the existing `./...` on
  Linux already covers it. No CI change needed.
- **A new build-tagged file pair** (e.g. `_unix.go` /
  `_windows.go`) → ensure both pass `go vet` cross-compiled and
  add a unit test in the matching `_test.go` to exercise the
  helper's contract. The pair pattern is the same as
  `proctree_unix.go` / `proctree_windows.go` and
  `workspace_lock_unix.go` / `workspace_lock_windows.go`.

## Cost

Approximate wall-clock per job (May 2026 baseline):

| Job | Time |
|-----|------|
| `Lint` | 1-2 min |
| `Unit tests` (Linux, with race + cross-compile) | 3-4 min |
| `Integration E2E` | 1-2 min |
| `Goreleaser dry-run` | 1-2 min |
| `Unit tests (Windows)` | 4-6 min |

Windows runners on GitHub-hosted infrastructure are 2x the
per-minute billing of Linux runners. The push-only gate makes
the Windows job effectively free on PRs and predictable on
develop/main merges.

## References

- Workflow: [.github/workflows/ci.yml](../.github/workflows/ci.yml).
- Release pipeline: [.github/workflows/release.yml](../.github/workflows/release.yml).
- Issues: 050 (Windows gate), 056 (goreleaser dry-run).
- ADRs: [ADR-030](decisions/030-windows-ci-on-push.md), [ADR-033](decisions/033-goreleaser-pr-dry-run.md).
- v0.5.1 incident (drove the cross-compile gate):
  [CHANGELOG.md](../CHANGELOG.md#051---2026-05-14).
