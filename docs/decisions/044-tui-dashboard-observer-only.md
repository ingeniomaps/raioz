# ADR-044: TUI dashboard is observer-only, not a control plane

- **Status:** Accepted
- **Date:** 2026-05-16

## Context

`internal/tui/` implements `raioz dashboard` using bubbletea. It
runs in-process under the cobra command surface, polls Docker
for state (`docker ps`, `docker inspect`, `docker stats`), and
reads the local `.raioz.state.json`.

The dashboard does **not** today:

- Mutate any state file.
- Spawn any process.
- Coordinate via IPC with other raioz processes (`raioz up`,
  `raioz down`, `raioz up --watch`).

Two raioz processes running concurrently (e.g. `raioz up --watch`
in one terminal + `raioz dashboard` in another) operate
independently: each polls Docker, neither knows the other
exists. This is fine — the dashboard is purely observational.

But this property is **load-bearing** for reasoning about lock
contracts (ADR-028, issue 038) and yet lives only as an absence
in the code. The next contributor who adds "Stop service",
"Restart", or "Promote dep" buttons to the TUI would silently
turn it into a control plane, which would then need:

- Workspace lock acquisition during every mutation.
- Coordination with `raioz up --watch` if it is active.
- State consistency under concurrent dashboard + CLI commands.
- IPC channel between the dashboard and any active raioz
  process (or accept races that ADR-023 — state mirrors reality —
  explicitly forbids).

## Decision

**The TUI dashboard is observer-only.** The contract:

1. **Read-only Docker access.** `docker ps`, `docker inspect`,
   `docker stats`, `docker events`. No `docker run`, `docker
   stop`, `docker rm`, `docker exec`.
2. **Read-only state access.** `.raioz.state.json` and any
   under `RaiozStateDir()` may be read; never written from the
   TUI code path.
3. **No process spawning.** No `exec.Command` of `raioz` itself
   or any other binary that mutates state.
4. **No cross-process IPC.** The dashboard does not coordinate
   with other raioz processes; it polls.

Future "interactive" features (Stop / Restart / Promote /
Migrate / etc.) **require a separate ADR** that addresses:

- Workspace lock acquisition (per ADR-023 + issue 038).
- Coordination semantics with `raioz up --watch` and any other
  active raioz process.
- IPC channel design (unix socket? state-file watch?
  centralized "control" daemon?).
- Failure mode when the dashboard cannot acquire a lock the
  user expected to be free.

Adding interactive features without a dedicated ADR is a CR
red flag.

## Consequences

### Positive

- The dashboard can be added, removed, or rewritten without
  touching the raioz lifecycle.
- Two raioz processes coexist safely (CLI + dashboard).
- The lock contract (ADR-023, ADR-028) doesn't need to enumerate
  TUI as a writer.
- TUI testing surface stays narrow — no mock for control-plane
  IPC.

### Negative

- Users who want interactive dashboard controls have to use the
  CLI in another terminal. Documented limit, not a bug.

### Neutral

- The dashboard polls Docker; polling cost scales with N
  services. Issue 040 covers batching the polls. Polling vs
  event-stream is an implementation choice that doesn't change
  the observer-only contract.

## Alternatives considered

- **Add `raioz dashboard --interactive` with control plane.**
  Rejected for v1.0. Would require IPC channel between dashboard
  and active raioz processes; out of scope.
- **Run dashboard as separate process with its own state.**
  Rejected — would complicate deploy independence (now two
  things to install and version) without solving the
  control-plane problem.
- **Leave the property undocumented.** Status quo. Costs the
  next contributor a wrong intuition and a revert.

## References

- Code: `internal/tui/`, `cmd/raioz/main.go` →
  `internal/cli/dashboard.go`
- Related: ADR-023 (state mirrors reality — writer discipline),
  ADR-028 (shared map mutexes — read paths exempted from lock
  but not from snapshot discipline)
- Follow-ups: issue 040 (poll-batching scalability — doesn't
  change this contract)
