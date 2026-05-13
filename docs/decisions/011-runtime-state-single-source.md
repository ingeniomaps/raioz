# ADR-011: LocalState is the single source of truth for runtime state

- **Status:** Accepted
- **Date:** 2026-05-13

## Context

Two state mechanisms coexist in `internal/state/`:

1. **Legacy whole-Deps snapshot.** `state.Save(ws, deps)` serializes
   the entire `*models.Deps` (everything the user declared in
   `raioz.yaml` plus everything raioz inferred) to `.state.json` after
   each `up`. `state.Load(ws)` reads it back. The infra adapter
   `internal/infra/state/manager_impl.go` wires both into the
   `StateManager.Save/Load/Exists` ports the app layer consumes.

2. **Minimal `LocalState`.** `state.LocalState` is a small struct
   (`.raioz.state.json` in the project dir) that holds only fields
   raioz cannot recover by re-reading Docker labels or the user's
   `raioz.yaml`: dev overrides, host PIDs, deferred-to-sibling deps,
   ignored-services list.

The whole-Deps snapshot duplicates information the system already
knows. Concretely:

- Docker labels (ADR-001) carry workspace, project, and service
  identity. `docker.IsProjectActive` is the canonical "is project
  up?" probe.
- `raioz.yaml` is the user's declaration; rereading it on every
  command is the source of truth for what they wanted.
- The snapshot drifts whenever `models.Deps` grows a field
  (ADR-006's clone-sync hazard manifests here too).
- The snapshot drifts whenever the user kills a `raioz up` between
  `state.Save` and the next command — the file then describes a
  state Docker never reached.

Wave 2 collects three issues against this duplication:

- **029 (this ADR).** Mark the legacy API `// Deprecated:` and add a
  linter that prevents new callers from creeping in.
- **030.** Migrate the first half of existing callers
  (read-only paths: list, status, exec, logs, volumes) to derive
  what they need from Docker + raioz.yaml directly.
- **031.** Migrate the remaining callers (up/down lifecycle) and
  delete the legacy API.

## Decision

`LocalState` is the only state runtime raioz writes. The legacy
`state.Save / state.Load / state.Exists` (and the
`StateManager.Save/Load/Exists` interface methods that wrap them) are
deprecated and frozen — no new callers. Each existing caller is
classified into one of three buckets by issues 030 and 031:

- **Derivable from Docker labels.** Replace
  `StateManager.Load(ws).Project.Name` with a label-based lookup. The
  same applies for "is the project up?", "which services are
  running?", "which network is this project on?".
- **Derivable from `raioz.yaml`.** Replace
  `StateManager.Load(ws).Services` with a fresh `config.LoadYAML`
  call. Saves nothing; we keep one source of truth.
- **Genuinely runtime-only.** Lift the field into `LocalState` if it
  cannot be recovered otherwise (dev overrides, host PIDs,
  ignored-services list, sibling deferrals).

The `Exists` ports are removed in 031 in favor of
`docker.IsProjectActive` for the "is project up?" question.

## Consequences

### Positive

- One file format raioz writes (`.raioz.state.json`) and one
  reader/writer pair (`LoadLocalState / SaveLocalState`). The
  drift class disappears at the source.
- `make check-state-legacy` enforces the boundary mechanically. A
  reviewer no longer has to remember "did this PR introduce a new
  `state.Save`?".
- `LocalState` carries only what the documentation actually
  describes ("dev overrides, host PIDs, deferrals"). Future
  contributors see a small file with three fields, not a frozen
  copy of every `Deps` field that ever existed.
- The struct-mirror hazard (ADR-006) shrinks: `cloneInfraEntry` and
  `cloneService` only have to track the live `models.Deps` shape,
  not also a serialized snapshot of an older shape.

### Negative

- The migration touches ~15 production files (the whitelist in
  `scripts/lint-state-legacy.sh`). Each move-to-LocalState carries
  small regression risk — the lint guard keeps the pace measured.
- Some commands (`raioz status`, `raioz logs`) currently work even
  when raioz.yaml is missing or moved, because they read the
  snapshot. Post-031 they will require the YAML; the change is
  reasonable but visible.
- Removing `Exists` means `raioz down` of a stopped project must
  ask Docker first, costing a small extra call versus a file stat.
  Acceptable given the correctness win.

### Neutral

- `LocalState` already carries `DeferredToSibling` (ADR-008) and
  `DevOverrides`, so the "what stays in LocalState" decision is
  partly already made; this ADR codifies it.

## Alternatives considered

- **Keep both APIs.** The "legacy is mostly fine" position. Rejected
  because every new caller is an opportunity to encode an invariant
  the file doesn't actually own; the failure mode is "rare but
  scary" (state diverges, raioz down skips a container, the user
  has to clean up by hand). The cost of phasing the legacy out is
  bounded and well-understood; the cost of letting it grow is not.
- **Compress `state.json` to "only what's needed."** Halfway
  between the legacy and `LocalState`. Discarded because it still
  duplicates *some* of `raioz.yaml` and leaves the question "is X
  in the file?" judgment-call rather than rule. Choosing exactly
  the same fields `LocalState` already has gets us there with a
  rename.
- **Move state to SQLite or a key-value store.** Overkill for the
  3-field projection we end up needing. Adds a runtime dependency
  for negligible benefit.

## References

- Code: `internal/state/state.go` (deprecated functions, Save/Load/Exists),
  `internal/state/project_state.go` (LocalState),
  `internal/domain/interfaces/state.go` (deprecated interface methods),
  `scripts/lint-state-legacy.sh` (regression guard).
- Related: ADR-001 (containers identified by labels — the
  derivation source), ADR-006 (clone-sync hazard — same risk class),
  ADR-008 (sibling deferral state — a `LocalState` field).
- Originated from: architecture review CRITICAL #3 ("dual state
  systems") and issue 029.
- Successor work: issues 030 and 031 perform the migration.
