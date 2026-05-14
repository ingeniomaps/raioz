# ADR-009: `internal/domain/models/` owns the model types; infrastructure depends on it

- **Status:** Accepted
- **Date:** 2026-05-12 (initial scope) — **superseded by full migration 2026-05-13** (issue 023 completed).

## Context

The original `internal/domain/models/` was a thin package of type aliases
pointing at infrastructure types:

```go
// Pre-2026-05-12 internal/domain/models/config.go
package models
import "raioz/internal/config"
type (
    Deps    = config.Deps
    Service = config.Service
    // ... 13 more aliases ...
)
```

Three problems documented in the architecture review:

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

An interim 2026-05-12 decision moved only `Runtime` / `DetectResult` and
left `Deps`, `Service`, `Infra`, `ProjectState`, etc. in their source
packages. That broke the cycle on the smallest possible surface but
preserved the "interfaces depend on infrastructure directly" honesty
trade-off. It was explicitly framed as an interim step.

## Decision

**Issue 023 (2026-05-13) completed the larger migration the interim ADR
had postponed.** All model struct definitions live in
`internal/domain/models/`. Infrastructure packages depend on the domain,
not the other way around.

Concretely:

- `internal/domain/models/` is the canonical home for `Deps`, `Service`,
  `Infra`, `InfraEntry`, `EnvValue`, `NetworkConfig`,
  `HealthcheckConfig`, `ProxyConfig`, `RoutingConfig`, `YAMLWatch`,
  `MockConfig`, `FeatureFlagConfig`, `PublishSpec`,
  `ServiceProxyOverride`, `SourceConfig`, `DockerConfig`, `Project`,
  `ProjectCommands`, `EnvironmentCommands`, `EnvConfig`,
  `ServiceCommands`, `MissingDependency`, `DependencyConflict`,
  `LocalState`, `DevOverride`, `GlobalState`, `ProjectState`,
  `ServiceState`, `ServiceInfo`, `ConfigChange`, `AlignmentIssue`,
  `ServicePreference`, `ServicePreferences`,
  `WorkspaceProjectPreference`, `WorkspacePreferences`, plus the
  pre-existing `Runtime` and `DetectResult`. Methods on those structs
  (`Deps.GetWorkspaceName`, `LocalState.MarkDeferred`,
  `FeatureFlagConfig.IsEnabled`, custom JSON / YAML
  marshalers, ...) live with the types.

- `internal/domain/interfaces/*.go` imports only
  `raioz/internal/domain/models` for its types. It still references
  `Workspace` (via `workspacepkg.Workspace`) and `ProcessInfo` (via
  `host.ProcessInfo`) — those are infrastructure value types the
  interface layer needs to talk about, and moving them is a separate
  exercise. Neither pulls in `config`, `state`, or `detect`.

- `internal/config/*.go`, `internal/state/*.go`, and
  `internal/detect/result.go` keep **type aliases** for the migrated
  structs (`type Deps = models.Deps`, etc.). The aliases are kept to
  avoid forcing every test fixture under those packages onto the new
  spelling in one PR. Production callers across `internal/app`,
  `internal/cli`, `internal/orchestrate`, `internal/proxy`, etc. were
  rewritten in this issue to reference `models.X` directly.

- `internal/detect/result.go` no longer re-exports `Runtime` and
  `DetectResult` — every production caller uses `models.Runtime` /
  `models.RuntimeCompose` etc. directly.

- `internal/workspace.Resolve` used to import `internal/config` for the
  legacy `.raioz.json` migration step. That dependency is now injected
  via a `LoadDepsForMigration` function variable, set by
  `internal/infra/workspace/manager_impl.go` in `init()`. `workspace`
  remains a leaf used by domain interfaces without transitively pulling
  `config` into the domain dependency graph.

The dependency graph for the domain layer is now:

```bash
$ go list -deps raioz/internal/domain/... \
    | grep raioz/internal/ \
    | grep -v raioz/internal/domain/
raioz/internal/env
raioz/internal/errors
raioz/internal/exec
raioz/internal/git
raioz/internal/host
raioz/internal/logging
raioz/internal/path
raioz/internal/resilience
raioz/internal/workspace
```

Notably **absent**: `raioz/internal/config`, `raioz/internal/state`,
`raioz/internal/detect`, `raioz/internal/infra`.

## Consequences

### Positive

- Clean Architecture's "dependencies point inward" rule is no longer
  nominal. Domain owns its types; infrastructure depends on the domain
  layer to know what types it operates on.
- `go list -deps raioz/internal/domain/...` is a one-line CI assertion
  that the boundary holds. Adding a stray `import "raioz/internal/config"`
  anywhere reachable from `domain/` shows up immediately.
- The `Runtime → DetectResult → config → detect` cycle that motivated
  this work is broken at the root: `models` has no internal deps at
  all.
- Aliases in `config`/`state` give us a controlled deprecation path for
  the spelling `config.Deps` / `state.LocalState` — tests can migrate
  later without blocking this refactor.
- The `LoadDepsForMigration` hook pattern is reusable for any future
  "domain-friendly package needs an optional legacy IO path" situation.

### Negative

- The migration touched ~260 files. The diff is mostly mechanical
  (`config.X` → `models.X`) but it does show up in `git blame` for code
  whose semantics didn't change. Mitigated by doing it in a single
  commit per ADR-006's "atomic struct-mirror" pattern.
- The type aliases in `internal/config/{deps,deps_types,yaml_aux_types,
  yaml_types,flags,dependency_assist}.go` and
  `internal/state/{project_state,global,diff,check,service_preferences,
  workspace_preferences}.go` add a small amount of indirection for
  anyone reading those packages. Each alias file carries a one-line
  comment pointing at this ADR.
- `internal/infra/workspace/manager_impl.go` now installs a hook via
  `init()`. Init functions can be load-order traps; the hook is
  guarded with `if LoadDepsForMigration != nil` in
  `workspace.Resolve` so a missing init is a no-op (legacy migration
  is best-effort already).

### Neutral

- Test fixtures keep using the alias spelling (`config.Deps{...}`,
  `state.LocalState{...}`). The aliases are real Go type aliases, so
  this is identical to `models.Deps{...}` — same memory layout, same
  method set, fully interoperable.
- `cloneInfraEntry` / `cloneService` (ADR-006) keep working
  unchanged: they read from `models.Service` / `models.Infra` fields
  whose names didn't change. Future struct-field additions still need
  the clone-sync grep documented in ADR-006.

## Alternatives considered

- **Stop at the interim solution (move only Runtime / DetectResult).**
  Honest about cost but leaves `domain/interfaces` importing
  infrastructure forever. The cost-vs-benefit math changed once the
  mass-rename tooling (sed + goimports, see issue 023's commit) proved
  reliable enough to do the full migration in a single session.
- **Define `Runtime` and `DetectResult` in a separate leaf package
  like `internal/domain/scan/`.** Would also work but adds a package
  whose role would be hard to distinguish from `domain/models`. Keeping
  them in `models` matches their conceptual role.
- **Move `Workspace` and `ProcessInfo` into `domain/models` too.** The
  obvious next step. Out of scope here because both types carry
  filesystem-shaped behavior (path resolution, process lifecycle) that
  is more than a value object, and lifting them needs its own ADR. The
  `LoadDepsForMigration` hook is the pattern that future work would
  follow.

## References

- Code: `internal/domain/models/{runtime,config_env,config_proxy,
  config_infra,config_service,config_deps,config_diag,state}.go`,
  `internal/domain/interfaces/*.go`,
  `internal/detect/result.go`,
  `internal/workspace/workspace.go` (LoadDepsForMigration hook),
  `internal/infra/workspace/manager_impl.go` (hook installer).
- Related: ADR-006 (clone-sync — same struct-mirroring discipline now
  applies to `internal/domain/models/` field additions).
- Originated from: architecture review findings CRITICAL #1 and #2;
  completed in issue 023.
