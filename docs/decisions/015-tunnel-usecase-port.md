# ADR-015: Tunnel lifecycle runs through a use-case port

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`raioz tunnel` was a CLI-direct command in the same shape ADR-014
just retired for snapshot: `internal/cli/tunnel.go` imported
`internal/tunnel` and called `tunnel.NewManager().Start(...)` inline.
The tunnel package shells out to `cloudflared` or `bore` and tracks
PIDs in `~/.raioz/tunnels.json`; CLI tests couldn't exercise it
without one of those binaries.

Same architectural smell, same fix.

## Decision

Mirror the snapshot pattern from ADR-014:

- **Port:** `interfaces.TunnelManager` covers Start / Stop / StopAll
  / List. The `TunnelInfo` value type lives next to the interface so
  callers don't import `internal/tunnel`.
- **Adapter:** `internal/infra/tunnel.ManagerImpl` wraps
  `internal/tunnel.Manager`. Backend detection (cloudflared vs bore
  vs future frp/ngrok) stays inside `internal/tunnel` — it's adapter
  concern, not domain.
- **Use case:** `internal/app/tunnelcase` with one struct per verb
  (Start/Stop/StopAll/List). Each takes a narrow `Dependencies`
  ({TunnelManager}) — the use case doesn't need a config loader
  because tunnels are addressed by service name, not by project.
- **CLI:** `internal/cli/tunnel.go` rewritten to thin wiring (~100
  lines for four subcommands; matches the snapshot result).
- **DI:** `Dependencies.TunnelManager` wired in
  `internal/app/dependencies.go`.

The CLI's default port (3000) stays a CLI-level constant
(`defaultTunnelPort`). The use case treats `LocalPort: 0` as "the
caller forgot to set it" and passes it through — picking a default
is a presentation decision, not a lifecycle one.

## Implementation status

Landed in this commit:

- `internal/domain/interfaces/tunnel.go` — port + `TunnelInfo`.
- `internal/infra/tunnel/manager_impl.go` — adapter.
- `internal/app/tunnelcase/usecase.go` — four use cases.
- `internal/app/tunnelcase/usecase_test.go` — five tests, local
  `mockTunnelManager`. No `cloudflared` or `bore` required.
- `internal/cli/tunnel.go` rewritten to thin wiring.
- `Dependencies.TunnelManager` wired.

## Consequences

### Positive

- Tunnel CLI tests can stub the port; no external binary needed to
  exercise the lifecycle.
- The "which backend?" decision is encapsulated in the adapter, with
  a clean spot to add frp/ngrok later without touching the CLI.
- Pattern consistency: snapshot and tunnel now follow the same
  shape as every other command (ADR-014 sibling).

### Negative

- Same wrapper cost as ADR-014: two new layers (port + use case) for
  what was ~95 lines of CLI. The win materializes with the next
  tunnel feature added on top.
- `interfaces.TunnelInfo` duplicates the shape of
  `internal/tunnel.Info`. The adapter copies field-by-field. Same
  treatment as ADR-014's `convertSnapshot`: track field additions in
  both places.

### Neutral

- `StopAll` returns `error` on the port but the underlying
  implementation today is best-effort and returns nothing. The
  adapter returns nil. Future implementations (e.g. one that fails
  fast when the registry can't be written) can surface real errors
  without changing the port.

## Alternatives considered

- **Leave the tunnel CLI as-is.** Status quo. Preserves the
  "external-binary-required to test" problem.
- **Single `TunnelUseCase` with verb arg.** Half the boilerplate;
  rejected for the same reason ADR-014 rejected it — every other
  command uses one-struct-per-verb.
- **Expose `internal/tunnel.Info` directly through the port.** Pollutes
  the domain with an infra type, the exact pattern ADR-009 fixed.

## References

- Code: `internal/domain/interfaces/tunnel.go`,
  `internal/infra/tunnel/manager_impl.go`,
  `internal/app/tunnelcase/`, `internal/cli/tunnel.go`.
- Related: ADR-014 (snapshot — same template), ADR-009 (domain owns
  model types).
- Issue: 035.
