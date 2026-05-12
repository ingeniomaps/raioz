# ADR-010: Workspace-shared proxy operations serialize on a dedicated lock

- **Status:** Accepted
- **Date:** 2026-05-12

## Context

ADR-005 introduces a workspace-shared proxy: one Caddy fronts every
project in a workspace, with per-project route files unioned into a
single Caddyfile. Two `raioz up` invocations in the same workspace
(a documented use case) can mutate this shared state concurrently.

Wave 0 issue 021 made each per-project route file write atomic via
temp + rename, eliminating the "reader sees a half-written JSON"
class of bug. But the multi-step Caddyfile pipeline —
`Reload = generateCaddyfile + ask Caddy admin API to reload` — is
still racy:

1. Process A renames `routes/a.json` to the new version.
2. Process B starts generating the Caddyfile, reads A's NEW version
   plus B's OLD version, writes Caddyfile.v1.
3. Process A finishes its own generate, writes Caddyfile.v2.
4. Both processes reload Caddy. Last writer wins; either Caddyfile
   is internally consistent, but the order is non-deterministic and
   the intermediate state on disk has lived for a millisecond.

There is also no signal to a contributor reading the code that the
multi-step operation must serialize. New code paths can be added
that touch the routes dir or Caddyfile without realizing they need
the same discipline.

The existing `internal/app/upcase/lock.go` is the wrong tool for
this: it guards a *project's* up lifecycle and is intentionally
bypassed when raioz spawns recursively as a sibling project
(`RAIOZ_SIBLING_STACK` cycle detection). The artifact this ADR
protects is *workspace-scoped*, not project-scoped, and must be
honored from sibling spawns too — otherwise an A→B→A spawn chain
can race itself.

## Decision

Introduce a separate lock dedicated to workspace-shared proxy
state. Implementation lives in `internal/proxy/workspace_lock.go`
as `(*Manager).acquireWorkspaceLock()`. It returns an idempotent
release function; callers `defer release()` immediately after a
successful acquire.

Lock scope: a single file at
`${WorkspaceProxyDir()}/.proxy.lock`, held exclusively via
`syscall.Flock(LOCK_EX)`. Per-project mode (no workspace
declared) is a no-op: there is no shared state to guard.

Two layers of mutual exclusion:

1. **Cross-process** — `syscall.Flock` on the lock file.
   Different raioz processes block on each other.
2. **In-process** — a package-level `sync.Mutex`
   (`processProxyMu`). Linux flock is per-process, so two
   goroutines in the *same* process can both grab `LOCK_EX`
   without blocking. The mutex closes that gap.

Sites that must take the lock:

- `Manager.SaveProjectRoutes` (writes per-project file).
- `Manager.RemoveProjectRoutes` (deletes per-project file).
- `Manager.Reload` (regenerate Caddyfile + ask Caddy to reload).

`Manager.generateCaddyfile` is a private helper called from
`Reload`; it does not take the lock itself (the caller already
holds it). `Manager.RemainingProjects` is a pure read of file
counts and does not take the lock either — atomic rename in
`SaveProjectRoutes` (Wave 0) already guarantees consistent
reads.

The lock is taken even when `RAIOZ_SIBLING_STACK` is set. The
cycle-detection bypass in `upcase/lock.go` exists because the
*up lifecycle* lock must be released for siblings; the *proxy
state* lock has no such reason — siblings touching the shared
Caddyfile must serialize with whoever else is touching it.

## Consequences

### Positive

- A second `raioz up` in the same workspace blocks at the proxy
  step instead of producing a non-deterministic Caddyfile.
- The lock makes the serialization point explicit. New code in
  `internal/proxy/` that touches the routes dir or Caddyfile
  has an obvious model to follow.
- Sibling spawns (mode A) participate in the same lock, so
  recursive raioz invocations don't race themselves.
- Process death drops the flock at OS-level cleanup; no stale
  lock files to clean up manually.

### Negative

- `syscall.Flock` is Unix-only. Windows raioz users would need
  a different impl. Acceptable because the broader
  workspace-shared proxy feature already targets Linux primarily
  (`proxy.publish: false` is documented Linux-only).
- A goroutine that panics inside the critical section without a
  proper `defer release()` leaks the lock until the process
  exits. Standard Go advice (defer immediately on acquire) is
  enough to avoid this in practice.
- The lock can hide a real deadlock: if a contributor adds a
  new helper that internally calls a public lock-taking method
  while already holding the lock, the in-process mutex makes
  this a deadlock instead of a race. Mitigation: keep the
  surface that takes the lock small (3 methods today) and
  documented.

### Neutral

- The lock file accumulates in the workspace proxy dir but is
  ~0 bytes and gets reused across runs. No cleanup needed.

## Alternatives considered

- **Reuse `internal/lock/`** — uses a PID-file with O_EXCL +
  stale-PID detection. Functional but adds the historical
  race window where two processes create the file at the same
  instant before checking ownership. Flock at the OS level
  avoids that class of bug entirely.
- **`github.com/gofrs/flock`** — wraps `syscall.Flock` for
  portability. Equivalent semantics with an external dep.
  Rejected for now because raioz already relies on syscall
  directly elsewhere and adding a dep for ~30 lines of code
  felt disproportionate.
- **Single global mutex in the Manager** — would only help
  within one process. Doesn't address the cross-process race
  that motivated the ADR.
- **Lock every read too** (`loadAllProjectRoutes`,
  `RemainingProjects`) — defensible defense-in-depth, but with
  Wave 0's atomic rename guaranteeing read consistency, the
  marginal value didn't justify the contention added on hot
  read paths (status/down).

## References

- Code: `internal/proxy/workspace_lock.go`,
  `internal/proxy/workspace_lock_test.go`,
  `internal/proxy/routes_persist.go`,
  `internal/proxy/proxy.go` (`Reload`)
- Related: ADR-005 (workspace-shared proxy lifecycle),
  ADR-008 (sibling raioz projects — bypass exemption)
- Originated from: docs/issues/025-proxy-lock.md
