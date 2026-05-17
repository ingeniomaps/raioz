# ADR-046: Runtime capability matrix for docker/podman/nerdctl

- **Status:** Accepted — v1 (binary-name keyed)
- **Date:** 2026-05-16

## Context

raioz abstracts container runtimes via `internal/runtime/runtime.go`
(`Binary()`, `ComposeBinary()`, `IsDocker()`). The abstraction is a
string + helpers — runners emit flags assuming all three backends
accept them. In practice the runtimes differ:

| Feature | docker 24+ | podman 4.x | podman 5.x | nerdctl 1.x | nerdctl 2.x |
|---|---|---|---|---|---|
| `--add-host=host.docker.internal:host-gateway` | ✅ | ≥ 4.7 | ✅ | ❌ rejects flag | ✅ |
| `compose --profile` | ✅ | ≥ 4.6 | ✅ | ✅ (1.7+) | ✅ |
| Label filter survives `compose down` | ✅ | sometimes drops | ✅ | ✅ | ✅ |

Without per-feature gating, every "raioz adopts a docker flag" PR
breaks one of the alternative runtimes silently. Issue 041.

## Decision

Introduce `internal/runtime/capability.go` with a small enum and a
`Supports(Capability) bool` lookup. V1 of the matrix is intentionally
small (three capabilities) and keyed only on the runtime binary
name; version detection is deferred. Conservative defaults: when
detection can't classify, raioz assumes the runtime supports the
capability (matches today's behaviour — runners emit flags optimistically).

```go
type Capability int
const (
    HostGatewayAlias  Capability = iota // --add-host=host.docker.internal:host-gateway
    ComposeProfiles                     // docker compose --profile
    LabelFilterOnDown                   // reliable --filter "label=..." on compose down
)
func Supports(c Capability) bool { ... }
```

Per-capability rules in v1:

- `HostGatewayAlias`: **false** for `nerdctl` (1.x doesn't support;
  no version detection yet — fail-safe). All other runtimes
  optimistic-true.
- `ComposeProfiles`: optimistic-true everywhere; version-pinning
  follows when a real bug surfaces.
- `LabelFilterOnDown`: optimistic-true everywhere; podman label-loss
  is intermittent and version-dependent.

### Sites that gate

v1 wires `Supports(HostGatewayAlias)` into the four runners /
proxy paths that previously injected the alias unconditionally:

- `internal/orchestrate/dockerfile_runner.go::Run` (`--add-host` arg)
- `internal/orchestrate/image_runner.go::Render` (compose
  `extra_hosts`)
- `internal/orchestrate/compose_runner.go::createNetworkOverlay`
  (compose `extra_hosts`)
- `internal/proxy/proxy.go::startContainer` (`--add-host` arg)

Issue 021 added the compose runner injection; ADR-046 makes it
runtime-aware along with the other three.

## What's deferred

- **Version detection.** Parse `<binary> --version` and refine the
  matrix when a known-bad version is detected. Currently nerdctl
  2.x users pay a false-negative on `HostGatewayAlias` (we report
  unsupported even though 2.x supports it). Acceptable v1 trade —
  emitting an unrecognised flag to nerdctl 1.x is worse than the
  false negative on 2.x.
- **Operator override.** `RAIOZ_RUNTIME_CAPABILITY=
  HostGatewayAlias=true` env knob to opt nerdctl 2.x users back in
  before version detection lands. Trivial to add when needed.
- **Publishing the matrix to users.** `docs/RUNTIMES.md` or a
  section in `docs/CI.md` enumerating per-feature support. Worth
  doing once two capabilities have non-trivial rules.

## Consequences

### Positive

- Adding a new capability is one enum + one switch case. Centralized
  source of truth replaces ad-hoc `if Binary() == "podman"` checks
  scattered through the runners (none today; ADR pre-empts the
  scattering).
- Bug reports against alternative runtimes become specific
  ("nerdctl 1.x fails because we hit `HostGatewayAlias`") instead
  of "raioz doesn't work on nerdctl".

### Negative

- Optimistic defaults mean some bugs still ship silently for
  runtimes/versions raioz hasn't profiled. The matrix shrinks the
  surface but doesn't eliminate it.
- Two parallel knobs (`RAIOZ_RUNTIME` + future
  `RAIOZ_RUNTIME_CAPABILITY`) is a configuration smell if version
  detection takes too long to land. Document the addition path
  when it ships.

### Neutral

- v1 keyed only on binary name means raioz doesn't fork
  `<binary> --version` per call. Pure constant lookup. Cost is
  effectively zero.

## Alternatives considered

- **Per-runner `if IsDocker()` chains.** Rejected — drifts as more
  capabilities accrue; impossible to audit "where does raioz
  assume docker semantics?"
- **Defer everything until a user reports a bug.** Rejected —
  issue 021 (the compose `extra_hosts` omission) showed that
  silent silent breakage on alternative runtimes accumulates
  faster than the bug-report cycle.
- **Pull version into capability lookups now.** Rejected for v1 —
  scope creep. Trade documented as "deferred".

## References

- Code: `internal/runtime/capability.go`,
  `internal/orchestrate/{dockerfile,image,compose}_runner.go`,
  `internal/proxy/proxy.go`.
- Related: Issue 021 (compose extra_hosts paired with this ADR),
  ADR-030 (Windows CI gate — the OS axis; this ADR is the runtime
  axis).
- Issue: 041.
