# ADR-002: Shared deps are workspace-scoped; project label omitted

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented)

## Context

Multiple raioz projects in the same workspace often need the
same dependency (one Postgres for everyone, one Redis for
everyone). The naive approach — one container per project —
wastes resources and breaks expectations (data isn't shared
across projects in the same workspace).

But sharing brings a lifecycle question: if project A and
project B both depend on Postgres, who tears it down? Naive
"raioz down tears down what it created" means the first project
to leave kills Postgres while the second is still running.

## Decision

When `workspace:` is declared OR the user writes
`dependencies.<n>.name:`, the dependency is treated as
**workspace-shared**:

- Container name: `{workspace}-{dep}` (e.g. `acme-postgres`),
  not per-project. Produced by
  `naming.DepContainer(workspace, project, dep, override)`.
- Labels: `com.raioz.workspace=<ws>` is set, `com.raioz.service`
  is set, but `com.raioz.project` is **omitted**. The absence of
  the project label is the signal "I am not owned by any single
  project".
- `ImageRunner.Start` is idempotent: if a sibling project
  already created the container, `Start` returns without
  re-creating.
- `raioz down` skips containers missing `com.raioz.project`
  unless `otherProjectsActiveInWorkspace(workspace) == false`,
  i.e. the leaving project is the last one. The check looks at
  Docker labels, not a refcount state file.

## Consequences

### Positive

- One Postgres per workspace; data shared as expected.
- Lifecycle is correct under any ordering (concurrent ups,
  random down order). The last leaver tears the shared dep
  down.
- No refcount state file to corrupt.

### Negative

- Two projects in the same workspace that **want** independent
  Postgres instances can't get them via the standard YAML; they
  must use `dependencies.foo.name: pg-projectA` to break out.
- The "omitted label" signal is subtle — a contributor reading
  `down` logic for the first time has to know to look for
  absence, not presence.

### Neutral

- A project without `workspace:` and without `name:` overrides
  gets per-project containers as before.

## Alternatives considered

- **Refcount file** — fragile (every crash leaks the count);
  discarded.
- **Marker label `com.raioz.shared=true`** — slightly more
  explicit, but the absence-of-project pattern already covers
  every case raioz needs to distinguish.
- **Always per-project containers** — defeats the workspace
  abstraction.

## References

- Code: `internal/naming/labels.go` (`IsSharedDep`),
  `internal/orchestrate/image_runner.go`,
  `internal/app/upcase/orchestration.go`
  (`otherProjectsActiveInWorkspace`)
- Related: ADR-001 (label-based identity)
