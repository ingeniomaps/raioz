# Roadmap

Planned work for upcoming raioz releases. **Not a commitment** —
priorities shift based on user feedback from the pilot group and
real-world friction. For shipped history, see
[CHANGELOG.md](CHANGELOG.md).

Work lives here until a release is cut, then the items that actually
shipped move to `CHANGELOG.md#Unreleased`, then to a versioned entry.

## v0.2.0 (tentative)

The v0.1.0 release cut corners in three areas to ship on time. v0.2.0
pays that debt back before adding new surface.

### Code quality — re-tighten lint baseline

For v0.1.0, `.golangci.yml` was reduced to a minimal set (govet,
staticcheck, unused, ineffassign, gofmt, goimports, misspell,
whitespace, copyloopvar, bodyclose) because the strict config fired
~2,500 issues across a mature CLI codebase — mostly false positives.

Re-introduce the stricter linters one at a time, with a focused
cleanup PR per linter. Priority order:

1. **`errcheck`** — catches real error-handling gaps (best signal, ~135 issues in v0.1.0).
2. **`gosec`** — CLI shells out, so security wins are concrete (~365 issues).
3. **`revive`** — curate a ruleset that excludes the noisy pedantic ones; the full default fires ~988 issues, most opinionated.
4. **`wrapcheck`** — purely stylistic, do last (~260 issues).

Each PR: enable one linter, fix every instance it flags, commit, done.
Don't re-enable the config and the fixes in separate PRs — CI would
break between them.

### Testing — coverage back to 80%

v0.1.0 threshold was lowered from 80% → 70% to match the actual state.
Push it back up by covering the thinner packages:

- `internal/tui` — 41%
- `internal/docker` — 59.6%
- `internal/app/upcase` — 60.8%
- `internal/tunnel` — 61.1%
- `internal/workspace` — 62.7%

Then bump `COVERAGE_THRESHOLD` in `Makefile` back to 80.

### Platform — Windows binary support

v0.1.0 ships only Linux + macOS binaries. Windows users must run raioz
via WSL2. Dropping Windows was a release-day fix because
`internal/orchestrate/host_runner.go` uses `syscall.SysProcAttr.Setpgid`
+ `syscall.Kill`, and `internal/validate/preflight.go` uses
`syscall.Statfs` — all Unix-only.

Split into `*_unix.go` and `*_windows.go` with platform-specific
implementations:

- Linux/macOS: existing code unchanged.
- Windows: equivalent via `golang.org/x/sys/windows` (`GetDiskFreeSpaceEx` for disk; `CreateJobObject`/`TerminateJobObject` for process groups) or documented stubs that degrade gracefully.

Then re-add `windows` to `.goreleaser.yml` `goos` list.

### Tooling — dependabot for GitHub Actions

`.github/dependabot.yml` today only covers Go modules. GitHub Actions
versions (`actions/checkout`, `setup-go`, `golangci-lint-action`,
`goreleaser/goreleaser-action`) drift silently. Add a second
`package-ecosystem: "github-actions"` entry so Dependabot opens PRs
for workflow action bumps too.

### Release discipline — hotfix flow

The v0.1.0 release required four re-tags (Go version, workflow
consolidation, lint config validation, Windows drop). For any urgent
fix against a released version:

1. Branch `hotfix/v0.1.x` from `main`.
2. Fix + test on that branch.
3. Merge to BOTH `main` AND `develop` (not just main — otherwise develop loses the fix).
4. Tag `v0.1.1` on main.

Don't keep re-tagging the same version. If the release pipeline is
already running, bump the patch version instead of deleting/re-pushing
the same tag.

## Future (unscheduled)

Ideas floated but not committed to any release:

- **`release-please` integration** — automate the version bump + CHANGELOG promotion + tag creation by opening a PR on every merge to `main`.
- **Native config for Podman/nerdctl** — raioz supports them via the runtime abstraction but hasn't been battle-tested on non-Docker daemons.
- **Multi-machine workspaces** — one workspace spanning dev laptop + remote build box via cloudflared tunnels.
- **`raioz migrate compose`** — import an existing `docker-compose.yml` into a `raioz.yaml` (reverse of the current legacy-JSON migration).
- **Dashboard polish** — the TUI has 41% coverage and limited interaction (view only). Add service restart / exec from the dashboard.
