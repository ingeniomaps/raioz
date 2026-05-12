# ADR-009: `internal/domain/models/` holds only types with no infrastructure dependencies

- **Status:** Accepted
- **Date:** 2026-05-12

## Context

The original `internal/domain/models/` was a kitchen-sink package of
type aliases pointing at infrastructure types:

```go
// Old internal/domain/models/config.go
package models
import "raioz/internal/config"
type (
    Deps    = config.Deps
    Service = config.Service
    // ... 13 more aliases ...
)
```

This had three problems documented in the architecture review:

1. **It lied about layering.** The aliases imported `internal/config`,
   `internal/state`, `internal/host`, `internal/workspace` — concrete
   infrastructure — so anyone importing `domain/interfaces` transitively
   pulled all of them. The clean-arch promise that "domain depends on
   nothing under it" was nominal at best.
2. **It blocked obvious refactors.** Trying to move `Runtime` and
   `DetectResult` to `domain/models` so the port interfaces could stop
   importing `internal/detect` created a cycle: `detect → models →
   config → detect` (config imports detect for its scanner functions).
3. **It hid the real dependency.** A reviewer reading
   `domain/interfaces/config.go` saw `models.Deps` and assumed the
   domain owned a clean abstraction. In reality the type was
   `config.Deps` — same struct, same package, with a forwarding name.

## Decision

`internal/domain/models/` holds **only** types whose definitions have no
internal package dependencies. Currently that means `Runtime` and
`DetectResult` (in `runtime.go`). They were genuine domain concepts that
the scanning code (`internal/detect`) just happens to produce; they were
declared in `detect` only because that's where they were first used.

Everything else — `Deps`, `Service`, `Infra`, `ProjectState`,
`GlobalState`, `Workspace`, `ProcessInfo`, etc. — stays in its source
package (`internal/config`, `internal/state`, `internal/workspace`,
`internal/host`). `internal/domain/interfaces/*.go` imports those
packages directly when the interface signatures need their types. No
forwarding aliases.

Concretely, after this decision:

- `internal/detect/result.go` re-exports `Runtime` and `DetectResult`
  as type aliases from `internal/domain/models` so existing scanner
  callers (`detect.Runtime`, `detect.RuntimeCompose`, ...) still work.
- `internal/domain/interfaces/discovery.go` and
  `internal/domain/interfaces/orchestrator.go` import
  `internal/domain/models` and use `models.Runtime` /
  `models.DetectResult` — they no longer depend on `internal/detect`.
- The other `domain/interfaces/*.go` files import `config`, `state`,
  `host`, `workspace` **directly** with package-qualified names like
  `config.Deps`, `state.ProjectState`, `host.ProcessInfo`,
  `workspacepkg.Workspace`. The dependency is now visible at the
  import block, not hidden behind a forwarding alias.

## Consequences

### Positive

- The dependency graph matches the prose. A contributor reading
  `domain/interfaces/config.go` sees `import "raioz/internal/config"`
  and knows immediately that this layer depends on the config package.
  No more "wait, what does `models.Deps` actually resolve to?"
- `domain/models` is now a real leaf in the dependency graph
  (`go list -deps` shows zero internal imports). Future genuinely
  domain-owned types — value objects, IDs, lifecycle states — have an
  honest home.
- The cycle that blocked moving `Runtime` to the domain is broken.
  `Runtime` is now genuinely a domain concept that infrastructure
  re-exports for ergonomics.

### Negative

- `domain/interfaces` explicitly depends on infrastructure packages.
  An architectural purist would say this violates Clean Architecture's
  "dependencies point inward" rule. The defense: the dependency was
  always there (via the forwarding aliases); we just stopped hiding it.
  If we want to actually invert the dependency, we need to move the
  struct definitions (not just rename the imports). That is a separate,
  much larger refactor — explicitly out of scope here.
- `domain/interfaces` now imports more packages directly. Each
  interface file picks up two or three new imports. Tests calling those
  interfaces don't see any change in API shape.

### Neutral

- Test mocks in `internal/mocks/` continue to satisfy the interfaces
  unchanged. Type aliases preserve method sets; concrete types in
  `config`/`state`/`host`/`workspace` continue to satisfy interfaces
  that reference them.

## Alternatives considered

- **Move all struct definitions into `domain/models`.** Would deliver
  true Clean Architecture layering. Discarded for now: it requires
  moving ~16 structs and their methods, then updating ~60 call sites.
  6+ hours of focused work with a high regression risk because the
  affected types are central to config and state. Worth doing if and
  when we have a longer planning window; ADR-009 does not preclude it
  (a future ADR would supersede this one).
- **Rename `domain/models` to `domain/typealiases` and keep the
  aliases.** Honest naming but preserves the cycle problem and
  doesn't actually fix the dependency-direction issue.
- **Define `Runtime` and `DetectResult` in a separate leaf package
  like `internal/domain/scan/`.** Would also work but adds a package
  whose role would be hard to distinguish from `domain/models`. Keeping
  them in `models` matches their conceptual role.

## References

- Code: `internal/domain/models/runtime.go`,
  `internal/detect/result.go`,
  `internal/domain/interfaces/{config,docker,env,git,host,state,workspace,discovery,orchestrator}.go`
- Related: ADR-006 (clone-sync — same struct-mirroring pattern is in
  scope for the larger refactor a future ADR may pursue)
- Originated from: architecture review findings CRITICAL #1 and CRITICAL #2.
