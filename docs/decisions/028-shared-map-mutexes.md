# ADR-028: Mutex discipline for shared in-memory maps

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

The quality auditor (issue 059) flagged three maps that are mutated
across multiple call paths without any synchronization:

- `internal/proxy/proxy.go::Manager.routes` — written by `AddRoute`
  / `RemoveRoute`, read by `Start`, `HostsLine`, `Reload`,
  `generateCaddyfile`, `GetURL`, and the persisted-routes paths.
- `internal/orchestrate/host_runner.go::HostRunner.processes` —
  written by `Start`/`SetPID`, deleted by `Stop`, read by
  `Status`/`GetPID`.
- `internal/orchestrate/host_runner.go::HostRunner.launchers` —
  written by `Start` (clean-exit-in-settle-window path, ADR-025),
  read by `Stop`.

Today none of these surfaces are actually concurrent — the use case
loop in `upcase/orchestration.go` serializes dispatch, and the
shared-proxy workspace lock (ADR-010) is taken around file
generation. The maps are race-free **by ambient serialization**, not
by design.

The known pre-bugs:

1. **Watch-multi**: a future feature watching N services in
   parallel would call `dispatcher.Start` from N goroutines —
   immediate write race on `HostRunner.processes`.
2. **Workspace-shared proxy with concurrent up flows in one
   process**: the workspace lock guards file writes, not the
   in-memory `Manager.routes` map.
3. **Any future `raioz daemon` / dashboard with a thicker server
   loop**: reusing `Manager` from goroutines is exactly the
   shape that fails today.

## Decision

### `Manager.routes` — `sync.RWMutex`

Writers (`AddRoute`/`RemoveRoute`) take the write lock; `GetURL`
takes a read lock. Iteration sites (`HostsLine`, `Start`'s
`--network-alias` loop, `generateCaddyfile`, `SaveProjectRoutes`)
call `snapshotRoutes()` which copies the map under RLock and
returns it, so docker exec / file write happens outside the lock.

Helpers (`snapshotRoutes`, `routesCount`) live in
`internal/proxy/routes_access.go` to keep `proxy.go` under the
400-line cap.

### `HostRunner.{processes,launchers}` — `sync.Mutex`

Single mutex guards both maps. Helper methods:

- `recordPID(name, pid)` — write
- `markLauncher(name)` — write
- `isLauncher(name)` — read
- `takePID(name)` — atomic read + delete (used by `Stop`)
- `peekPID(name)` — read (used by `Status`, `GetPID`)

Lazy init under the write lock so a fresh `HostRunner{}` works
without an explicit setup phase. All direct map access replaced
with helper calls.

### Tests

`-race` is the load-bearing assertion. CI already runs unit tests
under `-race`, so removing a mutex fails CI.

- `internal/proxy/proxy_concurrent_test.go` — concurrent
  `AddRoute` + reads, concurrent add/remove on overlapping keys.
- `internal/orchestrate/host_runner_concurrent_test.go` —
  concurrent `SetPID`/`GetPID`, concurrent `markLauncher`/`isLauncher`,
  `takePID` consume-once unit.

## Consequences

### Positive

- Three pre-bug surfaces become race-free by construction.
- Snapshot pattern preserves "long subprocess ops don't block
  other goroutines" — important for `Start`'s docker exec.
- Helper-based access is uniform: grep `r.processes[` returns
  zero non-test matches now, all access goes through methods.
- CI `-race` is the permanent regression guard.

### Negative

- ~70 LoC added across two packages with no user-visible change.
  Purely defensive engineering.
- The snapshot allocates a fresh map per iteration. Trivial cost
  for a CLI (routes map is 10-100 entries) but worth noting if
  the proxy ever becomes a long-running daemon.

### Neutral

- Three locking scopes now have consistent answers in the repo:
  per-goroutine in-process (this ADR), per-process
  (`internal/lock`), cross-process (`internal/proxy/workspace_lock`).

## Alternatives considered

- **`sync.Map`**. Rejected for `Manager.routes` because iteration
  sites need ordered range semantics. Rejected for `HostRunner`
  because the access pattern is dominated by `Get`/`Set`+delete
  where `sync.Map` doesn't outperform a regular mutex.
- **Lock-free atomic.Pointer to a copy-on-write map**. More
  scalable for many-readers, but the maps are tiny and writes are
  short. Premature optimization.
- **Document "single-goroutine only" and skip locks**. Rejected
  because the surface keeps growing (ADR-025 added `launchers`)
  and the next addition silently introduces the race.

## References

- Code: `internal/proxy/proxy.go`,
  `internal/proxy/routes_access.go`,
  `internal/proxy/caddyfile.go`,
  `internal/proxy/routes_persist.go`,
  `internal/orchestrate/host_runner.go`.
- Tests: `internal/proxy/proxy_concurrent_test.go`,
  `internal/orchestrate/host_runner_concurrent_test.go`.
- Issue: 059.
- Related: [ADR-005](005-workspace-shared-proxy.md),
  [ADR-010](010-proxy-workspace-lock.md),
  [ADR-025](025-launcher-pattern-container-wait.md).
