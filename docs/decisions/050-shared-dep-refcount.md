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
- **Shared deps use a single workspace-scoped compose project.** A
  workspace-shared dep is scoped to `{prefix}-dep-{dep}`
  (`naming.SharedDepComposeProjectName`), dropping the per-project
  segment of `DepComposeProjectName`. Per-project deps keep the segment
  (so `--remove-orphans` never sweeps another project's same-named dep).
  Without this, the first consumer creates the dep under *its* compose
  project and the last consumer's `down` — scoped to a *different*
  per-project name — finds nothing and leaks the container. Threaded via
  `interfaces.ServiceContext.SharedDep`, set by `up` and consumed by
  `ImageRunner` (create) and the down paths (`-p` teardown).
- **up** records a reference for every shared dep it dispatched
  (`registerSharedDepRefs`, `internal/app/upcase/refcount_wiring.go`).
  `AddRef` is idempotent, so a repeated `up` does not double-count.
- **down** drops only the leaving project's own reference
  (`DropRef`) and tears a shared dep down only when no reference
  remains; otherwise it logs the remaining consumers and keeps the dep
  alive. Selective `raioz down <dep>` does the same for that one dep.
- **The down decision trusts the refcount directly — it does NOT
  re-derive liveness from a container scan.** This is the load-bearing
  correction: a sibling that consumes *only* shared deps owns no
  project-labeled container (shared deps omit `com.raioz.project`,
  ADR-002), so any "is project X still live?" scan reads it as gone and
  would rip the dep out from under it. The persisted refcount is the
  only signal that survives a consumer's `down` and sees such a project,
  so it is the source of truth. New shared-dep state under
  `RaiozStateDir()` inherits the ADR-023 contract: up writes, down
  deletes (the file is removed once empty).

This supersedes `otherProjectsActiveInWorkspace` as the teardown gate;
the function is gone and its boolean question is no longer load-bearing.

**Failure mode and its bound.** Because the decision trusts the file, a
consumer that is hard-killed (SIGKILL, crash) without a clean `down`
leaves a stale reference that keeps its shared deps pinned until it is
cleaned (a later clean `down` of that project, or `raioz clean`). This is
the deliberately chosen safe direction: a pinned dep is recoverable,
whereas tearing a dep out from under a live consumer breaks a running
project. An automatic container-scan reconcile **on the `down` hot path**
was implemented and then removed precisely because it cannot tell a
hard-killed project from a live shared-deps-only one — a smoke test caught
it tearing down a dep that a live sibling was still using.

**`raioz clean` is the escape hatch — and now acts on it (issue 020).**
The reconcile that is unsafe on the `down` hot path is acceptable in
`clean`, which is explicit, user-initiated maintenance: a wrongly-pruned
ref is re-created by the next `up`, so the blast radius the smoke test
caught on `down` does not apply. `clean` therefore walks the refcount
(`internal/app/clean_refcount.go`), probes each referencing project with
`docker.IsProjectActive` (running-only, no fail-open), drops the refs of
projects that are not running, and tears down any dep left with zero live
references. It is conservative on uncertainty: a probe that errors keeps
the ref. The one residual hazard — a live consumer that uses ONLY shared
deps owns no project-labeled container and so reads as "not running" — is
the same edge that kept this off the `down` path; in `clean` it is bounded
to a re-`up` and surfaced in the action list. Before this, `clean` never
touched the refcount, so the only remedy for a stale ref was editing
`shared-deps.json` by hand. The pinned-dep leak is also no longer silent:
`down` now warns when it keeps a shared dep alive (issue 020, fix B).

## Consequences

### Positive

- Shared deps no longer leak after a clean `down`: the last consumer out
  frees them, reliably, without manual `docker compose -p … down`.
- The keep-alive decision is correct per-dep, not per-workspace — a live
  sibling that does not use a dep no longer pins it, and a sibling that
  consumes only shared deps is no longer invisible to the decision.
- It never tears a shared dep out from under a live consumer, because the
  decision trusts the persisted refs rather than a container scan that
  cannot see shared-deps-only consumers.

### Negative

- Adds a new cross-process state file and lock — one more moving part. A
  corrupt or unreadable file would mis-gate teardown; the atomic write +
  lock keep that from happening on a clean process exit.
- A hard-killed consumer leaves a stale ref that pins its shared deps
  until cleanup. No automatic reconcile heals this, because no reliable
  cross-process liveness signal exists for shared-deps-only consumers
  (see "Failure mode" above). `raioz clean` is the escape hatch.

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

- Code: `internal/refcount/refcount.go` (`Snapshot` / `Workspaces`
  enumerate for the clean-time GC),
  `internal/app/upcase/refcount_wiring.go:22`,
  `internal/naming/naming.go` (`SharedDepComposeProjectName` /
  `DepComposeProjectNameFor`),
  `internal/app/down_deps.go:29` (`stopDependencyComposeProjects`,
  reports kept-alive deps — issue 020 fix B),
  `internal/app/down_selective.go:159` (`stopSelectiveDep`),
  `internal/app/clean_refcount.go` (clean-time stale-ref GC — issue 020
  fix A)
- Related: ADR-002 (shared deps workspace-scoped), ADR-010 (workspace
  lock), ADR-022 (unified state paths), ADR-023 (state mirrors reality),
  ADR-047 §H-2 (atomic state writes)
- Issue: `docs/issues/069-down-no-tumba-dependencies-shared.md`,
  `docs/issues/020-down-no-tumba-deps-shared-con-refcount-stale.md`
