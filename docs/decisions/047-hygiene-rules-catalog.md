# ADR-047: Hygiene-rules catalog

- **Status:** Accepted
- **Date:** 2026-05-18

## Context

The architectural invariants section of `CLAUDE.md` lists rules
that govern the codebase. Most link to a dedicated ADR
(ADR-001..046) that records the full problem / decision /
alternatives chain. Two rules currently live in CLAUDE.md as
`Hygiene rule, no ADR — paired with issue NNN`:

- Host-gateway injection: every runner that owns a container must
  add `host.docker.internal:host-gateway` to `extra_hosts`
  (paired with local issue 021).
- Atomic state writes: every write under `RaiozStateDir()` or
  `<projectDir>/.raioz.state.json` must go through
  `fsutil.WriteFileAtomic` (paired with local issue 034).

That pairing has two problems. The issues are local drafts under
`docs/issues/` that get deleted upon resolution (per the project's
tracking convention — see CLAUDE.md / memory). Once the issue is
closed and the file removed, the only surviving record is the
one-paragraph rule in CLAUDE.md with no rationale, no enforcement
mention, and no link a future contributor can follow. A reviewer
who wants to understand *why* the rule exists has nothing to read.

We do not want to inflate every paragraph-sized rule into its own
ADR — the dedicated-ADR overhead (Context / Decision /
Consequences / Alternatives / References) is appropriate when a
real decision was weighed, not when an obviously-correct invariant
is being pinned. But the rule still needs a discoverable home.

## Decision

This ADR is the catalog. New hygiene rules that don't warrant
their own ADR are appended here, each with:

- The rule itself, in the same imperative form as CLAUDE.md.
- The reason it exists (the bug class it prevents, the past
  incident if any).
- The files / packages where the rule applies, with file paths.
- The enforcement mechanism (lint check, code review, test) — or
  an explicit "code review only" if there's no automation yet.

CLAUDE.md keeps its one-paragraph entry for each rule, but the
final clause changes from `no ADR — paired with issue NNN` to
`see ADR-047 § <section name>`.

## Threshold: catalog vs dedicated ADR

A rule belongs in this catalog when ALL of these hold:

- The decision is forced by an external constraint (Docker
  semantics, OS file-system behavior, security model) — not a
  weighed tradeoff between alternatives we evaluated.
- No reasonable alternative was rejected. The rule is the only
  correct answer, not a chosen one.
- Enforcement is mechanical or near-trivial (run X, never run Y).

A rule warrants a dedicated ADR when ANY of these hold:

- Real alternatives were evaluated and rejected. The "Alternatives
  considered" section would be load-bearing.
- The rule trades one class of pain for another (e.g. ADR-005
  workspace-shared proxy trades simpler routing for shared
  lifecycle). A future maintainer needs to know the tradeoff so
  they don't undo it.
- The decision affects user-visible behavior (config keys,
  command output, exit codes).

When in doubt, prefer a dedicated ADR. The catalog is for rules
where a dedicated ADR would mostly be ceremony.

## Catalog

### H-1. Host-gateway alias in extra_hosts

Every runner that owns a container MUST inject
`host.docker.internal:host-gateway` into its `extra_hosts` (or the
runtime-specific equivalent gated by the ADR-046 capability matrix
when `HostGatewayAlias` is false).

**Why.** `internal/discovery/discovery.go` hardcodes
`host.docker.internal` as the container→host bridge for service
discovery env vars. Docker Desktop ships the alias by default on
macOS / Windows, but Linux without Docker Desktop only resolves it
when the container's `extra_hosts` explicitly maps it to the magic
`host-gateway` value. Without the mapping, container→host calls
NXDOMAIN and every service-discovery env var pointing at the host
fails silently.

**Where.**

- `internal/orchestrate/compose_runner.go` — compose overlay.
- `internal/orchestrate/dockerfile_runner.go` — dockerfile run.
- `internal/orchestrate/image_runner.go` — image-only deps.
- `internal/proxy/proxy.go` — Caddy container.

**Enforcement.** Code review. The host-gateway literal is rare
enough in the codebase that a grep audit (`grep -rn host-gateway
internal/`) confirms the four call sites at a glance; new runners
added without it would surface in proxy-route smoke tests.

**Capability gating.** Runtimes that reject the magic value
(nerdctl 1.x) get the injection skipped via the ADR-046 capability
check on `HostGatewayAlias`. Adding the literal raw bypasses that
check — runners MUST read the capability matrix.

### H-2. Atomic state writes

Every write under `naming.RaiozStateDir()` or
`<projectDir>/.raioz.state.json` MUST go through
`fsutil.WriteFileAtomic` (`internal/fsutil/atomic.go`). Direct
`os.WriteFile` is prohibited for raioz-owned state files.

**Why.** State files are read at startup by
`LoadLocalState` / equivalents and parsed as JSON. A SIGKILL
(Ctrl-C through the parent shell, OOM, machine sleep mid-write)
during a non-atomic write leaves a zero-byte or truncated file
that fails to parse, wedging the project until the user manually
`rm`s it. `WriteFileAtomic` writes to a temp file in the same
directory, fsyncs, and renames — POSIX guarantees rename
atomicity within a filesystem, so partial state is never visible.

**Where.** All six raioz-owned state files migrated in v0.9.0:

- `internal/state/project_state.go`
- `internal/state/global.go`
- `internal/state/workspace_preferences.go`
- `internal/state/service_preferences.go`
- `internal/ignore/ignore.go`
- `internal/workspace/active.go`

**Out of scope.** `internal/proxy/routes_persist.go` predates this
rule and uses its own atomic-write helper; it's scheduled to
migrate to `fsutil` in v0.9.1+ (proxy is outside ADR-029 scope, so
the migration is free). User-owned files (caches, logs) and
tool-managed files (mkcert certs — ADR-042) are NOT raioz state
and don't need this treatment.

**Enforcement.** Code review. A grep ratchet
(`grep -rn "os.WriteFile" internal/state internal/workspace
internal/ignore`) returning empty is the smoke test; if a future
state writer appears, the regression surfaces on the first chaos
test run (`internal/testing/chaos/atomic_write_chaos_test.go`).

## Consequences

### Positive

- Every hygiene rule has a discoverable home with rationale and
  enforcement mention.
- The threshold section gives reviewers a check when deciding
  whether to write a dedicated ADR or append here.
- Local issue drafts can be deleted on resolution without losing
  the institutional reason for the rule.

### Negative

- One more file to scan when looking for the rule that governs a
  given site. Mitigated by CLAUDE.md's one-paragraph entry still
  being the entry point — the catalog is the deep link, not the
  primary surface.
- Risk of catalog rot if a rule becomes a real tradeoff over time
  (e.g. an alternative emerges that's worth evaluating). When
  that happens, promote the catalog entry to a dedicated ADR and
  replace the catalog section with a one-line "Promoted to
  ADR-NNN".

### Neutral

- The two existing rules (H-1, H-2) are documented identically to
  how they read in CLAUDE.md today — no semantic change, just
  relocation of rationale.

## Alternatives considered

- **Keep "no ADR, paired with issue NNN" as-is.** Discarded — the
  issue drafts vanish on resolution, so the pairing is a dead
  link as soon as the rule ships.
- **One ADR per hygiene rule.** Discarded — the ceremony
  (Alternatives considered: "writing to disk non-atomically is
  bad") would be filler. The threshold section above is the
  honest answer to "when is a dedicated ADR worth it".
- **Inline the rationale into CLAUDE.md.** Discarded — would push
  the invariants section past the 200-line context-window
  truncation that already affects MEMORY.md.

## References

- CLAUDE.md § "Architectural invariants" — entry-point one-liners
  link here.
- ADR-046 — capability matrix that gates H-1 on runtimes lacking
  `HostGatewayAlias`.
- `internal/fsutil/atomic.go` — H-2 enforcement helper.
- `internal/testing/chaos/atomic_write_chaos_test.go` — H-2
  regression coverage.
