# ADR-032: ProxyManager interface uses Configure-only + typed TLSMode

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14
- **Supersedes (in part):** [ADR-013](013-proxy-config-as-value.md) Phase 2

## Context

ADR-013 (2026-05-13) introduced `ProxyConfig` + `Configure(cfg)` on
the `ProxyManager` port but kept the eight per-field setters
(`SetDomain`, `SetTLSMode`, `SetBindHost`, `SetProjectName`,
`SetNetworkSubnet`, `SetContainerIP`, `SetWorkspace`, `SetPublish`)
marked `Deprecated:` for backward compatibility. Phase 1 migrated
`upcase/orchestration_proxy.go` as a demonstration; the rest of the
call sites and the mocks kept the legacy surface.

Two follow-up problems were flagged by issue 055:

1. **Vendor leakage.** `ProxyConfig.TLSMode` was `string` with no
   constraint, so the interface effectively required callers to know
   Caddy vocabulary (`"mkcert"`, `"letsencrypt"`). A future Traefik
   adapter would either have to accept Caddy strings (semantically
   wrong) or break the interface.
2. **Setter duplication.** `Configure` + 8 deprecated setters meant
   every implementation (concrete `Manager`, mock, future adapters)
   had to provide nine equivalent code paths. Tests split between
   the two styles.

## Decision

Two coupled changes, landed together:

### 1. Add `interfaces.TLSMode` typed enum

`internal/domain/interfaces/tls_mode.go` defines:

```go
type TLSMode string

const (
    TLSModeLocal  TLSMode = "local"
    TLSModeACME   TLSMode = "acme"
    TLSModeManual TLSMode = "manual"
)

func ParseTLSMode(s string) (TLSMode, bool)
```

The three constants are vendor-neutral. `ParseTLSMode` accepts the
canonical values plus legacy aliases (`"mkcert"` → `TLSModeLocal`,
`"letsencrypt"` → `TLSModeACME`) so user-facing YAML keeps working;
empty → `TLSModeLocal`; unknown → `("", false)`.

`ProxyConfig.TLSMode` is now `TLSMode`, not `string`.

### 2. Remove the eight deprecated setters

`ProxyManager` keeps only `Configure(cfg ProxyConfig)` for
configuration. All eight `SetXXX` methods are removed from:

- `internal/domain/interfaces/proxy.go` — the port.
- `internal/proxy/manager_config.go` — the concrete `Manager`.
- `internal/mocks/proxy.go` — the test double.

`Manager.Configure` now inlines what each setter used to do.
`MockProxyManager.Configure` captures the fields directly so tests
can still inspect `m.Domain`, `m.TLSMode`, etc.

### 3. Caddy vocabulary stays internal to the Caddy adapter

`caddyTLSValue(TLSMode) string` in `manager_config.go` maps the
neutral enum onto the existing Caddy literals (`"mkcert"`,
`"letsencrypt"`, `"manual"`). Caddyfile generation and `EnsureCerts`
keep branching on Caddy strings; only the boundary moves.

## Implementation status

Landed in this commit:

- `internal/domain/interfaces/tls_mode.go` (new).
- `internal/domain/interfaces/proxy.go` — `ProxyConfig.TLSMode`
  retyped; 8 `Deprecated:` methods removed from `ProxyManager`.
- `internal/proxy/manager_config.go` — setters removed; `Configure`
  inlines their bodies; `caddyTLSValue` helper added.
- `internal/mocks/proxy.go` — setters removed; `Configure` captures
  fields; `TLSMode` field is `interfaces.TLSMode`.
- `internal/app/upcase/orchestration_proxy.go` — `TLS` string from
  the YAML model is mapped through `ParseTLSMode`.
- `internal/app/down_proxy.go` — uses `Configure({ProjectName,
  Workspace})` instead of two setter calls.
- Tests across `internal/proxy/*_test.go` and
  `internal/app/upcase/proxy_coverage_test.go` migrated from setter
  calls to direct field access (in-package) or `Configure(...)`
  (out-of-package); setter-guard tests replaced with equivalent
  `Configure`-based assertions.

## Consequences

### Positive

- The interface no longer leaks Caddy vocabulary. A future Traefik
  adapter implements `Configure(ProxyConfig)` and supplies its own
  TLS-mapping helper; no port change.
- One configuration entry point, period. Setter-ordering bugs are
  impossible by construction.
- The mock and the real `Manager` share the same surface — tests
  can be written against either without worrying about which subset
  of methods is implemented.

### Negative

- Breaking change for any external consumer of the `ProxyManager`
  port. Acceptable because the port is `internal/`; no published
  API.
- Tests that exercised the "empty value doesn't overwrite" guard
  on setters had to be rewritten as `Configure`-based equivalents.
  Done in this change.

### Neutral

- `caddyTLSValue` keeps the legacy strings alive inside the proxy
  package so Caddyfile templates and existing cert paths don't
  churn. Refactoring those is a separate task with no architectural
  payoff.
- The user-facing YAML key (`proxy.tls`) still accepts `"mkcert"` /
  `"letsencrypt"` for backward compatibility; `ParseTLSMode`
  bridges. A future major version could drop the legacy aliases.

## Alternatives considered

- **Keep deprecated setters indefinitely.** ADR-013's stance. The
  duplication cost compounded once tests and mocks had to support
  both styles.
- **Use `iota` int constants instead of typed strings.** Rejected
  because the wire/YAML representation is a string and parsing/round
  -tripping is more legible when the underlying type matches.
- **Add a `TLSResolver` interface and inject it.** Over-engineered
  for a three-value enum where the mapping fits in 6 lines.

## References

- Code: `internal/domain/interfaces/tls_mode.go` (enum),
  `internal/domain/interfaces/proxy.go` (port),
  `internal/proxy/manager_config.go` (Configure +
  `caddyTLSValue`), `internal/mocks/proxy.go`.
- Tests: `internal/proxy/manager_test.go`,
  `internal/proxy/proxy_coverage_test.go`,
  `internal/proxy/publish_test.go`,
  `internal/proxy/routes_persist_test.go`,
  `internal/app/upcase/proxy_coverage_test.go`.
- Issue: 055.
- Related: ADR-013 (Phase 1 of this work), ADR-012 (DockerRunner
  segregation — same "narrow the contract" pattern).
