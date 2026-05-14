# ADR-030: Windows unit-test job, gated on push

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

v0.5.1 shipped a hotfix for a Windows cross-compile failure
(`syscall.Flock` slipped into a file without a build tag). The
incident drove a cross-compile gate in `ci.yml`'s `Unit tests` job:
`go build` against `windows/amd64`, `windows/arm64`, `darwin/amd64`,
`darwin/arm64` after the Linux build. That catches **build-time**
regressions on every PR.

Issue 050 noted what the build-only gate doesn't catch:

1. `os.Rename` semantics on Windows (destination must not exist)
   — audit rotation (ADR-020) silently fails when `audit.log.1`
   is already there.
2. `naming.RaiozStateDir()` returns `~/.local/state/raioz`
   literal on Windows; should map to `%LOCALAPPDATA%`-shaped.
3. `proctree_windows.go::isProcessAlive` uses substring match
   on tasklist CSV — PID 22 reads as alive when only PID 522
   is running.
4. `LockFileEx` vs `Flock` under fork/exec: recursive
   `os.Executable()` spawn (ADR-008 mode A) inherits FDs
   differently across OSes.

All four are runtime bugs the cross-compile gate cannot detect.

## Decision

Add a fourth job `Unit tests (Windows)` that runs `go test` on
`windows-latest` against OS-sensitive packages only:

`internal/naming`, `internal/host`, `internal/audit`,
`internal/workspace`, `internal/proxy`, `internal/lock`,
`internal/ignore`.

**Gated on `push` only.** PRs already pay for Linux unit tests
+ integration E2E + the build-only cross-compile. The Windows
runner adds 4-6 min; deferring it to push-to-develop /
push-to-main keeps PR wall-clock predictable while still
catching regressions before a release.

The package list is intentionally narrow:

- Pure-Go packages (config, errors, naming utility, i18n) are
  already covered by Linux. Pure Go behaves identically across
  platforms.
- Tests needing Docker (E2E orchestration) stay Linux-only.
- Tests using POSIX shell helpers
  (`internal/app/upcase/sibling_spawn_test.go::TestSpawnSibling_*`)
  already `t.Skip` on Windows.

`docs/CI.md` is the operational matrix — what runs where, when,
why. ADRs are for the policy decision; CI.md keeps the runtime
facts.

## Implementation status

Landed in this commit:

- `.github/workflows/ci.yml`: new `test-windows` job, gated on
  `github.event_name == 'push'`. Sits after `integration`; the
  ordering is for log readability, not workflow dependency.
- `docs/CI.md` (new): matrix doc plus rationale for the
  packages selected and the push-only gate.
- `CONTRIBUTING.md`: one-paragraph addition linking to CI.md.

## Consequences

### Positive

- Windows-runtime-only regressions fail on develop/main merge,
  not on user reports. v0.5.1 incident shape no longer reaches
  production silently.
- Narrow package list keeps the job at 4-6 min — acceptable
  marginal CI cost. Easy to expand when a new OS-sensitive
  package ships.
- `docs/CI.md` is contributor-readable. "What runs in CI?" has
  a 30-second answer.

### Negative

- Windows runners are 2x Linux per-minute billing. The
  push-only gate scopes the cost — releases pay a few cents
  extra; PRs pay nothing.
- Tests flaky on Windows (filesystem perms, path quirks)
  surface here first. Triage cost bounded by the narrow list.
- Regression in a PR's Windows-specific code only fails post-
  merge. Mitigation: contributors with Windows access can run
  the matching tests locally; cross-compile gate still catches
  build-time issues on every PR.

### Neutral

- The "OS-sensitive packages" list is editorial. Contributors
  adding a new package with platform-tagged files must update
  both `ci.yml` and `docs/CI.md`. CI.md's "Adding new coverage"
  section spells this out.

## Alternatives considered

- **Run Windows on every PR.** Rejected: doubles PR wall-clock
  for a rare regression class. Cross-compile gate handles
  build-time cheaply. Reconsidered if Windows runtime
  regressions become common.
- **Run `go test ./...` on Windows.** Rejected: many tests
  would fail for reasons unrelated to the package (POSIX
  shell, hardcoded `/tmp`). Narrow list catches high-value
  targets without the noise.
- **`nodocker` build tag.** Rejected for now — would require
  tagging every Docker-touching test. Package-list approach
  is good enough until a new OS-sensitive package lands
  outside the listed set.
- **Skip Windows entirely; trust contributors with Windows
  machines.** Rejected: v0.5.1 proved nobody was checking. CI
  is the durable check.

## References

- Workflow: `.github/workflows/ci.yml`.
- Matrix doc: `docs/CI.md`.
- Issue: 050.
- Driving incident: v0.5.1 hotfix (cross-compile gate added
  there).
- Related: [ADR-026](026-signal-handling-and-pdeathsig.md)
  flagged macOS kqueue / Windows Job Objects as runtime
  follow-ups — this ADR is the testing companion.
