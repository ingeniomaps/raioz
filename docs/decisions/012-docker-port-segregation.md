# ADR-012: Docker operations are split across six segregated ports

- **Status:** Accepted — Plan B implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`internal/domain/interfaces.DockerRunner` accumulated 46 methods over
the project's life: container lifecycle, docker-compose orchestration,
network/volume management, port validation, image checks, naming
helpers, and presentation primitives. Any test that needed to mock
"a piece of Docker" had to implement (or generate) the entire surface.
A cosmetic tweak to `FormatStatusTable` could break compilation in
packages that touch only `ContainerUp`.

The aggregate-port shape also hid which dependencies a use case
genuinely needed. Reading `uc.deps.DockerRunner` told the reader
"everything Docker can do is reachable from here" — accurate but
useless for understanding the SUT's real surface.

## Decision

Split the Docker surface into six segregated interfaces, each scoped
to one responsibility. `DockerRunner` is kept as an aggregate that
embeds all six so existing callers (~37 production files, ~88 total
including tests) keep compiling unchanged. New callers and tests
should reference the smallest interface they actually use.

The six interfaces, by concern:

- **`ContainerManager`** — low-level container ops (status probe,
  stop, label-based lookup, `IsProjectActive`).
- **`ComposeRunner`** — every `docker compose`-shaped operation
  (up, down, logs, exec, generate, restart, services info).
- **`NetworkManager`** — Docker network lifecycle (ensure, project
  membership, sweep unused).
- **`VolumeManager`** — Docker volume operations (ensure, sweep
  unused, named-volume derivation from deps, shared-volume
  detection).
- **`ImageValidator`** — pre-up image existence checks and
  unused-image sweep.
- **`PortValidator`** — port-conflict detection and active-port
  enumeration.

`DockerRunner` becomes:

```go
type DockerRunner interface {
    ContainerManager
    ComposeRunner
    NetworkManager
    VolumeManager
    ImageValidator
    PortValidator

    // Deprecated: presenters move to internal/output/.
    FormatStatusTable(...)
    FormatPortConflicts(...)
    FormatSharedVolumesWarning(...)
    // Deprecated: naming helpers move to internal/naming/.
    NormalizeVolumeName(...)
    NormalizeContainerName(...)
    NormalizeInfraName(...)
}
```

A compile-time proof + functional sample in
`internal/domain/interfaces/segregation_test.go` shows that a test
needing only `ContainerManager` can drive `MockDockerRunner` through
that narrow surface.

## Why Plan B over the full split

The issue (032) sketched two options:

- **Plan A — full split.** Delete `DockerRunner`; every caller imports
  the specific interface it needs. ~37 production files change, plus
  every test that constructs `Dependencies`. Cleaner end state but a
  2-3 day refactor with regression risk in the up/down hot path,
  without Docker integration tests in CI to catch behavior drift.

- **Plan B — aggregate embeds the six (this ADR).** The segregated
  contracts exist and are usable. Existing callers don't change.
  Migration to the narrow interfaces is opportunistic (each new test
  picks the narrowest fit) rather than a big-bang rewrite.

Plan B was picked because the architectural value (narrow mocking,
explicit dependencies) is captured the moment the interfaces exist;
the call-site rewrite is mostly cosmetic. The issue itself flagged
the >50-file threshold for falling back to Plan B; raioz crosses it.

## Implementation status

Landed in commit `<this commit>`:

- Six segregated interfaces in `internal/domain/interfaces/`.
- `DockerRunner` rewritten to embed them.
- Deprecation markers on `Format*` and `Normalize*` methods.
- Sample test in `internal/domain/interfaces/segregation_test.go`.

**Not landed in this ADR** (tracked for follow-up):

- **Move `FormatStatusTable` / `FormatPortConflicts` /
  `FormatSharedVolumesWarning` to `internal/output/`.** Each is
  already a free function in `internal/docker/` (`format.go`,
  `ports.go`, `volumes_shared.go`). Relocating them is mechanical
  but touches the `internal/docker` ↔ `internal/output` boundary,
  which deserves its own ADR follow-up rather than smuggling
  through here.
- **Move `NormalizeContainerName` / `NormalizeInfraName` /
  `NormalizeVolumeName` to `internal/naming/`.** Same story: the
  functions are self-contained inside `internal/docker/naming.go`,
  but `internal/docker` currently imports `internal/naming` (for
  `naming.Labels`), so the move requires either flipping the import
  direction or introducing a leaf-only sub-package. Out of scope
  for the Plan B minimum.
- **Migrate production callers to the narrow interfaces.** Today
  every caller still grabs `DockerRunner`. Migration is incremental:
  each new feature can take the narrower port, and old call sites
  flip when they're touched for unrelated reasons.

## Consequences

### Positive

- New tests can mock only what they consume. The
  `TestSegregatedInterfaceMocking` sample wires a single
  `IsProjectActiveFunc` into a `MockDockerRunner` and passes it as
  `ContainerManager` to a probe function — no need to populate the
  other 45 `*Func` fields.
- Reading a use case's `Dependencies` struct will eventually reveal
  which segregated surfaces it actually depends on (once migrations
  happen). Aggregating callers stay readable in the meantime.
- The deprecation markers on `Format*` / `Normalize*` make the
  separation-of-concerns intent visible to anyone reading the
  interface today.

### Negative

- The end state still has `DockerRunner` as a 6-way embed plus a
  handful of deprecated methods. A reviewer skimming the interface
  may wonder why the embed exists at all. Mitigated by this ADR and
  the comment block on `DockerRunner`.
- Plan A's win — "callers state exactly what they need" — only
  materializes as migrations happen. Plan B leaves the architecture
  capable but not yet realized.

### Neutral

- `MockDockerRunner` keeps its 46 `*Func` fields. Tests that
  populate the wrong subset for what the SUT actually calls
  continue to work as long as the called subset is set; the
  segregation contract doesn't change runtime behavior.

## Alternatives considered

- **Plan A (full split).** See "Why Plan B" above. Reconsidered when
  CI has real Docker integration tests, or when the migration to
  narrow interfaces is far enough along that deleting `DockerRunner`
  becomes a small change.
- **Keep `DockerRunner` aggregate, no split.** The status quo.
  Preserves the test-mocking pain that motivated 032. Rejected.

## References

- Code: `internal/domain/interfaces/{container,compose,
  network_manager,volume_manager,image_validator,port_validator,
  docker,segregation_test}.go`.
- Sample test: `internal/domain/interfaces/segregation_test.go`.
- Related: ADR-011 (this work introduced `IsProjectActive`, which
  becomes part of `ContainerManager`).
- Issue: 032.
