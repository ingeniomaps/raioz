# ADR-041: Meta orchestrator (`kind: meta`) — out-of-process sequential spawn

- **Status:** Accepted
- **Date:** 2026-05-16

## Context

The `MetaRunner` in `internal/app/meta.go` orchestrates a meta-orchestrator
config (a `raioz.yaml` with `kind: meta`) by delegating `up` / `down` /
`status` to N sub-projects. Several later ADRs assume specific
behaviour of this runner:

- ADR-037 (replaceable edge router) — the router project is brought up
  first by the meta runner; consumer sub-ups receive
  `RAIOZ_ROUTER_ACTIVE=1` in their env so they suppress the bundled
  Caddy.
- ADR-038 (dual-flow JSON deprecation) — `SelectFlow` decisions live in
  the same surface that the meta runner uses to invoke children.
- ADR-040 (sibling mode-A trust transitive) — the audit transitive
  trust analysis treats the meta runner's child-spawn surface as a
  trust hop.
- v0.8.2 `--audit-siblings` preflight + lifecycle audit events use the
  meta runner as their attachment point.

Until this ADR, the decisions that make the meta runner work were
only encoded as a doc comment on `MetaRunner` (`meta.go:21-31`). The
next architectural change that touches parallelism or partial-failure
semantics had no anchor to argue against.

## Decision

The meta runner operates as **one out-of-process sub-spawn per
project, sequentially, with documented per-sub-command failure
semantics**. The seven load-bearing rules:

1. **Out-of-process spawn.** Each sub-project runs in a fresh
   `raioz` process invoked via `MetaRunner.resolveBinary()`
   (`m.Binary` → `os.Executable()` → `filepath.Abs(os.Args[0])`,
   refusing the fallback under `testing.Testing()`). The runner
   never re-uses in-process use cases. Reasons: clean config
   loader / i18n state / naming prefix per sub, and automatic
   failure isolation (a panic in one sub cannot drag the meta
   runner down).

2. **Sequential, declared order.** Sub-projects run in the order
   they appear in `projects:`. No parallel spawn. When
   `cfg.Router` is non-nil and `opts.RouterOff` is false, the
   router project runs **first** on `up` and **last** on `down`;
   `status` reports the router first. Parallelism is rejected
   because the existing per-project lock contract (workspace lock
   + project lock) is not designed for concurrent acquirers
   started by the same parent.

3. **Per-sub-command failure semantics.** Fixed contract:
   - `up`: hard-fail on the first non-optional sub failure (the
     remaining subs do not run). Sub entries marked `optional:
     true` that fail emit a warning + audit event and are
     recorded as `Skipped` in the summary.
   - `down`: always best-effort. Every sub is attempted; failures
     warn + audit but never abort.
   - `status`: same best-effort as `down` — a missing or failing
     sub does not blank the rest of the report.

4. **No meta-level lock.** The meta runner does not acquire any
   workspace or project lock; each child takes its own. The meta
   process is just a coordinator. Consequence: a SIGKILL on the
   meta parent leaves N children holding their per-project locks.
   See ADR-026 (Pdeathsig) for the Linux cleanup story and
   `docs/LOCKS.md § "Meta runner sits outside both locks"` for
   the cross-platform floor (24h age cap via the lock package).

5. **Env propagation is the public contract.** Sub-spawns inherit
   `os.Environ()` verbatim plus a small set of raioz-managed
   signaling vars (the only mutation):
   - `RAIOZ_ROUTER_ACTIVE=1` — set on consumer sub-ups when a
     router is up. Declared in `internal/protocol.RouterActive`.
   - `RAIOZ_SIBLING_STACK` — appended with the parent path before
     spawn. Declared in `internal/protocol.SiblingStack`.
   - `RAIOZ_CORRELATION_ID` — propagated unchanged so audit
     events from sub-spawns share the parent's correlation.
     Declared in `internal/protocol.CorrelationID` (ADR-008,
     ADR-024).
   `docs/SECURITY.md § "Meta env inheritance"` documents the
   threat model around the verbatim `os.Environ()` propagation.

6. **`Optional + subCmd=="up"` is the only "skip" path.** No
   other field, flag, or env var controls partial failure
   tolerance. This keeps the failure matrix small enough to
   document in one paragraph (above).

7. **Lifecycle audit events.** `MetaRunner.Up` / `Down` / `Status`
   emit `LogLifecycleStart` + deferred `LogLifecycleComplete`
   from `internal/app/meta_audit.go`. Per-sub failures (optional
   skip OR best-effort `down`/`status`) emit their own
   `meta_sub_<cmd>` audit event with `best_effort: true`.

## Consequences

### Positive

- Future ADRs that touch meta semantics (router V2, transitive
  audit, parallel sub-spawn proposals) have a concrete anchor
  instead of doc comments.
- The failure matrix is small, predictable, and grep-able from a
  single ADR.
- Failure isolation is by construction: process boundary +
  declared per-cmd contract.

### Negative

- No parallelism — large meta workspaces with N slow sub-ups pay
  the sum of their times. ADR re-open candidate if a concrete
  use case surfaces.
- No per-sub timeout in v1 of this ADR — see issue 042 for the
  follow-up that adds `RAIOZ_META_SUB_TIMEOUT`.
- Pdeathsig (ADR-026) is Linux-only; Windows + macOS rely on the
  24h lock age cap as the cross-platform cleanup floor. See
  issue 030 for the multi-machine angle.

### Neutral

- The out-of-process choice rules out fast feedback for tiny
  sub-projects that could complete in milliseconds. Not a
  concern in practice — meta is for genuinely separate
  projects, not loop iteration.

## Alternatives considered

- **In-process delegation (no fork).** Rejected for the
  global-state contamination (config loader, i18n, naming) and
  the loss of automatic panic isolation.
- **Parallel sub-spawn.** Rejected for v1 of the meta runner —
  the per-project lock contract isn't designed for concurrent
  same-parent acquirers, and there is no concrete use case
  demanding it yet.
- **Meta-level workspace lock.** Rejected because each
  sub-project already takes its own project lock; a meta-level
  lock would force serialization across unrelated workspaces in
  the same meta dir.

## References

- Code: `internal/app/meta.go`, `internal/app/meta_audit.go`,
  `internal/app/upcase/router_env.go`,
  `internal/protocol/childenv.go`
- Related: ADR-008 (sibling spawn), ADR-024 (correlation ID),
  ADR-026 (Pdeathsig), ADR-037 (router project),
  ADR-038 (dual-flow drain), ADR-040 (transitive trust)
- Docs: `docs/LOCKS.md § "Meta runner sits outside both locks"`,
  `docs/SECURITY.md § "Meta env inheritance"`
- Follow-ups: issue 042 (per-sub timeout)
