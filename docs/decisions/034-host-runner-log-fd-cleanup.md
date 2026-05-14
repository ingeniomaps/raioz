# ADR-034: HostRunner drain goroutine owns the log fd

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

`HostRunner.Start` (in `internal/orchestrate/host_runner.go`) opens
a `*os.File` for the service's log, hands it to `cmd.Stdout` /
`cmd.Stderr`, and starts the child. After fork+exec the **child
inherits a duplicate** of the fd via Go's `os/exec` plumbing; the
parent retains its own copy. The parent must close that copy or
the fd stays open in the raioz process until the GC finalizer
fires.

Three exit paths existed in `Start`:

1. `cmd.Start` itself fails → explicit `logFile.Close()` (line
   157, before the issue-061 fix).
2. The child exits non-zero inside the settle window → explicit
   `logFile.Close()` then return an error.
3. The child survives the settle window → fall through to
   `r.recordPID(...)`. **No close.** The comment said the
   finalizer would handle it, but the closure capturing `cmd` (and
   transitively `cmd.Stdout = logFile`) kept the `*os.File` reachable
   until the child died, so the finalizer never ran while the
   service was up.

Issue 061 flagged this for long watch-mode sessions: every restart
opens a new fd, the old one waits for the next GC scan after the
child exits. Over hours of file-save-driven restarts, the parent
accumulates handles.

## Decision

Move `logFile.Close()` into the wait goroutine. After this change,
`Start` looks like:

```go
waitCh := make(chan error, 1)
go func() {
    err := cmd.Wait()
    _ = logFile.Close()  // release parent's copy of the fd
    waitCh <- err
}()
```

The drain goroutine is the single owner of the parent-side log
handle. Every exit path — clean-exit-in-window (launcher
pattern), error-in-window, survived-window — converges on the
goroutine releasing the fd exactly once, when `cmd.Wait()` returns.

The explicit `logFile.Close()` on the error path is removed (the
goroutine closes first; calling Close twice on an `*os.File` is
harmless but the duplication invited confusion). The pre-`cmd.Start`
failure path keeps its explicit Close because no goroutine has been
spawned at that point.

## Implementation status

Landed in this commit:

- `internal/orchestrate/host_runner.go::Start` — drain goroutine
  closes `logFile`; comments updated.
- `internal/orchestrate/host_runner_fd_linux_test.go` (new) —
  Linux-only regression test. Starts a 1-second `sleep`, waits
  out the settle window, polls `/proc/self/fd` for the log path's
  symlink, and asserts the fd disappears within 5s of child exit.

## Consequences

### Positive

- Long-running watch-mode sessions no longer accumulate file
  handles. Restart count is no longer bounded by `ulimit -n`.
- The cleanup is deterministic: fd release happens at child
  death, not at GC. Tests can assert on it (regression pinned via
  `/proc/self/fd`).
- Lock-free coordination: the goroutine that knows when cmd died
  is also the one with the fd to close.

### Negative

- The drain goroutine now does two things (read exit + close fd)
  instead of one. Trivial — still one statement each.
- The "clean exit during settle window" path used to be the only
  one where the goroutine completed quickly; now it always
  completes "at child death" which is the same as before for the
  long-running case. No behavior change for users.

### Neutral

- The regression test is Linux-only because `/proc/self/fd` is the
  cheapest cross-Go-version way to enumerate open fds.
  macOS/Windows runtime behavior is the same (Go's os/exec dups
  the fd identically on every OS); a missing regression test on
  those platforms is acceptable given the bug is OS-independent
  and the cross-compile gate already catches build-time forks.

## Alternatives considered

- **Restructure `Start` into a `launchSettling` helper that returns
  a launch result enum + per-result cleanup goroutines** (the
  issue 061 "alternativa más limpia"). Rejected: more surface area
  changes for the same end state. The drain goroutine already had
  the only piece of state that needed to know about cleanup (the
  fd); promoting it to owner adds zero new types.
- **Close logFile inline after the timeout case** and skip the
  goroutine's close. Doesn't cover the clean-exit-during-window
  path without a second close site. Worse than centralizing.
- **Defer the close at the top of Start.** Doesn't work — the fd
  must outlive Start's stack frame because the child keeps writing
  until it dies.

## References

- Code: `internal/orchestrate/host_runner.go::Start` (drain
  goroutine), `internal/orchestrate/host_runner_fd_linux_test.go`
  (regression).
- Issue: 061.
- Related: ADR-025 (launcher pattern), ADR-028 (HostRunner
  map mutex — orthogonal concurrency invariant).
