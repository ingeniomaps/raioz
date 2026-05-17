# ADR-029: Baseline ratchet for app-layer infra imports

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

ADR-012 chose Plan B for the `DockerRunner` segregation: split
into 6 narrow ports while the aggregate keeps embedding all of
them so callers compile unchanged. The audit (issue 049) found
that 6 months later, **22 production files** under
`internal/app/` and `internal/cli/` import
`internal/{docker,proxy,orchestrate}` directly instead of going
through the segregated ports. The ports exist as declaration; the
app layer doesn't depend on them.

Plan A (delete the aggregate, force every caller through narrow
ports) stays rejected — the migration cost is real and the
user-facing benefit is zero. The question is **how to keep Plan B
honest** without forcing Plan A.

## Decision

A baseline-ratchet lint, matching ADR-001 (label discipline),
ADR-017 (CLI thin-viz), ADR-027 (i18n source discipline).

### `scripts/lint-app-infra-imports.sh`

Greps `internal/app/` and `internal/cli/` (excluding `_test.go`)
for direct imports of `raioz/internal/{docker,proxy,orchestrate}`.
Cross-references against
`scripts/app-infra-imports-baseline.txt`.

Rules:

- **Existing files** on the baseline pass.
- **New files** importing those packages directly fail the lint.
  Use the segregated port (`interfaces.ContainerManager`,
  `interfaces.ProxyManager`, etc.).
- **Stale entries** (file on baseline but no longer matches the
  grep) emit a prune-suggestion note but don't fail — soft
  enforcement keeps noise low while the ratchet stays monotonic.
- **Test files** are exempt. Tests aren't the ADR-012 contract
  surface; they may use concrete packages for fixtures.

Wired into `make check` as `check-app-infra-imports` and into CI.

### What's on the baseline (22 entries)

Clusters:

- `internal/app/upcase/` — orchestrator flow (port allocation,
  validation, sibling dispatch, watch setup, compose,
  host_lifecycle, orchestration, orchestration_proxy).
- `internal/app/down*.go` — teardown family (compose, deps,
  orchestrated, others, proxy, selective).
- `internal/app/{dev,doctor_orchestrator,switch,ports_conflicting,yaml_mode}.go`
  — single-purpose use cases.
- `internal/cli/hosts.go` — pure-viz exception (ADR-017
  allows this).

The list is the public TODO; migration happens opportunistically
when a feature touches the file.

## Implementation status

Landed in this commit:

- `scripts/lint-app-infra-imports.sh`.
- `scripts/app-infra-imports-baseline.txt` (22 entries).
- `Makefile` — `check-app-infra-imports` target wired into
  `check` after `check-cli-layering`.
- `.github/workflows/ci.yml` — CI step after
  `check-i18n-source`.

## Consequences

### Positive

- Plan B becomes enforceable. The migration declared in ADR-012
  becomes a list that shrinks per release rather than a "we'll
  get to it eventually". Drain plan added 2026-05-16 commits the
  baseline to per-cluster release targets (see § "Drain plan"
  below) — silent slip is now visible.
- New code is forced through the ports. The segregated
  interfaces become the contract for net-new development.
- Pattern matches three existing lints in the repo — uniform
  shape (baseline file + grep script + Makefile target + CI
  step).
- Tests exempt — the existing fixture pattern keeps working.

### Negative

- 22 entries visible in source control as the TODO. Some
  prefer hidden lists; we chose visibility because invisible
  TODOs don't shrink.
- The lint allows direct imports if the file is on the baseline.
  No automatic forcing of migration. Same trade-off as ADR-027.
- The stale-entries prune is soft enforcement. Reviewers must
  catch unpruned entries during PR review.

### Neutral

- The lint catches direct imports; it does NOT catch transitive
  coupling (`app/X.go` → `app/Y.go` → `docker/`). Covered by
  `go list -deps` when needed.

## Alternatives considered

- **Plan A migration.** Rejected for the same reasons as
  ADR-012: real cost, no user-facing benefit. Plan A might
  happen at v1.0 if the package boundary calcifies.
- **Allow direct imports forever; remove the segregated
  ports.** Rejected: the ports are still useful documentation
  of what each concern needs, even if app doesn't honor them
  yet. Tests benefit.
- **One big migration PR.** Rejected: 22 files × ~30 LoC each
  with high regression risk. Opportunistic migration is the
  standard answer.

## References

- Code: `scripts/lint-app-infra-imports.sh`,
  `scripts/app-infra-imports-baseline.txt`,
  `Makefile` (`check-app-infra-imports`).
- CI: `.github/workflows/ci.yml`.
- Issue: 049.
- Predecessors: [ADR-001](001-container-identity-labels.md),
  [ADR-012](012-docker-port-segregation.md) (Plan B this ADR
  makes honest), [ADR-017](017-cli-layering-policy.md),
  [ADR-027](027-i18n-source-discipline.md) (baseline ratchet
  pattern).
