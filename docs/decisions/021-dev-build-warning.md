# ADR-021: Dev builds surface a one-time warning on stderr

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`internal/cli/version.go` declares `Version`, `Commit`, `BuildDate`
package-level vars that `make build` rewrites via
`-ldflags="-X ..."`. Binaries built without those flags — `go build`,
`go run`, plain `go install github.com/ingeniomaps/raioz/cmd/raioz` —
ship with the literal defaults (`"dev"`, `"unknown"`). `raioz
version` shows the fallback values and a small "(Development build)"
note at the end.

The failure mode the issue captures: users file bug reports against a
"dev" binary and there's no commit hash for triage. `go install`
without ldflags is a documented install path; nothing in the runtime
signals that the resulting binary can't be traced back to a release.

## Decision

Three coordinated changes:

1. **One-time stderr warning at startup.**
   `MaybePrintDevBuildWarning()` in `internal/cli/version.go` emits a
   short message:

   > warning: this raioz binary was built without version metadata.
   > Bug reports against a dev build can't be traced to a commit.
   > Rebuild with `make build` or `go install -ldflags=...` — see
   > CONTRIBUTING.md.

   `sync.Once` guarantees it appears at most once per process.
   `rootCmd.PersistentPreRun` calls it, gated on `cmd.Name() !=
   "version"` because the version subcommand already prints
   "(Development build)" — duplicating the warning there is noise.

2. **Doctor surfaces it as a yellow check.**
   `app.DoctorUseCase.checkBuildInfo()` emits a `warning` status with
   the same actionable hint. `internal/cli/doctor.go` populates
   `useCase.DevBuild` from the CLI's `IsDevBuild()` so the doctor
   doesn't have to import `internal/cli` (layering preserved per
   ADR-009).

3. **CONTRIBUTING.md** documents the exact `-ldflags` for a
   reproducible local build, both as a `make build` recommendation
   and as the raw `go build` invocation for callers that need it.

## Implementation status

Landed in this commit:

- `internal/cli/version.go`: `IsDevBuild()`,
  `MaybePrintDevBuildWarning()`, `sync.Once` gate.
- `internal/cli/root.go`: PersistentPreRun calls the warning helper
  (skip when the command is `version`).
- `internal/app/doctor.go`: `DoctorUseCase.DevBuild` field +
  `checkBuildInfo()` check.
- `internal/cli/doctor.go`: plumbs `IsDevBuild()` into the use case.
- `CONTRIBUTING.md`: new "Building with reproducible version
  metadata" section.

Smoke-tested both ways:

- `make build` → no warning on stderr, `raioz doctor` shows
  `[ok] Build info` with "release build with version metadata".
- `go build -o /tmp/raioz-dev ./cmd/raioz` (no ldflags) →
  warning on stderr at first command, `raioz doctor` shows
  `[!!] Build info` with the rebuild hint.

## Consequences

### Positive

- Users who accidentally run a dev binary see actionable text the
  first time they run any command. The output isn't recurring
  noise — once per process.
- Bug reports against `raioz version: dev` are no longer mysteries:
  the dev build is loudly identified, and the doctor section
  echoes the same signal for users running diagnostics.
- The path from user → fix is explicit: CONTRIBUTING.md has the
  exact command.

### Negative

- Stderr noise for contributors who knowingly use `go build` to
  iterate. The `sync.Once` keeps it to one line per session, and
  `--quiet` (not implemented) could mute it if it ever bites.
- One more piece of state to plumb through `DoctorUseCase`. The
  `DevBuild` field defaults to `false`, which yields "ok" for any
  test fixture that doesn't bother to populate it — fine, but
  worth noting.

### Neutral

- `IsDevBuild()` checks `Version == "dev"`. If someone passes
  custom ldflags that set Version to something other than "dev"
  (a fork, say), the warning is silenced. That's the right
  behavior — we trust the build to declare itself.

## Alternatives considered

- **runtime/debug.ReadBuildInfo()** instead of ldflag vars. Returns
  rich VCS info for `go install` invocations, but the existing
  Commit/BuildDate flow already works in the make-build path.
  Switching everything to ReadBuildInfo would be a separate ADR;
  here we layer on top of what's there.
- **Fail loudly (`raioz` refuses to start without metadata).**
  Hostile to contributors. The warning + doctor combo gives the
  signal without breaking workflows.
- **Skip the warning, only update doctor.** Doctor is opt-in
  (`raioz doctor` is a command users have to run); the failure
  mode the issue describes is users who never run doctor. Stderr
  is the right surface.

## References

- Code: `internal/cli/version.go`, `internal/cli/root.go`,
  `internal/cli/doctor.go`, `internal/app/doctor.go`.
- Docs: `CONTRIBUTING.md` ("Building with reproducible version
  metadata" section).
- Issue: 041.
