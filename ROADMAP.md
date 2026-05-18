# Roadmap

Planned work for upcoming raioz releases. **Not a commitment** —
priorities shift based on user feedback from the pilot group and
real-world friction. For shipped history, see
[CHANGELOG.md](CHANGELOG.md); the entries below cross-link there for
detail.

Work lives here until a release is cut, then the items that actually
shipped move to `CHANGELOG.md#Unreleased`, then to a versioned entry.

## v0.1.0 — shipped 2026-04-14

First stable release. Complete rewrite from compose generator to
meta-orchestrator: `raioz.yaml` as primary config, auto-detection
across 24 runtimes, Caddy HTTPS proxy, workspace networking.
See [CHANGELOG.md](CHANGELOG.md) for the full surface.

## v0.1.1 — shipped 2026-04-15

Hotfix from the keycloak pilot. `dependencies.<n>.proxy:` plumbing
(#1), `expose:` fallback (#2), and unknown-field warnings (#4)
shipped. The `ExposedPorts` image-manifest lookup (#3) was deferred
to v0.2.0 because it needed a design pass larger than a patch
release should carry.

## v0.2.0 — shipped 2026-04-15

Paid back the technical debt items v0.1.0 deferred:

- **Lint baseline re-tightened.** `errcheck`, `gosec`, `revive`
  (curated 17-rule set), and `wrapcheck` (scoped to errors from
  outside `raioz/internal/**`) enabled in four atomic PRs. ~770
  production hits fixed across the four linters.
- **Windows binaries** restored via `_unix.go` / `_windows.go` build
  tags in `internal/host` (process trees) and `internal/validate`
  (disk space).
- **`ExposedPorts` lookup** runs `docker image inspect` after deps
  start to backfill the proxy port for images without explicit
  `ports:`/`expose:`.
- **Dependabot** now tracks GitHub Actions versions alongside Go
  modules.
- **Coverage** raised from 70 → 73% (mocks/testing excluded from
  the metric).

## v0.3.0 — shipped 2026-04-17

Workspace-shared follow-ups surfaced by pilot users:

- **`hostnameAliases:`** on services and deps — expose the same
  upstream under extra subdomains (Keycloak admin/user split,
  domain migrations). Shared Caddy site block; each alias gets
  its own `--network-alias`.
- **`dependencies.<n>.hostname`** and **`.proxy.port`** plumbed
  through the YAML → `Infra` bridge so multi-port images route
  to the right port.
- **`cloneInfraEntry` field-drop hardening** with a reflective
  regression test — any new field on `config.Infra` / `config.Service`
  that the clone misses now breaks CI with a pointer to the file to
  update.
- **`raioz down --conflicting`** / **`--all-projects`** — stop
  sibling projects holding host ports or every active raioz project
  except the cwd's.

## v0.4.0 — shipped 2026-05-12

Multi-project orchestration release. Headlines:

- **Sibling raioz projects as deps (#26)** — `project:` (mode A,
  sibling *is* the dep, raioz spawns `raioz up` recursively) or
  `siblingProject:` + `image:` (mode B, fallback to local image
  when the sibling isn't running). `requiredHostname:` validates
  declared hostnames; cycles fail fast.
- **`kind: meta`** at the top of `raioz.yaml` delegates `up`/`down`/
  `status` to a list of sub-projects (each in its own raioz process).
- **`raioz up` detaches by default.** `--attach` keeps the old
  foreground/stream-logs behavior; `--watch` keeps following files.
  Workspace lock released as soon as bring-up completes — unblocks
  parallel up of siblings and `kind: meta` sub-projects. **Breaking**
  for scripts that assumed `raioz up` blocked.
- **Selective targeting** on `raioz down [name…]`, `status [name…]`,
  and `restart [name…]` (the last now correctly honors `--all` /
  `--include-infra` in YAML mode).
- **Lint baseline** migrated to golangci-lint **v2** schema.

Plus a batch of fixes for proxy compose-alias resolution (#43),
status `:latest` spurious tag (#44), host-launcher orphan cleanup,
workspace state out of `/tmp`, and the #26 sibling-deps polish pass.
Full detail in [CHANGELOG.md](CHANGELOG.md).

## v0.5.0 — shipped 2026-05-14

Architecture-hardening release. Headlines:

- **Legacy `.state.json` snapshot API removed** (ADR-011 phases
  1-3). The workspace-project conflict prompt retired with it —
  `raioz down --conflicting` covers the multi-project collision
  case via labels.
- **Domain inversion completed** (ADR-009). Model types own
  themselves; `internal/domain/` no longer pulls `config`, `state`,
  `detect`, or `infra`.
- **`DockerRunner` segregated into six interfaces** (ADR-012
  Plan B). Aggregate keeps embedding all six so callers compile
  unchanged; new tests mock only the surface they exercise.
- **Snapshot / tunnel / proxy lifecycles** moved behind use-case
  ports (ADR-014/015/016) with adapters under `internal/infra/`.
- **Runner dispatch via package-init registry** (ADR-019) replaces
  the 23-case switch.
- **CLI thin-viz lint gate** (ADR-017). `make check-cli-layering`
  prevents silent expansion of the bypass list.
- **Unified state paths** under `naming.RaiozStateDir()` (ADR-022)
  with one-shot migrator for `~/.raioz` and `/opt/raioz-proyecto`.
- **`preUp:` hook** (ADR-024) for bootstrap that needs a
  sibling-spawned dep already up — e.g. `make createdb` against a
  workspace-shared postgres.
- **Launcher-pattern container wait** (ADR-025). When a host
  `command:` declares `proxy.target:`, HostRunner polls for the
  container to appear before reporting ready and drains in-progress
  builds before invoking `stop:`. No more orphan containers when
  `docker compose up -d --build` finishes after `raioz down`.
- **Audit-log rotation** (ADR-020) at a 10 MiB soft cap.
- **Dev-build warning** (ADR-021) when `raioz` was built without
  ldflags; `raioz doctor` surfaces the same signal.
- **`raioz yaml lint`** + `since:` markers on every schema field.

Full detail in [CHANGELOG.md](CHANGELOG.md).

## Tentative next

Items considered for the upcoming release but not yet committed.
Promotion to a versioned section happens at release-cut time.

### Testing — coverage back to 80%

v0.2.0 raised the threshold from 70% to 73% after a focused unit-test
push and excluding `internal/mocks` + `internal/testing` from the
metric. Real total sits at ~74% as of v0.4.0. The remaining gap is
concentrated in packages whose uncovered code needs a live Docker
daemon to exercise:

- `internal/tui` — 41% (bubbletea models)
- `internal/docker` — ~59% (image / network / clean operations)
- `internal/app/upcase` — ~61% (full up-flow integration)
- `internal/tunnel` — ~61% (cloudflared / bore subprocess)
- `internal/workspace` — ~64%

Path forward is integration tests under a real Docker daemon, not
more unit tests — most pure-function gaps already closed. Bump
`COVERAGE_THRESHOLD` to 80 once the integration suite lands.

### Release automation (shipped, v0.9.1)

`release-please` watches pushes to `main` and opens a "release PR"
that bumps the version in `.release-please-manifest.json`, prepends
a CHANGELOG entry generated from conventional commits, and on merge
creates the `v<X.Y.Z>` tag — which fires `release.yml` → goreleaser
as before. Configuration lives in `release-please-config.json`
(`release-type: go`, `bump-minor-pre-major: true`, curated
changelog sections). The existing `[Unreleased]` block stays as
the hand-written cut for releases prepared before release-please
landed; release-please prepends new versioned entries above it.

Manual fallback path still works — the workflow only opens PRs, it
doesn't replace `git tag` if a release must ship out-of-band.

## Future (unscheduled)

Ideas floated but not committed to any release:

- **Native config for Podman / nerdctl** — raioz supports them via
  the runtime abstraction but hasn't been battle-tested on non-Docker
  daemons.
- **Multi-machine workspaces** — one workspace spanning dev laptop +
  remote build box via cloudflared tunnels.
- **`raioz migrate compose`** — import an existing
  `docker-compose.yml` into a `raioz.yaml` (reverse of the current
  legacy-JSON migration).
- **Dashboard polish** — the TUI has 41% coverage and limited
  interaction (view only). Add service restart / exec from the
  dashboard.
- **`--strict` config validation** — turn the unknown-field warnings
  (v0.1.1) into hard errors via a flag, useful for CI.
