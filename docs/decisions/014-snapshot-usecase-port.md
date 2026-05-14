# ADR-014: Snapshot lifecycle runs through a use-case port

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`raioz snapshot {create,restore,list,delete}` was a CLI-direct
command: `internal/cli/snapshot.go` imported the concrete
`internal/snapshot` package and called `snapshot.NewManager("").Create(...)`
inline. Every other CLI command goes through the
`cli → app/<case> → port` pattern that ADR-009 and ADR-012
formalize. Snapshot was the lone special case.

Concrete consequences:

- **Untestable without Docker.** Each of the snapshot commands ended
  up calling `internal/snapshot/exportVolume`, which shells out to
  `docker run alpine tar`. Tests had to skip the path or run Docker.
- **No port boundary.** Adding a feature (encrypted snapshots,
  bandwidth-limited restore, dry-run preview) required touching both
  CLI and the snapshot package in lockstep with no intermediate
  contract.
- **Reading the CLI lied about the architecture.** New contributors
  saw "snapshot" and concluded "this one's allowed to talk to
  snapshot directly," extending the exception.

## Decision

Lift snapshot to the standard three-layer pattern:

- **Port:** `interfaces.SnapshotManager` declares Create / Restore /
  List / Delete, plus the value types `Snapshot` and
  `VolumeSnapshot` (the metadata raioz records).
- **Use case:** `internal/app/snapshotcase` with one struct per verb
  (CreateUseCase, RestoreUseCase, ListUseCase, DeleteUseCase). Each
  takes a narrow `Dependencies` ({ConfigLoader, SnapshotManager}) so
  tests don't have to assemble the whole graph.
- **Adapter:** `internal/infra/snapshot.ManagerImpl` is a thin
  wrapper over `internal/snapshot.Manager` that satisfies the port.
  The Docker-shaped `tar` logic stays where it is — no behavior
  change.
- **CLI:** `internal/cli/snapshot.go` is wiring only. Each subcommand
  builds the use case from `app.NewDependencies()` and calls Execute.

A small wrinkle worth recording: snapshot only ever archives **infra**
volumes (services declare bind mounts, not named volumes). The volume
selection logic — walk `deps.Infra`, collect `Inline.Volumes`,
build a `volume → service` map — used to live in the CLI. It's now a
private helper in `snapshotcase` (`collectInfraVolumes`), keeping the
"which volumes count" decision next to the use case rather than at
the CLI boundary.

## Implementation status

Landed in this commit:

- `internal/domain/interfaces/snapshot.go` — port + value types.
- `internal/infra/snapshot/manager_impl.go` — adapter.
- `internal/app/snapshotcase/usecase.go` — four use cases.
- `internal/app/snapshotcase/usecase_test.go` — six tests, all
  driven by a local `mockSnapshotManager`. No Docker required.
- `internal/cli/snapshot.go` rewritten to thin wiring (~136 lines;
  the issue's "≤60 lines" target was per-subcommand-as-function, not
  per-file, so the actual structure is fine).
- `Dependencies.SnapshotManager` wired in
  `internal/app/dependencies.go`.

## Consequences

### Positive

- Snapshot is testable without Docker. The six tests in this commit
  cover the lifecycle paths plus error propagation.
- The CLI looks like every other CLI command, lowering the
  cognitive cost of reading the codebase.
- Future snapshot features (the encryption / dry-run / etc. ideas
  the issue alluded to) have an obvious home: extend the use case,
  add a method to the port, implement in the adapter.

### Negative

- Two new layers (port + use case) for what used to be ~20 lines per
  subcommand. The cost is paid up front; the ergonomics win starts
  with the next feature added on top.
- The `interfaces.Snapshot` and `interfaces.VolumeSnapshot` value
  types duplicate the shape of `internal/snapshot.Snapshot` /
  `VolumeSnapshot`. The adapter copies field-by-field. Justifiable
  because the port shouldn't leak the concrete package's types, but
  any field added in one place needs adding in the other and in the
  adapter's copy step. Treat the adapter's `convertSnapshot` like
  `cloneInfraEntry` (ADR-006): grep before tagging a release.

### Neutral

- The Docker-shaped logic in `internal/snapshot` (the
  `docker run alpine tar` calls) is untouched. Future work to move
  those to a Docker port would happen at the
  `internal/snapshot` ↔ `internal/docker` boundary, not at the port.

## Alternatives considered

- **Leave snapshot as a CLI-direct command.** Status quo. Preserves
  every problem listed in Context.
- **Skip the value type duplication; expose `internal/snapshot.Snapshot`
  through the port.** Lazier but pollutes the domain with an infra
  type and re-creates the ADR-009 problem we just spent an ADR
  fixing.
- **Roll Create/Restore/List/Delete into one `SnapshotUseCase` with
  a verb arg.** Half the boilerplate, but every other command (down,
  status, restart, …) uses the one-struct-per-verb pattern. Keep the
  consistency.

## References

- Code: `internal/domain/interfaces/snapshot.go`,
  `internal/infra/snapshot/manager_impl.go`,
  `internal/app/snapshotcase/`, `internal/cli/snapshot.go`.
- Related: ADR-009 (domain owns model types — same reasoning behind
  duplicating Snapshot/VolumeSnapshot), ADR-012 (port segregation
  pattern this commit follows).
- Issue: 034.
