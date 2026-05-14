# ADR-026: Parent signal handling + Pdeathsig for sibling spawns

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

raioz' CLI entrypoint (`internal/cli/root.go::Execute`) ran cobra's
top-level command without wrapping it in a signal-aware context.
Every `cmd.Context()` site downstream therefore observed an
un-cancellable `context.Background()`, and Ctrl+C reaching the
parent terminated the parent process but did nothing to the
descendants the parent had spawned.

The architecture-reviewer's quality-auditor pass flagged this as
the single 🔴 with deploy impact in issue 057. The concrete
failure mode chain (ADR-008 mode A — recursive `raioz up`
spawning siblings):

1. User runs `raioz up` in project `A`.
2. `A` declares `dependencies.foo.project: ../B` (mode A).
3. `applySiblingVerdict` calls `spawnSibling(ctx, …)` which runs
   `exec.CommandContext(ctx, raiozBinary, "up")` with `cmd.Dir = ../B`.
4. `B` is in the middle of bringing up its own deps + containers.
5. User Ctrl+C's the parent.
6. The kernel sends SIGINT to the parent process only. The parent
   exits. `B`'s `raioz` process — and `B`'s children (containers
   half-up, sub-siblings if `B` itself spawned `C`) — keep running
   with no parent to wait for them.

Two structural gaps:

- **No ctx cancellation.** Cobra's `cmd.Context()` returned
  `context.Background()` (cobra falls back to that when none is
  set). `cmd.Wait()` in `spawnSibling` never sees a `Done()`
  signal because the bound `exec.CommandContext` ctx is the same
  un-cancellable one.
- **No parent-death signal on the spawned binary.** Even with
  cancellation in place, a kill -9 of the parent would still
  leave the child alive (cancellation rides on the parent's
  goroutines, which are gone). The Linux `prctl(PR_SET_PDEATHSIG)`
  syscall asks the kernel to send a signal to the child when its
  parent dies — orphan prevention at the kernel level rather than
  the user-space level.

## Decision

Two coordinated halves.

### Part A — Signal-aware root context

`Execute` builds a context from `signal.NotifyContext(ctx,
os.Interrupt, syscall.SIGTERM)` and passes it to
`rootCmd.ExecuteContext(ctx)`. Cobra propagates the context to
every `cmd.Run`/`RunE`. The `cmd.Context()` API that downstream
code already calls now returns a context whose `Done()` channel
closes on Ctrl+C / SIGTERM.

```go
func Execute() {
    ctx, stop := signal.NotifyContext(
        context.Background(), os.Interrupt, syscall.SIGTERM,
    )
    defer stop()
    if err := rootCmd.ExecuteContext(ctx); err != nil {
        fmt.Print(errors.FormatError(err))
        os.Exit(1)
    }
}
```

Zero changes at call sites — every existing `cmd.Context()` is
already correct. The fix is one place.

### Part B — Pdeathsig on Linux

`spawnSibling` sets `cmd.SysProcAttr` and delegates to a
platform-specific `setPdeathsig` helper:

```go
// sibling_spawn_linux.go
//go:build linux
func setPdeathsig(attr *syscall.SysProcAttr) {
    attr.Pdeathsig = syscall.SIGTERM
}

// sibling_spawn_other.go
//go:build !linux
func setPdeathsig(_ *syscall.SysProcAttr) {}
```

In the spawn:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{}
setPdeathsig(cmd.SysProcAttr)
```

Linux: the kernel sends SIGTERM to the child the moment the
parent process exits, regardless of how the parent died (clean
exit, SIGINT, SIGKILL, OOM). The child observes the signal and
its own `signal.NotifyContext`-derived ctx cancels — recursive
case covered automatically because the child uses the same
Execute path.

macOS and Windows fall back to the portable half (ctx
cancellation from Part A). Clean exit propagates fine; a hard
kill of the macOS parent still leaks because there's no
equivalent of `Pdeathsig`. Mitigations explored and deferred:

- **macOS kqueue parent-PID monitoring.** Watch the parent's
  PID via kqueue and exit on EV_PROC | NOTE_EXIT. Doable; cost
  is a long-lived goroutine per child raioz. Deferred — the
  90% case is Linux dev boxes.
- **Windows Job Objects.** Wrap the child in a job with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. Different infrastructure
  (`golang.org/x/sys/windows`); also deferred.

## Implementation status

Landed in this commit:

- `internal/cli/root.go::Execute`: `signal.NotifyContext` + `ExecuteContext`.
- `internal/app/upcase/sibling_spawn.go`: `cmd.SysProcAttr` +
  `setPdeathsig` call.
- `internal/app/upcase/sibling_spawn_linux.go`: `setPdeathsig`
  with `Pdeathsig = SIGTERM`.
- `internal/app/upcase/sibling_spawn_other.go`: no-op fallback.
- Tests:
  - `sibling_spawn_linux_test.go::TestSetPdeathsig` — verifies the
    Linux setter populates `Pdeathsig`.
  - `sibling_spawn_test.go::TestSpawnSibling_PdeathsigKillsOrphans` —
    end-to-end: long-running child reaped within 5 s when ctx
    cancels.

## Consequences

### Positive

- Ctrl+C on `raioz up` cleans the entire spawn tree on Linux.
  No more orphan `raioz` processes lingering after an aborted
  recursive up.
- The portable half (Part A) means every cobra command now
  honors cancellation. Long-running operations (watch mode,
  attach mode, future `tunnel up`) get the right behavior
  automatically.
- Zero call-site changes for Part A. The contract is fixed at the
  root.

### Negative

- macOS and Windows users still leak hard-killed children. The
  portable half mitigates Ctrl+C; SIGKILL of the parent still
  orphans descendants on those platforms.
- One more cross-platform seam (`setPdeathsig`) to maintain. The
  pattern matches `proctree_unix.go` / `proctree_windows.go`
  already in `internal/host/`.

### Neutral

- The cross-compile gate added in v0.5.1's CI (`ci.yml`) catches
  the most common regression class for this seam — a future
  `setPdeathsig` change that uses a Linux-only field without a
  build tag fails the windows-amd64 build, not the release.

## Alternatives considered

- **Hard kill the child explicitly in a defer.** Adds a fragile
  PID-tracking layer in the parent and doesn't help when the
  parent gets SIGKILL'd. Pdeathsig is the kernel-level answer
  that survives all parent-exit modes.
- **Run sibling raioz in the same process via library call.**
  Closer to the architecture-reviewer's "shared workspace model"
  but rejected (ADR-008): mode A's process isolation is
  load-bearing for env hygiene and crash containment.
- **prctl on every spawn**, not just sibling. The launcher
  pattern (ADR-025) and host-runner already use process groups;
  Pdeathsig on top of that would double up. Limit to sibling
  spawn where the orphan-tree concern is genuinely new.

## References

- Code: `internal/cli/root.go::Execute`,
  `internal/app/upcase/sibling_spawn.go`,
  `internal/app/upcase/sibling_spawn_linux.go`,
  `internal/app/upcase/sibling_spawn_other.go`.
- Tests: `internal/app/upcase/sibling_spawn_linux_test.go`,
  `internal/app/upcase/sibling_spawn_test.go::TestSpawnSibling_PdeathsigKillsOrphans`.
- Issue: 057.
- Predecessor ADRs: [ADR-008](008-sibling-projects-as-deps.md)
  (mode A spawn lifecycle), [ADR-025](025-launcher-pattern-container-wait.md)
  (launcher process tree handling on Linux).
