# ADR-044: TUI dashboard reads state; its three actions are the ceiling

- **Status:** Amended 2026-09-06 — the original was written on a false premise
- **Date:** 2026-05-16 (amended 2026-09-06)

> **What was wrong.** The original text asserted that the dashboard does
> not mutate state and does not spawn processes, and decided it must stay
> that way. Both claims were already false when written: `r`, `s` and `e`
> have been bound to restart, stop and exec since `c675a26`
> (2026-04-08), five weeks before this ADR. The decision was therefore
> adopted against a dashboard that was already a small control plane, and
> the "adding interactive features is a CR red flag" line pointed at
> features that had shipped.
>
> The amendment keeps what the reasoning got right — the coordination
> problem is real and nobody has solved it — and drops the claim that the
> code was clean. What follows describes the dashboard as it is.

## Context

`internal/tui/` implements `raioz dashboard` using bubbletea. It
runs in-process under the cobra command surface, polls Docker
for state (`docker ps`, `docker inspect`, `docker stats`), and
reads the local `.raioz.state.json`.

The dashboard does **not** today:

- Mutate any state file (`.raioz.state.json` and everything under
  `RaiozStateDir()` are read-only from TUI code).
- Coordinate via IPC with other raioz processes (`raioz up`,
  `raioz down`, `raioz up --watch`).

It **does** mutate container state, through exactly three
keybindings in `internal/tui/actions.go`:

- `r` — `docker restart <container>`
- `s` — `docker stop <container>`
- `e` — `docker exec -it <container> sh` (via `tea.ExecProcess`)

Each addresses the container by `naming.Container()` and shells out
directly. **None of them takes the project or workspace lock.** A
restart pressed while `raioz up --watch` is rebuilding the same service
in another terminal is a race with no arbiter: both write to Docker,
neither knows the other exists, and `docs/LOCKS.md` does not list the
TUI as a writer because — per the original text of this ADR — it was
believed not to be one.

That gap is the reason to stop here rather than grow. Adding "Promote
dep", "Migrate", or anything that touches state files would need:

- Workspace lock acquisition during every mutation.
- Coordination with `raioz up --watch` if it is active.
- State consistency under concurrent dashboard + CLI commands.
- IPC channel between the dashboard and any active raioz
  process (or accept races that ADR-023 — state mirrors reality —
  explicitly forbids).

## Decision

**The three existing actions are the ceiling, not a precedent.** The
contract:

1. **Docker access is read-only except for `restart`, `stop` and
   `exec` on a single container the user selected.** No `docker run`,
   no `docker rm`, no compose invocation, no `raioz` subprocess.
2. **Read-only state access.** `.raioz.state.json` and anything
   under `RaiozStateDir()` may be read; never written from the TUI
   code path. This half of the original contract holds.
3. **No process spawning.** No `exec.Command` of `raioz` itself or
   any other binary that mutates state. `docker exec` hands the
   terminal to a shell inside a container the user picked; it does
   not orchestrate.
4. **No cross-process IPC.** The dashboard does not coordinate with
   other raioz processes; it polls.
5. **The three actions are unlocked and stay that way until someone
   does the design work below.** They are cheap, single-container,
   user-initiated operations whose worst case is losing a race with a
   concurrent CLI command — recoverable by re-running it. That is the
   accepted risk, stated rather than assumed.

Any *fourth* action — Promote / Migrate / anything touching state
files or more than one container — **requires a separate ADR** that
addresses:

- Workspace lock acquisition (per ADR-023 + issue 038).
- Coordination semantics with `raioz up --watch` and any other
  active raioz process.
- IPC channel design (unix socket? state-file watch?
  centralized "control" daemon?).
- Failure mode when the dashboard cannot acquire a lock the
  user expected to be free.

Widening the action set without a dedicated ADR is a CR red flag.
Not the three that exist; the fourth.

## Consequences

### Positive

- The dashboard can be added, removed, or rewritten without
  touching the raioz lifecycle.
- The blast radius stays one container per keypress.
- TUI testing surface stays narrow — no mock for control-plane
  IPC.

### Negative

- Restart / stop / exec race any concurrent `raioz up`, `raioz down`
  or `raioz up --watch` on the same service. Known, accepted, and
  now written down instead of denied.
- `docs/LOCKS.md` describes writers that take a lock; the TUI mutates
  without one and is the exception that document does not cover.
- Users who want anything beyond those three have to use the CLI in
  another terminal. Documented limit, not a bug.

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
