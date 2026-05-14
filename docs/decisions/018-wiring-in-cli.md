# ADR-018: Adapter wiring lives in `internal/cli/wiring.go`

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

Pre-ADR-018 layout:

- `internal/app/dependencies.go` declared the `Dependencies` struct
  AND held `NewDependencies()`, which constructed every adapter
  inline. The file imported `internal/infra/*` plus
  `internal/discovery` and `internal/proxy` directly.
- `NewDependenciesWithMocks` accepted only nine of the thirteen
  ports — `ProxyManager` / `DiscoveryManager` / `SnapshotManager` /
  `TunnelManager` were hard-coded into `NewDependencies` and never
  reached the mock path.
- Two adapters didn't have `internal/infra/` wrappers
  (`internal/discovery`, `internal/proxy`); they were imported
  directly from the constructor.

Consequence: importing any use case (e.g. `internal/app/upcase/`)
transitively pulled every adapter. Tests of the `app` package
couldn't avoid the entire dependency graph.

## Decision

1. **`internal/app/dependencies.go` owns the struct shape only.**
   It imports nothing under `internal/` except `domain/interfaces`.
   `NewDependencies()` is gone from this file.

2. **`internal/cli/wiring.go` owns the production wiring.** It
   constructs `*app.Dependencies` from concrete adapters under
   `internal/infra/*` and is the only file in the binary that
   imports every infra package. CLI command files call the
   package-local `newDependencies()` instead of
   `app.NewDependencies()`.

3. **Every adapter has an `internal/infra/` home.** Issue 038 ships
   two new thin wrappers — `internal/infra/discovery` and
   `internal/infra/proxy` — so `wiring.go` imports nothing under
   `internal/<concrete>` for adapters. The wrappers exist for
   layering uniformity, not behavior; they pass through to
   `internal/discovery` and `internal/proxy` unchanged.

4. **`NewDependenciesWithMocks` accepts all thirteen ports.**
   Tests that previously couldn't mock proxy/discovery/snapshot/
   tunnel now pass them positionally (or `nil` when not exercised).

## What didn't move

- Use cases under `internal/app/<usecase>/` still import the
  concrete packages they consume directly. The criterion this ADR
  attacks is "the constructor doesn't drag adapters" — narrower
  than "every app file is import-pure." Cleaning up the wider
  import surface is a separate effort and would have to thread
  through the dozen use-case packages.
- The `cli/wiring_test.go` companion test asserts every field of
  the constructed Dependencies is non-nil. It's the regression
  guard against forgetting to wire a new port.

## Implementation status

Landed in this commit:

- `internal/app/dependencies.go` reduced to the struct +
  `NewDependenciesWithMocks` (thirteen-port signature). Imports
  only `internal/domain/interfaces`.
- `internal/cli/wiring.go` contains `newDependencies()`.
- `internal/cli/wiring_test.go` checks every port wires up.
- `internal/infra/proxy/manager_impl.go` wraps `internal/proxy`.
- `internal/infra/discovery/manager_impl.go` wraps
  `internal/discovery`.
- Twenty-eight CLI files updated from `app.NewDependencies()` to
  the package-local `newDependencies()` via a mechanical sed +
  `goimports`.
- `graph.go` and `wiring.go` added to the CLI thin-viz exempt list
  (ADR-017). `graph.go` no longer needs `internal/app` because it
  doesn't reference any use case; `wiring.go` is structural.

## Consequences

### Positive

- `internal/app/dependencies.go` is a pure contract file — six
  exports, one import. Future contributors reading it learn the
  shape of `Dependencies` without seeing how it's wired in
  production.
- New tests can mock any subset of the thirteen ports. The
  previous "only nine ports were mockable" wart is gone.
- `internal/cli/wiring.go` is the single place to find what runs
  in production. Add a port, wire it here, done.
- `internal/infra/{proxy,discovery}` joining the wrapper set means
  every adapter follows the same `internal/infra/<name>` shape.

### Negative

- Two new wrapper files (`infra/proxy`, `infra/discovery`) that
  literally pass through. The cost is the cognitive uniformity:
  knowing every adapter sits under `internal/infra/` is more
  valuable than saving the four lines.
- `NewDependenciesWithMocks` now takes thirteen positional
  arguments. The downside is real but bounded; callers that don't
  use a port pass `nil`. A future "builder/struct literal" form
  could clean this up if the call sites grow further.

### Neutral

- `app.NewDependencies()` is gone from the public API surface.
  Every caller in this repo is inside `internal/cli/`, which now
  uses `newDependencies()`. External consumers (none in tree)
  would notice; we don't.

## Alternatives considered

- **Keep `NewDependencies` in `app/` but move imports to a
  build-tag-gated init file.** Compiles to the same binary, but
  the file shape becomes weird and lint can't enforce the boundary
  cleanly. Rejected.
- **Make `NewDependenciesWithMocks` accept an
  `app.DependenciesBuilder` instead of positional ports.**
  Cleaner ergonomics. Out of scope for this ADR — the call sites
  haven't bitten anyone yet.
- **Don't add `infra/proxy` and `infra/discovery` wrappers.**
  Would leave two adapters without an infra home. The wiring file
  would import `internal/proxy` directly, breaking the rule we
  just codified. Rejected.

## References

- Code: `internal/app/dependencies.go`,
  `internal/cli/wiring.go`,
  `internal/cli/wiring_test.go`,
  `internal/infra/proxy/manager_impl.go`,
  `internal/infra/discovery/manager_impl.go`.
- Related: ADR-014 / 015 / 016 / 017 (the use-case + thin-CLI
  pattern this ADR completes).
- Issue: 038.
