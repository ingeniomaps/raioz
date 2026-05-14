# ADR-025: HostRunner waits for the launcher's container before reporting ready

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`internal/orchestrate/host_runner.go` runs a service's `command:` on
the host. When that command exits with status 0 within
`host.SettleWindow()` (default 500ms), the runner treats it as the
*launcher pattern* — a script that detached long-running work and
returned. Examples:

- `make dev-docker` invoking `docker compose up -d --build`.
- `./scripts/start.sh` that spawns a daemonized binary.

The original rule (issue 010) was "trust the clean exit; downstream
status checks will surface failures." That works when the launcher
already produced the artifact (a running container, a daemon) by
the time it exits. It does **not** work when the launcher returns
while its work is still in progress:

> `docker compose up -d --build` exits the moment the build job is
> *queued*, not when the container is running. A first-time build
> of a Next.js or Go service can take 60s+ during which raioz tells
> the user "ENTORNO LISTO" and shows the service as healthy.

Two failure modes follow:

1. **Premature ready.** Downstream consumers (sibling projects, the
   user staring at the terminal) believe the service is up. The
   user moves on, makes a request, gets a connection refused —
   confidence in `raioz up` erodes.

2. **Orphan on down.** If the user runs `raioz down` during the
   build window, raioz tears down what it sees (nothing yet) and
   reports success. The build finishes afterwards; the container
   appears with nobody owning it. Next `raioz up` fails on port or
   name conflict.

Issue 047 captured both. The contract the runner enforces needs to
include "the container declared as the proxy target actually
exists" before reporting ready, and "wait for any in-progress
launcher work to finish" before running the user's `stop:`.

## Decision

Add **two opt-in waits**, gated on the service declaring
`proxy.target:` (the only signal HostRunner has about the launched
container's identity):

### Wait 1 — post-launcher container appearance

After `Start` observes a clean exit inside the settle window:

```text
if !docker.IsHostGatewayTarget(svc.ProxyTarget):
    poll `docker inspect <ProxyTarget>` for up to LauncherWaitTimeout
    print "Waiting for launcher container '<name>' to appear…"
    on success: print "Launcher container '<name>' ready"
    on timeout: print warning + continue (no abort)
```

- `LauncherWaitTimeout` defaults to **60s**, configurable via
  `RAIOZ_LAUNCHER_TIMEOUT` (Go duration: `90s`, `2m`, etc.).
  Explicit `0s` opts out — preserves pre-fix behavior for users
  whose launchers are fast enough that the wait is pure overhead.
- Timeout is a warning, not an abort. A slow CI box should still
  make progress; the warning text points the user at `docker ps`
  if downstream connection failures emerge.

### Wait 2 — pre-stop launcher drain

When `Stop` is called with a custom `stop:` declared, AND the
service was marked launcher-mode at `Start` time, AND the
container doesn't exist yet:

```text
if !docker.IsHostGatewayTarget(svc.ProxyTarget):
    if container does not exist:
        poll for up to LauncherDrainTimeout
        print "Waiting up to <T> for launcher build of '<name>' …"
        on success: continue and run stop:
        on timeout: print warning + run stop: anyway
```

- `LauncherDrainTimeout` defaults to **30s**, configurable via
  `RAIOZ_LAUNCHER_DRAIN_TIMEOUT`.
- Without this, a `raioz down` that races with `docker compose up
  -d --build` leaves the user with an orphan once the build
  finishes.

### What stays unchanged

- The launcher-pattern detection in Start (clean exit in settle
  window). No regressions for users who relied on the previous
  "fast ready" behavior — they didn't declare `proxy.target:` and
  the new path is a no-op.
- Services without `proxy.target:` get the same diagnostic as
  before: "exited 0 within the settle window — likely a launcher
  that detached…", followed by the "no `stop:` declared" warning
  if applicable. The waits don't apply because we have no
  container name to poll.
- The legacy compose / dockerfile runners. They don't suffer this
  problem (they wait for `docker compose up` to complete) and the
  fix is HostRunner-specific.

## Implementation status

Landed in this commit:

- `internal/docker/wait.go`: `IsHostGatewayTarget`,
  `WaitForContainer`.
- `internal/host/process_helpers.go`: `LauncherWaitTimeout`,
  `LauncherDrainTimeout`, `osGetenv` indirection seam, env-var
  constants.
- `internal/domain/interfaces/orchestrator.go`: `ProxyTarget`
  field on `ServiceContext`.
- `internal/app/upcase/orchestration.go`: populates
  `svcCtx.ProxyTarget` from `svc.ProxyOverride.Target` for both
  the orchestrated startup loop and the watch-mode shutdown /
  restart paths.
- `internal/app/upcase/watch_setup.go`: same plumbing for the
  shutdown sweep and restart callback.
- `internal/orchestrate/host_runner.go`: `launchers map[string]bool`,
  Start invokes `waitForLauncherContainer` on launcher detection,
  Stop invokes `drainLauncherBeforeStop` before running `stop:`.
- `internal/orchestrate/host_runner_launcher.go`: the two helpers,
  each ~30 LoC, with visible progress + warning text.
- Tests: `internal/docker/wait_test.go::TestIsHostGatewayTarget`
  (predicate matrix); `internal/host/launcher_timeout_test.go`
  (env-driven defaults + opt-out + fallback semantics for both
  timeouts).

## Consequences

### Positive

- `raioz up` no longer prints "ENTORNO LISTO" while the user's
  `docker compose up -d --build` is still queueing builds. The
  user sees an explicit "Waiting for launcher container…" line and
  a "Launcher container 'X' ready" confirmation.
- `raioz down` no longer creates orphan containers in the race
  window between the launcher exit and the actual build finish.
- The fix is opt-in via declaration (`proxy.target:`) and the
  configured timeout, so existing services that don't hit this
  scenario see no behavior change.

### Negative

- A new env var pair to remember (`RAIOZ_LAUNCHER_TIMEOUT`,
  `RAIOZ_LAUNCHER_DRAIN_TIMEOUT`). Documented next to the field
  table in CONFIG_REFERENCE and surfaced in the warning text.
- Slow boxes with `proxy.target:` declared but no launcher
  pattern will see an extra ~60s wait if the launcher takes >500ms
  to detach. The `0s` opt-out is the escape hatch; the warning
  on timeout suggests `docker ps` as a confirmation.
- HostRunner gains direct knowledge of `internal/docker`. The
  alternative — a docker-side adapter wired through `Dependencies`
  — was rejected as overkill for two thin probe functions. Both
  helpers (`WaitForContainer`, `IsHostGatewayTarget`) are tiny
  and the `host_runner_launcher.go` separation isolates the
  coupling.

### Neutral

- The launcher tracking map is in-memory on the HostRunner
  struct. It does not survive a process restart — meaning if
  the user kills raioz mid-up, the next invocation won't know
  about the launcher mode and falls back to the old behavior on
  Stop. Acceptable trade-off: this is a niche recovery path, and
  persisting launcher mode in LocalState would couple the state
  schema to a transient runner concern.

## Alternatives considered

- **Always wait, no opt-out** — would slow down every host service
  by up to 60s when the container check times out. Rejected: the
  fast-launcher case is common and shouldn't pay for the slow
  case.
- **Wait by sniffing `make`/`docker compose` subprocesses** —
  fragile: launchers vary widely. The `proxy.target:` signal is
  user-supplied and unambiguous.
- **Refuse to mark ready until ProxyTarget is up** (no warning,
  abort on timeout). Too aggressive: the original "trust the
  clean exit" rule shipped to ship; reverting that contract on a
  hard timeout would regress users with very slow first-time
  builds. The warning + continue path lets them choose.
- **Block in Stop until the launcher PID exits** (issue 047
  option B as written). The launcher PID is already dead when
  the clean-exit path fires — that's why we wrapped the launcher
  detection around `case exitErr := <-waitCh` with exitErr == nil.
  Polling the container name is the right signal.

## References

- Code: `internal/orchestrate/host_runner.go`,
  `internal/orchestrate/host_runner_launcher.go`,
  `internal/docker/wait.go`,
  `internal/host/process_helpers.go`.
- Tests: `internal/docker/wait_test.go`,
  `internal/host/launcher_timeout_test.go`.
- Predecessor: issue 010 introduced the launcher-pattern
  detection without the post-launcher container wait. This ADR
  closes the loop.
- Issue: 047.
