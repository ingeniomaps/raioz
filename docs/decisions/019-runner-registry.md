# ADR-019: Runtime → runner dispatch via a package-init registry

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`internal/orchestrate.Dispatcher.selectRunner` used a 23-case switch
to map `models.Runtime` to one of four runners (compose, dockerfile,
host, image). Adding a runtime required two edits — `models.Runtime`
constants and the switch — and forgetting the second was a silent
failure mode: `raioz up` for a project with that runtime fell into
the `default:` branch and returned "unsupported runtime: X". Nothing
in the build, lint, or tests caught the gap.

## Decision

Replace the switch with a package-init registry:

- `internal/orchestrate/registry.go` declares
  `runnerRegistry map[models.Runtime]runnerSelector` and a
  `register(rt, sel)` helper that panics on duplicate registration.
- Each runner file (`compose_runner.go`, `dockerfile_runner.go`,
  `host_runner.go`, `image_runner.go`) calls `register` in its
  `init()`. `host_runner.go` registers all 21 host runtimes in one
  loop; the others register one runtime each.
- `Dispatcher.selectRunner(rt)` becomes a single map lookup.
- `models.AllRuntimes()` returns the canonical list of runtimes (all
  declared constants except `RuntimeUnknown`).
- `TestAllRuntimesHaveRunner` (in
  `internal/orchestrate/registry_test.go`) asserts the registry
  covers every entry of `models.AllRuntimes()`. A missing
  registration fails CI with the offending runtime named.
- `TestRegistryRejectsDuplicates` documents the panic-on-duplicate
  contract.

The exhaustiveness test is the new safety net. Adding a runtime now
requires three things, and the missing one fails the test:

1. Add the `Runtime` constant in `internal/domain/models/runtime.go`.
2. Add it to the slice in `models.AllRuntimes()`.
3. Call `register(rt, …)` from the relevant runner file's `init()`.

## Implementation status

Landed in this commit:

- `internal/domain/models/runtime.go` gains `AllRuntimes()`.
- `internal/orchestrate/registry.go` (new).
- `internal/orchestrate/{compose,dockerfile,host,image}_runner.go`
  each gain an `init()` that registers.
- `internal/orchestrate/orchestrate.go::selectRunner` replaced with
  a map lookup.
- `internal/orchestrate/registry_test.go` (new) carries the two
  guards.

## Consequences

### Positive

- One-step add: a new runtime needs one register() call. The
  exhaustiveness test is the safety net.
- The switch fan-out is gone; runner files now own their dispatch
  mapping. Reading `host_runner.go` reveals which runtimes route to
  it without grepping a switch in another file.
- Duplicate registration panics at package init, surfacing the
  programming error before the binary boots.

### Negative

- `init()` ordering. Go runs file-scope inits in alphabetical order
  within a package; within a file, top-to-bottom. The current
  registrations are independent (each runner registers only its own
  runtimes), so order doesn't matter. If a future runner ever needs
  to read the registry during its own init(), this becomes a foot-gun.
  Mitigated by the test catching missing entries, not by the design.
- The registry is package-global mutable state. Acceptable because
  the only writes happen at init() and `register` panics on
  conflict; reads are read-only after the package finishes loading.

### Neutral

- The closures captured in `host_runner.go`'s for loop all close
  over `d.host` (a field reference), not over `rt`. Go's loop-var
  capture semantics don't bite here because `rt` is passed as the
  first positional argument to `register`, not used inside the
  closure body.

## Alternatives considered

- **Generate the dispatch table from a single source.** `go
  generate` with a `//go:generate` comment that emits both the
  Runtime constants and the registry entries. Heavier; would force
  a `go generate` step on every contributor. Rejected.
- **Keep the switch and add a separate exhaustiveness test that
  inspects the switch AST.** Stricter than the registry approach
  (couldn't even forget to register one) but a lot more code than
  this fix. Revisit if the simpler test starts missing things.
- **Move the switch logic to `models.Runtime.Dispatch(d *Dispatcher)`
  as a method.** Couples the domain Runtime to orchestrate's
  Dispatcher type. Rejected on ADR-009 (domain layer doesn't
  depend on infra/app types).

## References

- Code: `internal/domain/models/runtime.go`,
  `internal/orchestrate/{registry,orchestrate,compose_runner,
  dockerfile_runner,host_runner,image_runner,registry_test}.go`.
- Related: ADR-009 (domain owns Runtime), issue 039.
