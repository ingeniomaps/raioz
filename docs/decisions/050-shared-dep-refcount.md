# ADR-050: Refcount shared dependencies instead of scanning for live siblings

- **Status:** Accepted
- **Date:** 2026-06-05

## Context

Workspace-shared dependencies (`loki`, `jaeger`, a single `postgres`
serving several sibling projects) must outlive the `raioz down` of any
one consumer and only be torn down when the **last** consumer leaves.

Until now `raioz down` decided this with a heuristic
(`otherProjectsActiveInWorkspace`): scan the workspace for any
raioz-managed container belonging to a *different* project, and if one
exists, keep every shared dep alive. This is wrong in both directions:

- **False keep-alive.** A sibling that is up but does **not** consume the
  dep still counts as "someone is home", so the dep is kept running with
  zero real consumers. With several projects in a workspace, shared deps
  effectively never get torn down — the leak that motivated issue 069.
- **False teardown.** If a consumer's project label is missing or its
  containers are momentarily gone, the scan reports "nobody home" and
  rips the dep out from under a project that is still using it. The
  `DeferredToSibling` patch (issue #26) exists to paper over these edges.

The scan answers "is *anyone* live?" when the question that actually
gates teardown is "does *anyone reference this dep*?". Those are
different questions, and `LocalState` never persisted the mapping needed
to answer the second one.

## Decision

raioz keeps an explicit **reference count** of which projects consume
each shared dependency, in a new workspace-shared state file, and uses it
— not the live-container scan — to gate shared-dep teardown.

- **State.** `internal/refcount` persists
  `<RaiozStateDir>/shared-deps.json`, shape
  `{ "workspaces": { "<ws>": { "<dep>": ["<project>", ...] } } }`. The
  empty-string workspace key holds name-override deps declared outside a
  workspace. Writes go through `fsutil.WriteFileAtomic` (hygiene H-2);
  the read-modify-write cycle is serialized by a workspace-shared
  advisory file lock (`.shared-deps.lock`) plus a per-process mutex, the
  same belt-and-suspenders the proxy uses (ADR-010), because sibling
  projects mutate the file from separate processes.
- **up** records a reference for every shared dep it dispatched
  (`registerSharedDepRefs`, `internal/app/upcase/refcount_wiring.go`).
  `AddRef` is idempotent, so a repeated `up` does not double-count.
- **down** reconciles the workspace's refs against the projects that are
  actually live (`liveProjectsInWorkspace`), which drops the leaving
  project's refs and purges any left by a sibling that died without a
  clean down. It then tears a shared dep down only when its reconciled
  ref set is empty; otherwise it logs the remaining consumers and keeps
  the dep alive. Selective `raioz down <dep>` drops just that one
  reference and keeps the dep up while any still-live sibling references
  it.
- **Reconciliation is the safety net, not the source of truth.** The
  live-container scan that used to *decide* teardown is demoted to
  *reconciling* the refcount — it only removes references to projects
  that are provably gone. A shared dep stays up while ≥1 live project
  references it; the last consumer's `down` frees it. New shared-dep
  state under `RaiozStateDir()` inherits the ADR-023 contract: up writes,
  down deletes (the file is removed once empty).

This supersedes `otherProjectsActiveInWorkspace` as the teardown gate;
the function is gone and its boolean question is no longer load-bearing.

## Consequences

### Positive

- Shared deps no longer leak after `down`: the last consumer out frees
  them, reliably, without manual `docker compose -p … down`.
- The keep-alive decision is correct per-dep, not per-workspace — a live
  sibling that does not use a dep no longer pins it.
- Dirty downs self-heal: a crashed project's stale refs are reconciled
  away on the next `up`/`down` instead of pinning the dep forever.

### Negative

- Adds a new cross-process state file and lock. A bug in the lock or a
  corrupt file degrades teardown back toward the old best-effort
  behavior (refs missing → reconcile against live containers), which is
  no worse than before but is one more moving part.
- The refcount and the running containers can disagree transiently (e.g.
  between a SIGKILL and the next reconcile); the reconcile pass against
  live containers is what bounds that window.

### Neutral

- The empty-string workspace bucket tracks name-override deps that have a
  single owner in practice — uniform handling, slightly more state than
  strictly needed for that case.

## Alternatives considered

- **Keep the live-container scan, just fix the labels.** Doesn't address
  the false-keep-alive case: a live sibling that doesn't use the dep
  still reads as "someone home". The scan answers the wrong question.
- **Store the refcount in per-project `LocalState`.** It must survive a
  consumer's `down` to answer "does anyone *else* reference this", so it
  has to be workspace-shared, not per-project.
- **Extend `raioz.root.json`.** That file is per-project and is deleted
  on a full down (ADR-023); the refcount needs independent
  workspace-shared lifecycle, so it gets its own file.

## References

- Code: `internal/refcount/refcount.go`,
  `internal/app/upcase/refcount_wiring.go:22`,
  `internal/app/down_deps.go:29` (`stopDependencyComposeProjects`),
  `internal/app/down_selective.go:159` (`stopSelectiveDep`)
- Related: ADR-002 (shared deps workspace-scoped), ADR-010 (workspace
  lock), ADR-022 (unified state paths), ADR-023 (state mirrors reality),
  ADR-047 §H-2 (atomic state writes)
- Issue: `docs/issues/069-down-no-tumba-dependencies-shared.md`
