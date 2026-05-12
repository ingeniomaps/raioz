# ADR-001: Container identity via labels, not names

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented; decision predates this file)

## Context

Early raioz identified its own containers by name prefix
(`raioz-<project>-<service>`). Two patterns broke that:

1. Users brought up dependencies via their own
   `docker-compose.yml` with `container_name:` set to a custom
   value — raioz couldn't find them.
2. Workspaces share dependencies across projects with names like
   `{workspace}-{dep}`, which don't carry the project name at
   all.

The result was containers that raioz had created but couldn't
sweep on `raioz down`, leaking until manually removed. BUG-2 in
the historical bug log.

## Decision

Every container raioz creates is stamped with a fixed set of
Docker labels. Lookups and sweeps filter by labels, never by
name prefix.

Label set (defined in `internal/naming/labels.go`):

- `com.raioz.managed = "true"` — always present.
- `com.raioz.kind = "service" | "dependency" | "proxy"`.
- `com.raioz.workspace = <workspace>` — when set; absent
  otherwise.
- `com.raioz.project = <project>` — omitted on workspace-shared
  deps (see ADR-002).
- `com.raioz.service = <name>` — service/dep/proxy name.

New runners that create containers **must** stamp these labels
or the containers will leak. The constants live in
`internal/naming/labels.go` and the helper `naming.Labels()`
returns the canonical map.

Enforcement: `make check-labels` (script
`scripts/lint-labels.sh`) fails if any production Go file
outside `internal/naming/` references `"com.raioz.*"` as a
string literal.

## Consequences

### Positive

- `raioz down` works regardless of container name (user-supplied
  compose, custom `name:` overrides, workspace-shared deps).
- Identity contract is one file (`labels.go`), greppable, lintable.
- Adding a new runner only needs `naming.Labels(...)` to
  participate in lifecycle.

### Negative

- Every Docker `inspect` for ownership questions has to parse the
  label map (slightly slower than a string prefix check). Not
  hot-path.
- New label keys are a breaking change for any external tool
  that reads them — keep the set small and stable.

### Neutral

- Container names are still deterministic and used for DNS
  resolution inside the network; only the **ownership question**
  shifts to labels.

## Alternatives considered

- **Stick with name prefix** — broke for user-supplied compose
  and shared deps; discarded.
- **State file with container ID list** — duplicates Docker as
  source of truth; goes stale when Docker is touched outside
  raioz (manual `docker rm`).
- **Single label `com.raioz.id = <uuid>`** — denies cross-cutting
  filters like "all containers of project X", which we need.

## References

- Code: `internal/naming/labels.go`, `internal/naming/labels_test.go`
- Lint: `scripts/lint-labels.sh`, `Makefile` target `check-labels`
- Related: ADR-002 (shared deps omit project label)
