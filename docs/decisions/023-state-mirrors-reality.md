# ADR-023: State files mirror reality — `down` deletes `raioz.root.json`

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`raioz.root.json` lives under
`<RaiozStateDir>/workspaces/<project>/raioz.root.json` and is the
authoritative snapshot of the project's resolved configuration after
the most recent `up`. It is read on the next `up` by
`internal/root/root.go::DetectAssistedServiceDrift` to surface
configuration drift (services that appeared/disappeared since the
snapshot, override origin changes, etc.) and was historically also
the source for the "you're switching projects" prompt that
ADR-011 Phase 3 retired.

`raioz down` did not touch this file. The result: the snapshot
persisted indefinitely after every teardown. The visible bug
ADR-011 Phase 3 closed (an EOF-bound interactive prompt blocking CI
and agents) was the loud symptom; the quiet symptom remains —
**drift detection on the next `up` of an unrelated project compares
against a months-old snapshot of the previous owner of this
project's slot**.

Issue 045 captured both. The interactive prompt is gone; the stale
state survived ADR-011 because Phase 3 was specifically about
killing the prompt path, not about state hygiene.

The duplicated path-resolution logic that ADR-022 just unified
makes the failure mode worse: every contributor who imagines "state
goes here" gets a single canonical answer
(`naming.RaiozStateDir()`), but if `down` doesn't clean what `up`
wrote, the directory accumulates per-project debris.

## Decision

**Invariant — State files mirror reality.** A state file recording
"project P last ran with shape S" may not survive past the moment
project P is fully torn down. Concretely:

1. `internal/root/root.go::Delete(ws)` is the cleanup primitive.
   Absent file is a no-op (returns nil); only OS-level failures
   propagate.
2. `internal/app/down_orchestrated.go::downOrchestrated` calls
   `root.Delete(ws)` after the final teardown phase, **gated on the
   leftovers check**: if any container still carries the project's
   `com.raioz.project` label, the down was incomplete and the state
   file is preserved for diagnostics.
3. The selective-services path (`raioz down svcA svcB`) does NOT
   delete the file — the project isn't being torn down, only a
   subset of its services. `downSelectiveServices` returns early
   before the cleanup hook.
4. Workspace resolve errors and `(nil, nil)` returns from the
   workspace manager are tolerated: the cleanup is best-effort. A
   missed cleanup is a minor annoyance; a failing down is a real
   one.

The invariant is named broadly on purpose. `LocalState`
(`.raioz.state.json` in the project directory) already follows
this rule — see `internal/app/down_orchestrated.go` "Clean local
state" block. Future state files (anything that records "raioz was
here") inherit the same contract.

## Implementation status

Landed in this commit:

- `internal/root/root.go`: `Delete(ws)` added below `Save`.
- `internal/root/root_test.go::TestDelete`: covers
  removes-existing-file and absent-file-is-not-an-error.
- `internal/app/down_orchestrated.go`: cleanup hook after the
  failed-stops check, gated on `len(leftovers) == 0`, defensive
  about resolve failures.
- `internal/app/down_orchestrated_coverage_test.go::TestDownUseCase_downOrchestrated_DeletesRootConfig`:
  end-to-end mock that seeds `raioz.root.json`, drives a full down,
  and asserts the file is gone.

The selective path (`internal/app/down_selective.go`) is untouched —
verified by the early-return at `downOrchestrated.go:51`.

## Degraded mode: Docker unreachable

When the Docker daemon is unreachable AND the operator invokes
`raioz down --force-state-cleanup`, the
`internal/app/down_offline.go::forceOfflineCleanup` path
**relaxes** the "state mirrors reality" invariant. The reasoning:

- The container teardown that the invariant normally guards
  cannot run — there is no daemon to talk to.
- The operator has explicitly opted in via
  `--force-state-cleanup`, accepting that any container that
  was alive before the daemon went down may survive as an
  orphan.
- Preserving the state file in this scenario buys nothing: the
  reality the file used to mirror is unknown.

`forceOfflineCleanup` therefore:

1. Stops host PIDs we tracked (`stopHostProcesses`).
2. Removes `.raioz.state.json` from the project dir
   (`cleanLocalState`).
3. Removes `raioz.root.json` via `root.Delete(ws)`.

The teardown emits a warning naming the
`com.raioz.project=<name>` label filter so the operator can
recover any orphan containers manually with
`docker rm $(docker ps -a --filter ...)` once the daemon is
back. Issue 032 chose this documentation update over a
breadcrumb-based auto-recovery because the path is opt-in and
the documentation matches the behavior the operator already
signed up for. Cross-references: ADR-029 (typed
`ErrDaemonUnreachable` from infra), `internal/app/down_offline.go`.

## Consequences

### Positive

- Drift detection on the next `up` of an unrelated project compares
  against current state, not a stale snapshot. Drift warnings
  become actionable instead of noisy.
- The state directory stops accumulating per-project debris.
  Important now that ADR-022 anchored the directory location: the
  XDG-conformant default makes the cost of debris cheaper to spot.
- The invariant ("state mirrors reality") gives future state
  additions a default contract. New file under
  `RaiozStateDir()`? It gets a `Delete` and `down` calls it.

### Negative

- Diagnostic data is gone after a successful down. If a user runs
  `raioz down` and the next `up` exhibits something weird, the
  previous snapshot can't be inspected from disk. Mitigated by:
  - The down path skips cleanup when leftovers survive (so failed
    downs preserve the file).
  - `LocalState` still lives in the project directory under
    version control's blast radius, so the audit log + commit
    history are the real diagnostic surfaces.

### Neutral

- `root.Delete(ws)` returns an error but the call site only logs
  warnings. Tests that want to assert cleanup failures need to
  intercept the log; we don't have any such test today and the
  failure mode is rare enough to leave as an integration concern.

## Alternatives considered

- **Auto-detect stale state on the next `up`** (issue 045 option
  B). Cheaper to implement — just skip the load if the project's
  containers don't exist — but it papers over the underlying
  invariant. State files outliving their reality will keep biting
  in subtler ways (drift reports, future tooling that reads the
  file expecting it to be live). The cleanup-on-down option is
  the one that scales.
- **Add `raioz state clean` as a manual command.** Pushes the
  cleanup decision to the user. Same drawback: the file lingers
  by default, every release ships with a "did you remember to
  run state clean?" footnote.
- **Hard delete on every down, even with leftovers.** Simpler but
  loses the only diagnostic signal we have when a teardown
  leaves containers behind. The leftovers gate is cheap and
  preserves that signal at the cost of one Docker label query
  that down already runs.
- **Treat the file as workspace-shared rather than
  project-scoped.** That would require redesigning the storage
  layout. Out of scope here; the current path
  (`workspaces/<project>/raioz.root.json`) is what `up` writes
  and what users have on disk.

## References

- Code: `internal/root/root.go::Delete`,
  `internal/app/down_orchestrated.go` cleanup hook.
- Tests: `internal/root/root_test.go::TestDelete`,
  `internal/app/down_orchestrated_coverage_test.go::TestDownUseCase_downOrchestrated_DeletesRootConfig`.
- Predecessor: [ADR-011](011-runtime-state-single-source.md) Phase
  3 retired the interactive prompt that surfaced the symptom of
  this invariant being violated.
- State-file map (where each file lives, who writes/deletes):
  [docs/STATE.md](../STATE.md).
- Issue: 045.
