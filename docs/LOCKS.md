# Lock matrix

raioz coordinates work across goroutines, processes, and parallel
`raioz up` invocations in the same workspace. This page is the
single source for "what serializes what." Companion to the ADRs
listed at the bottom — those argue *why* each lock exists; this
page documents *how they interact*.

## At a glance

| Lock | Scope | Protects | Acquired in | Bypass | Hierarchy |
|------|-------|----------|-------------|--------|-----------|
| **`lock.Acquire(ws)`** | per-project (file) | `raioz up` lifecycle / state writes for one project | `usecase.Execute` (ADR-008) | Yes — siblings (via `RAIOZ_SIBLING_STACK`) | 1 (outermost) |
| **`Manager.acquireWorkspaceLock`** | per-workspace (`flock` + per-process mutex) | `Caddyfile` + routes dir + Caddy reload | `SaveProjectRoutes` / `RemoveProjectRoutes` / `Reload` (ADR-005, ADR-010) | No — siblings included | 2 |
| **`Manager.routesMu`** | per-process (`sync.RWMutex`) | `Manager.routes` map in memory | `AddRoute` / `RemoveRoute` / `GetURL` / `snapshotRoutes` (ADR-028) | n/a | 3 (innermost) |
| **`HostRunner.mu`** | per-process (`sync.Mutex`) | `processes` + `launchers` maps (ADR-025) | `recordPID` / `markLauncher` / `takePID` / `peekPID` / `isLauncher` (ADR-028) | n/a | 3 (innermost) |
| **`processProxyMu`** | per-process (`sync.Mutex`) | Closes flock's per-process gap so two goroutines can't both grab the workspace lock (ADR-010) | inside `acquireWorkspaceLock` | n/a | nested under workspace lock |
| **`RAIOZ_SIBLING_STACK`** | cycle detection (env var, ADR-008) | Detects A→B→A recursion in mode A sibling spawn | `checkSiblingCycle` in `sibling_spawn.go` | n/a | not a lock — env propagation |
| Docker daemon | external | Container/network/volume create | n/a — daemon serializes | n/a | n/a |

Levels 1-3 are nested in order: the up flow takes level 1, then
level 2 inside it (transiently — see "Hold times" below), then
level 3 around in-memory mutations.

## Rules

1. **Acquire in scope order.** Project lock first, workspace lock
   second, in-process mutex third. Never go bottom-up — the same
   goroutine grabbing routesMu and then workspace lock could
   deadlock if another goroutine is mid-rotation.

2. **Workspace lock is never bypassed.** Sibling spawns
   (ADR-008 mode A) bypass the *project* lock because the parent
   already holds it; the *workspace* lock is per-workspace, not
   per-project, so siblings take it independently. Without this,
   two projects in the same workspace would race on Caddyfile
   regeneration.

3. **Workspace lock hold time is short.** It guards the routes-dir
   write + Caddy reload — both file/exec work measured in
   milliseconds. Never hold it across a long subprocess (docker
   pull, image build); use `snapshotRoutes()` to copy the routes
   map and release the lock before the slow work.

4. **The in-process mutex (`routesMu`, `HostRunner.mu`) does NOT
   span process boundaries.** It guards the in-memory maps for
   goroutines in the same `raioz` process. Cross-process
   coordination of files goes through the workspace lock (file
   `flock`); cross-process coordination of state files goes
   through `naming.RaiozStateDir()` plus the migrator (ADR-022).

5. **`os.Executable()` spawn inherits env (`RAIOZ_SIBLING_STACK`,
   `RAIOZ_CORRELATION_ID`) but NOT file descriptors held by Go's
   `os.OpenFile`.** The child opens its own file handle when it
   re-acquires the workspace lock — that's how cross-process
   serialization actually works. Advisory `flock` on Linux is
   per-process, not per-handle, so the in-process mutex
   (`processProxyMu`) closes the gap for sibling code paths in
   the same Go process.

6. **Tests that exercise cross-process serialization must spawn
   real subprocesses.** `exec.Command(os.Executable(), …)` with a
   shared workspace tempdir is the only way to reproduce the
   flock contention. Goroutines in the same process hit the
   mutex; goroutines across processes hit the file lock.
   `TestSpawnSibling_PdeathsigKillsOrphans` is a precedent.

7. **`Pdeathsig` (ADR-026, Linux only) is unrelated to lock
   ordering** — it's a kernel-level cleanup signal. It fires when
   the parent exits, regardless of whether the parent released
   its lock first. Don't try to use Pdeathsig as a lock; it
   doesn't serialize anything.

## Sequence — `raioz up` with a sibling project

```
parent: raioz up
  acquireLock(ws)                  ← project lock (level 1)
    bootstrap, validate, prep
    processOrchestration:
      applySiblingVerdict:
        spawnSibling(ctx, ...)      ← recursive raioz up
          [child inherits RAIOZ_SIBLING_STACK]
          child: raioz up
            acquireLock(ws)         ← SKIPPED (sibling, env-detected)
            bootstrap, validate, prep
            processOrchestration:
              dispatch deps          ← starts containers
              startProxy
                ProxyManager.AddRoute (routesMu)        ← level 3
                SaveProjectRoutes  → acquireWorkspaceLock ← level 2
                  generateCaddyfile (snapshotRoutes)
                  os.WriteFile(Caddyfile)
                release()                               ← level 2 freed
                ProxyManager.Start (docker run)
            return
          [child exits]
      back in parent:
      dispatch deps                ← starts containers
      startProxy
        ProxyManager.AddRoute      ← level 3
        SaveProjectRoutes  → acquireWorkspaceLock      ← level 2
          generateCaddyfile (which NOW sees both
          parent + child routes via the persisted
          routes dir — ADR-005)
          os.WriteFile(Caddyfile)
        release()
        ProxyManager.Reload  → acquireWorkspaceLock    ← level 2 (re-take)
          docker exec caddy reload
        release()
  defer release()                  ← project lock released
```

The workspace lock is taken and released **three** times in the
parent here (once in the child too): SaveProjectRoutes ×2 +
Reload ×1. Each acquisition is short. The project lock is held
across the whole `up` for the parent but bypassed in the child.

## Hold-time policy

| Lock | Typical hold time | Why short matters |
|------|-------------------|-------------------|
| Project lock | full `up`/`down` duration (seconds to minutes) | Concurrent ups of the same project would corrupt state; long hold is fine. |
| Workspace lock | milliseconds (one file write or one Caddy reload) | Several projects share it; long hold means everyone waits. |
| `routesMu` | microseconds (one map op or one map copy) | All readers/writers in one process. |
| `HostRunner.mu` | microseconds | Same. |
| `processProxyMu` | nested inside workspace lock | Same. |

## Audit cases

### `raioz dev` (dev promote/revert)

`dev.go` calls `dispatcher.Start`/`Stop` to swap a dep container
for a local service. It does **not** touch the proxy state, the
routes dir, or `AddRoute`/`RemoveRoute`. Conclusion: `dev` is
proxy-lock-free by design. If a future change has `dev` register
a new proxy route (e.g. the promoted local also wants HTTPS),
add an `AddRoute` call inside the existing project-lock critical
section — same shape as the up flow.

### `raioz down --conflicting` / `--all-projects`

`down_others.go::stopProjects` spawns subprocess `raioz down` for
each conflicting project via `exec.Command(raiozBin, "down")`.
Each subprocess goes through its own
`DownUseCase.Execute` → `acquireLock(ws)` → `stopProxy()` →
`RemoveProjectRoutes()` (which takes the workspace lock
internally). The parent `down --conflicting` itself does NOT
hold any lock while the children run — the lock contention is
between siblings, not between parent and siblings. Conclusion:
the model is preserved; no special handling needed.

### Sibling spawn — parent's lock vs child

The parent's project lock is **held** while it spawns siblings
(see sequence above). The child detects this via
`RAIOZ_SIBLING_STACK` and skips its own project-lock acquisition
(otherwise it would deadlock waiting for the parent). This is
the load-bearing bypass: lose it and mode A breaks.

The child still takes the workspace lock when it touches the
routes dir — that lock is per-workspace, and the parent's
project lock is per-project. They don't collide.

### Failure mode — parent SIGKILL and stale project lock

`internal/lock/lock.go` uses `O_EXCL` + PID file (not `flock`) for
portability with Windows. If a raioz process is SIGKILL'd the
file survives. The acquisition path treats the lock as stale and
sweeps it in two cases:

1. **PID no longer exists** — `isProcessRunning` returns false
   (`os.FindProcess` + `Signal(0)`). Common case.
2. **Lock file older than `staleLockMaxAge` (24h)** — defense
   against PID wraparound landing on a non-raioz process that
   keeps the PID alive. A legitimate raioz session, including
   `raioz dashboard` watch mode, rarely survives that long, so
   the bound is generous in practice. Issue 075.

The workspace proxy lock (`flock` via
`internal/proxy/workspace_lock.go`) does not need this defense —
the kernel releases the advisory lock automatically when the
process holding it terminates, regardless of signal.

### Test infrastructure

Cross-process tests for the workspace lock live in
`internal/proxy/`. The cross-OS implementation
(`workspace_lock_unix.go::syscall.Flock` vs
`workspace_lock_windows.go::LockFileEx`) is exercised by the
Windows runtime CI job (ADR-030) on push to develop/main.

## Adding a new mutator of workspace state

If a feature touches the workspace's persisted state (routes dir,
Caddyfile, workspace-shared deps, future shared assets):

1. Take the **workspace lock** (`acquireWorkspaceLock` via
   `Manager`). Always via `defer release()`.
2. Take any **in-process** mutex you need to read shared maps
   (`routesMu` for routes, `HostRunner.mu` for PIDs).
3. Hold both for the minimum work — file write, map mutation —
   then release.
4. Do slow work (docker exec, network IO) **outside** the
   workspace lock, using `snapshotRoutes()` or equivalent
   copy-then-iterate.
5. If the new code spawns sub-`raioz` processes (mode A or
   meta), propagate `RAIOZ_SIBLING_STACK` so the child
   correctly bypasses its project lock.

## References

- [ADR-005](decisions/005-workspace-shared-proxy.md) — workspace-shared proxy lifecycle.
- [ADR-008](decisions/008-sibling-projects-as-deps.md) — recursive sibling spawn and `RAIOZ_SIBLING_STACK`.
- [ADR-010](decisions/010-proxy-workspace-lock.md) — proxy workspace lock + `processProxyMu`.
- [ADR-026](decisions/026-signal-handling-and-pdeathsig.md) — `signal.NotifyContext` + `Pdeathsig` (orphan cleanup, not lock ordering).
- [ADR-028](decisions/028-shared-map-mutexes.md) — `routesMu` / `HostRunner.mu`.
- [ADR-030](decisions/030-windows-ci-on-push.md) — cross-OS lock semantics tested on Windows.
- Code: `internal/lock/`, `internal/app/upcase/lock.go`,
  `internal/proxy/workspace_lock.go`,
  `internal/proxy/workspace_lock_unix.go`,
  `internal/proxy/workspace_lock_windows.go`,
  `internal/proxy/routes_access.go`,
  `internal/orchestrate/host_runner.go`,
  `internal/app/upcase/sibling_spawn.go`.
- Issue: 051.
