# Roadmap

Planned work for upcoming raioz releases. **Not a commitment** —
priorities shift based on user feedback from the pilot group and
real-world friction. For shipped history, see
[CHANGELOG.md](CHANGELOG.md).

Work lives here until a release is cut, then the items that actually
shipped move to `CHANGELOG.md#Unreleased`, then to a versioned entry.

## v0.1.1 — shipped 2026-04-15

Hotfix release surfacing from the keycloak pilot user (2026-04-14).
Fixes for `dependencies.<n>.proxy:` being silently dropped (#1),
the `expose:` fallback (#2), and unknown-field warnings (#4) shipped
— see [CHANGELOG.md#011---2026-04-15](CHANGELOG.md). The
`ExposedPorts` lookup (#3) was deliberately deferred to v0.2.0
because it needs a design pass larger than a patch release should
carry; see below.

## v0.2.0 (tentative)

The v0.1.0 release cut corners in three areas to ship on time. v0.2.0
pays that debt back before adding new surface.

### Read `ExposedPorts` from the Docker image manifest

_Deferred from v0.1.1._ `detect.ForImage()`
(`internal/detect/detect.go:363`) returns `Port: 0` for every image.
Most official images declare `EXPOSE` in their Dockerfile (e.g.
`redis/redisinsight: 5540`, `dpage/pgadmin4: 80,443`,
`postgres:16: 5432`), so the proxy route ends up incomplete for
common deps unless the user adds `expose:` manually (partial
mitigation already shipping in v0.1.1, but that only covers cases
where the user opts in).

Fix: run `docker inspect --type=image --format
'{{json .Config.ExposedPorts}}'` and take the first TCP port. Cache
by image:tag. Pull on demand with a short timeout; fall back to
`Port: 0` (current behavior) if pull fails. Larger change — design
pass, caching layer, and offline-mode fallback — which is why it
didn't fit the v0.1.1 hotfix window.

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

v0.1.0 threshold was 70%. v0.2.0 raised it to 73% after a focused
testing push and excluding `internal/mocks` + `internal/testing` from
the metric (they're test infrastructure, not production code). Real
total now sits at ~74%.

Remaining gap to 80% is concentrated in packages whose uncovered code
needs a live Docker daemon to exercise:

- `internal/tui` — 41% (bubbletea models; integration-test heavy)
- `internal/docker` — 58.9% (image / network / clean operations)
- `internal/app/upcase` — 61.2% (full up flow integration)
- `internal/tunnel` — 61.1% (cloudflared / bore subprocess)
- `internal/workspace` — 64.2%

Path forward is integration tests under a real Docker daemon, not more
unit tests — most pure-function gaps already closed. Bump
`COVERAGE_THRESHOLD` to 80 once the integration suite lands.

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
