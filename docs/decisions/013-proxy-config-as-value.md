# ADR-013: ProxyManager configuration is passed as a value

- **Status:** Accepted — Phase 1 implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`interfaces.ProxyManager` accumulated eight individual setters
(`SetDomain`, `SetTLSMode`, `SetBindHost`, `SetProjectName`,
`SetNetworkSubnet`, `SetContainerIP`, `SetWorkspace`, `SetPublish`)
that callers were expected to invoke in a specific order before any
useful method (`Start`, `AddRoute`, `SaveProjectRoutes`). The
`upcase` orchestration alone had a 4-8 setter dance before each
`raioz up`, repeated across `raioz down` and `raioz proxy`.

Failure modes:

- **Order dependency.** Forgetting `SetWorkspace("")` before `Start`
  in a per-project run would carry a stale workspace name from a
  previous call, attaching the proxy to the wrong network.
- **Tests had to set 3+ fields before exercising the real behavior.**
  Intent was buried in setup.
- **The interface said "Manager has 8 setters" but really meant
  "Manager has one config that arrives in pieces."**

## Decision

Add a `ProxyConfig` value type to `internal/domain/interfaces/` and a
`Configure(cfg ProxyConfig)` method on `ProxyManager` that applies
every field in one call. The eight per-field setters are kept for
backward compatibility but marked `Deprecated:`. Every new call site
should construct a `ProxyConfig` and call `Configure` once.

`ProxyConfig` is a plain struct with zero-value defaults documented
inline; the implementation in `internal/proxy/manager_config.go`
provides `Configure` by chaining the existing setters internally, so
the behavior of each field stays identical to today.

`upcase/orchestration_proxy.go` is migrated as a demonstration:

```go
cfg := interfaces.ProxyConfig{
    ProjectName:   deps.Project.Name,
    Workspace:     deps.Workspace,
    NetworkSubnet: deps.Network.GetSubnet(),
}
if deps.ProxyConfig != nil {
    cfg.Domain      = deps.ProxyConfig.Domain
    cfg.TLSMode     = deps.ProxyConfig.TLS
    cfg.ContainerIP = deps.ProxyConfig.IP
    cfg.Publish     = deps.ProxyConfig.Publish
}
uc.deps.ProxyManager.Configure(cfg)
```

Mocks (`mocks.MockProxyManager` and the per-package
`mockProxyManager` in `internal/app`) implement `Configure` by
delegating to the existing setter mocks, so test fixtures that
inspect `m.ProjectName`, `m.Domain`, … keep working.

## Implementation status

Landed in this commit:

- `interfaces.ProxyConfig` value type.
- `ProxyManager.Configure(cfg)` method on the interface.
- `proxy.Manager.Configure` implementation.
- Existing setters marked `Deprecated:`.
- `upcase/orchestration_proxy.go` migrated as a sample.
- Mocks updated.

Not landed (follow-up):

- **Migrate the remaining four call sites** (`down_proxy.go`,
  `cli/proxy.go`, …). Mechanical: replace 1-4 setter calls with a
  single `Configure(...)`. Deferred because each touches the
  up/down hot path and earns no architectural value beyond
  ergonomics.
- **Move presenters** (`GetURL`, `HostsLine`) **out of the manager**
  to free functions in `internal/output/`. Same rationale as
  ADR-012's deferred Format moves: the cycle direction makes the
  relocation non-trivial.
- **Delete the deprecated setters** once every call site is
  migrated. Gated on a grep, not build-time enforcement.

## Consequences

### Positive

- New call sites get one obvious entry point.
- Tests can build a `ProxyConfig` literal and pass it; intent is
  visible in one place.
- The 4-8 setter dance in `upcase` collapsed to ~6 lines.

### Negative

- The interface is larger by one method while the deprecated setters
  live. Temporary regression in surface area that pays back on the
  next cleanup.
- `*bool` for `Publish` carries the same tri-state quirk it had on
  `SetPublish(*bool)`. Could become `enum ProxyPublishMode` later;
  out of scope.

### Neutral

- `ProxyConfig`'s field shape mirrors `models.ProxyConfig` (the
  YAML-side struct) but with semantic-only field names. A future
  consolidation could pick one as canonical; for now they're
  independent.

## Alternatives considered

- **Pass `ProxyConfig` to a new constructor (`NewWithConfig`).**
  Discarded because raioz currently builds a single `ProxyManager`
  at `Dependencies` init time and per-project config arrives later
  from `raioz.yaml`. Switching to a per-project constructor reworks
  the DI model.
- **Builder pattern (setters return `*Manager`).** Same number of
  calls, less readable than a struct literal.
- **Keep the setters and add no `Configure`.** Status quo. Preserves
  the order footgun.

## References

- Code: `internal/domain/interfaces/proxy.go` (interface +
  ProxyConfig value), `internal/proxy/manager_config.go`
  (implementation), `internal/app/upcase/orchestration_proxy.go`
  (sample migration), `internal/mocks/proxy.go` (mock).
- Related: ADR-012 (DockerRunner segregation — same "narrow the
  contract" pattern).
- Issue: 033.
