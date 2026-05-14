# ADR-016: Proxy CLI runs through use cases and a shared preflight

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`raioz proxy {status,stop}` was the last lifecycle CLI not routed
through `internal/app/`. The CLI talked directly to `ProxyManager`
via the deprecated setters (`SetProjectName`, `SetWorkspace`), echoing
the orchestration dance that ADR-013 just collapsed for `raioz up`'s
proxy section.

The issue also flagged a cross-cutting value: a **shared preflight**
that both `raioz proxy` and `raioz doctor` could run against a proxy
configuration to surface fixable issues (mkcert missing, host ports
busy) before any state changes.

The codebase today only has two proxy subcommands — `status` and
`stop` — because `raioz up`/`raioz down` own the Up/Down half of the
lifecycle. The issue's third use case (`UpUseCase`) doesn't apply to
the current CLI surface and is dropped.

## Decision

Two parallel pieces:

1. **Use cases** in `internal/app/proxycase/`:
   - `StatusUseCase` reports `ProxyManager.Status` after applying
     project scope (via `Configure`, per ADR-013) from the loaded
     `raioz.yaml`.
   - `StopUseCase` stops the proxy with the same scoping.
   - `ErrProxyNotConfigured` lets callers distinguish "no proxy
     configured for this project" from real failures without parsing
     error strings.

2. **Preflight** in `internal/app/proxycase/preflight.go`:
   - `PreflightInput` carries the small piece of configuration each
     check needs (`Publish`, `TLSMode`).
   - `PreflightCheck` is a result row with `Required` separating
     blocking failures from advisory ones, plus a `Hint` field so
     the CLI can render actionable suggestions.
   - Probes today: `mkcert` on PATH (required when TLS=mkcert),
     host port 80 free (required when `Publish=true`), host port
     443 free (same). Each probe is independent — one failing does
     not short-circuit the rest.

`internal/cli/proxy.go` shrinks from 78 → 68 lines and stops
touching `deps.ProxyManager` directly; both subcommands wire through
the use case.

## Implementation status

Landed in this commit:

- `internal/app/proxycase/preflight.go` — `RunPreflight`, three
  probes.
- `internal/app/proxycase/usecase.go` — `StatusUseCase`,
  `StopUseCase`, `ErrProxyNotConfigured`.
- `internal/app/proxycase/usecase_test.go` — seven tests covering
  the use cases and the preflight switching.
- `internal/cli/proxy.go` rewritten to thin wiring.

**Not landed in this ADR** (tracked for follow-up):

- **Wire `RunPreflight` into `raioz doctor`.** Doctor today has
  `checkMkcert` and `checkCaddy` of its own
  (`internal/app/doctor_orchestrator.go`); the natural follow-up is
  replacing those with `RunPreflight` so the two commands report
  identical diagnoses. Deferred because it requires aligning the
  output shape (`DoctorCheck` vs `PreflightCheck`) and that's a
  doctor-side decision, not a proxy one.
- **Surface preflight from `raioz proxy`.** Today the CLI doesn't
  call `RunPreflight` — the use cases just probe `Status`/`Stop`.
  Adding a `raioz proxy doctor` subcommand or running preflight as
  a preamble to `raioz up` is a UX call that needs design discussion
  separate from the structural refactor.

## Consequences

### Positive

- Proxy CLI now matches the wiring shape of every other command
  (cli → app → port). The deprecated setters
  (`SetProjectName`/`SetWorkspace`) are no longer touched from
  `cli/`; both subcommands flow through `Configure` (ADR-013).
- `RunPreflight` is a small, testable, shared surface. Tests this
  commit ships cover the gating logic (`publish=false` skips port
  probes; mkcert is required iff TLS defaults). The future doctor
  wire-up only has to consume the result list.
- `ErrProxyNotConfigured` is now a typed sentinel callers can
  `errors.Is` against — the CLI's "Proxy is not configured" path
  no longer hinges on `deps.ProxyManager == nil` leaking through
  every consumer.

### Negative

- Two new layers for two small subcommands. The win is the shared
  preflight; without that, this would be busywork.
- The `Status`/`Stop` use cases each declare their own
  `applyProjectScope` method that wraps the same `applyScope` free
  function. Three lines of duplication trading off against making
  the inner function package-private. Acceptable; tighten when a
  third use case appears.

### Neutral

- The use case takes `interfaces.ConfigLoader` instead of a plain
  `*models.Deps` because the CLI hands it a path, not a parsed
  config. Future callers (TUI, daemon) that already have parsed
  deps will get a small "double-load" wart — fix when it actually
  bites.

## Alternatives considered

- **Pass already-parsed `*models.Deps` to the use case.** Saves the
  inner `LoadDeps` call. Discarded because the CLI never holds the
  deps at the proxy command's entry point, so the saving would be
  notional.
- **Roll `Status`/`Stop` into one `ProxyUseCase` with a verb arg.**
  Rejected for the same reason as snapshot/tunnel: every other
  command uses one-struct-per-verb. Stay consistent.
- **Skip the preflight, ship only the wiring.** Would close the
  acceptance criterion narrowly but miss the cross-cutting value
  the issue actually highlights — shared checks for doctor.

## References

- Code: `internal/app/proxycase/{usecase,preflight,usecase_test}.go`,
  `internal/cli/proxy.go`.
- Related: ADR-013 (proxy Configure entry point — both use cases
  call it), ADR-014/015 (snapshot/tunnel — same use-case shape).
- Issue: 036.
