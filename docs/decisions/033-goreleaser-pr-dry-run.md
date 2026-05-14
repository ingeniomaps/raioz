# ADR-033: Goreleaser packaging dry-run on every PR

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

v0.5.0 broke at release time: a Unix-only `syscall.Flock` import
killed the Windows cross-build inside goreleaser, and the tag
v0.5.0 had to be deleted and reissued as v0.5.1. ADR-030 added a
cross-compile gate to `ci.yml`'s `Unit tests` job — every PR now
`go build`s the four non-Linux targets. **That catches compile-
time regressions** (the immediate cause of the v0.5.0 incident).

It does **not** catch packaging-time regressions:

- A typo in `archives.name_template` (e.g. `{{ .Version }}` →
  `{{ .Verson }}`) breaks the archive output, but only when
  goreleaser actually runs.
- A regression in `.changelog.filters` or `.changelog.groups`
  produces empty release notes — invisible until the tag.
- The `builds.hooks.post` script (`verify-stamp.sh`, ADR-019)
  could be moved or renamed without anything noticing until the
  next release.
- A future `signs:`, `dockers:`, or `brews:` section that
  depends on missing secrets or templates would fail in the same
  hidden way.

The pattern from v0.5.0 — "rollback is delete-tag, fix, retag" —
applies to every member of this class of bug. The cross-compile
gate covers one entry point; packaging needs its own gate.

## Decision

Add a `release-dry-run` job to `ci.yml` that runs:

```
goreleaser release --snapshot --clean --skip=publish,sign
```

Flag semantics:

- `--snapshot` — bypass the tag requirement. Goreleaser fills
  `.Version` with the default snapshot template
  (`{{ incpatch .Version }}-next-<sha>`).
- `--clean` — wipe `dist/` before the run. Idempotent across CI
  reruns.
- `--skip=publish,sign` — go through every phase **except**
  uploading to GitHub Releases and signing artifacts. The build,
  archive, checksum, changelog, and hook stages all execute.
  `sign` is listed defensively in case signing is configured in
  the future.

The job runs on every PR and every push, on `ubuntu-latest`.
Wall-clock cost: ~1-2 min (six target builds plus archives).

On failure, the job uploads `dist/` as an artifact
(`goreleaser-snapshot-dist`, 7-day retention) so the failure can
be inspected without re-running locally.

### Interaction with existing gates

| Gate                              | Catches                                          |
|-----------------------------------|--------------------------------------------------|
| `Unit tests`/cross-compile (PR)   | Build-time forks (Unix syscalls, build tags)     |
| **`release-dry-run` (PR)** (new)  | Packaging templates, hook scripts, archives, changelog regex |
| `Unit tests (Windows)` (push)     | Runtime forks (`os.Rename`, `LockFileEx`, …)     |

The cross-compile step in `Unit tests` is kept as a faster
early-fail signal — a Go compile error fails in ~10s versus
~90s for the full goreleaser run.

## Implementation status

Landed in this commit:

- `.github/workflows/ci.yml`: new `release-dry-run` job.
- The post-build hook `verify-stamp.sh` (ADR-019) already self-
  skips on cross-built binaries, so no script change is needed.

## Consequences

### Positive

- Future bugs in `.goreleaser.yml` (archive templates, changelog
  regex, hook script paths, …) fail in CI on the PR that
  introduces them, not on the next tag.
- The hook script `verify-stamp.sh` runs on every PR — catches
  regressions to that script before they hit a real release.
- `dist/` artifact upload on failure means debugging doesn't
  require reproducing the goreleaser run locally.

### Negative

- +1-2 min of CI per PR. Trivial compared to the ~30 min cost
  of a re-published release.
- Goreleaser action pulls in another third-party Marketplace
  action (`goreleaser/goreleaser-action@v7`). Already a release-
  time dependency, so no new attack surface.

### Neutral

- `verify-stamp.sh` greps for the snapshot-derived version string
  in the binary output. Both the ldflag and the hook arg use
  `v{{ .Version }}`, so they match. If the snapshot template
  ever diverges from the ldflag template, the hook will fail
  first — fine, that's the kind of regression this job is built
  to surface.

## Alternatives considered

- **Restrict to push events only** (mirroring ADR-030's Windows
  gate). Rejected: packaging breaks don't have the slow-runner
  cost of `windows-latest`; 1-2 min on ubuntu is cheap enough to
  protect every PR.
- **Run goreleaser only when `.goreleaser.yml` changes.** Path-
  filter logic in workflows is brittle and easy to misroute;
  always-on is the right default.
- **Replace cross-compile with goreleaser dry-run.** Goreleaser
  subsumes the cross-compile gate, so we could drop those four
  `go build` lines. Kept for now because the cross-compile step
  finishes in seconds and provides a faster first-fail signal
  than the full packaging run.

## References

- Code: `.github/workflows/ci.yml` (`release-dry-run` job),
  `.goreleaser.yml`, `scripts/verify-stamp.sh`.
- Issue: 056.
- Related: ADR-019 (verify-stamp hook), ADR-030 (Windows CI
  gate).
