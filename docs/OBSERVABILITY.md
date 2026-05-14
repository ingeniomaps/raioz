# Observability — logging, audit, notify, output

Raioz has four packages that emit signals. Each owns one job. The
intent of this doc is to make "which channel?" a 30-second decision
instead of an ad-hoc one.

| Package | Where it goes | Audience | Persistence |
|---------|---------------|----------|-------------|
| [`internal/logging`](../internal/logging/) | stderr (slog text/JSON) | developer debugging | none — re-run with higher level |
| [`internal/audit`](../internal/audit/) | `<state>/audit.log` (JSONL) | future you / postmortems | append-only, rotated at 10 MiB (ADR-020) |
| [`internal/notify`](../internal/notify/) | desktop notification | the user *not looking at the terminal* | OS-managed, transient |
| [`internal/output`](../internal/output/) | stdout (ANSI colored) | the user *looking at the terminal* | none — scroll buffer |

## When to use what

The rule of thumb, in priority order: ask **who is going to read
this, and when?**

- **Right now, in the foreground →** `output`. The user is
  watching. A success tick, an error block, a table.
- **Right now, but they're elsewhere →** `notify`. Long-running
  command finished, watch promoted a service, dev mode just
  flipped on. Don't use for routine status.
- **Later, when reconstructing what happened →** `audit`. "When
  did the proxy last restart? Who promoted postgres to dev?"
  Append-only history.
- **Later, when debugging an internal problem →** `logging`. Wire
  detail, retries, decisions raioz made. Assume nobody reads
  unless something already broke.

These are not mutually exclusive — a `raioz up` success usually
emits **all four** (info log + audit entry + notify success +
output tick).

## Channel rules

### `internal/logging` — structured logs

slog-backed; `Debug`/`Info`/`Warn`/`Error` plus `WithContext`
variants that pull request-scoped fields out of `context.Context`.

- **Debug:** wire-level detail. Docker commands invoked, config
  paths read, retries attempted, decisions ("falling back to
  fallback path because /opt is read-only"). Verbose by intent;
  this is what `--log-level=debug` is for.
- **Info:** lifecycle events. Service started, dependency pulled,
  proxy reload completed. Should match the audit log for events
  that are also audited.
- **Warn:** recoverable anomalies. Notification tool missing,
  config field deprecated but still honored, host service exited
  cleanly inside the settle window.
- **Error:** anything that fails the command. Pair with a return
  to the caller — `Error()` is not a substitute for returning the
  error.

**Never user-facing.** Logs go to stderr. The default level is
`info` and 99% of users never see them — assume nobody reads logs
unless they're already debugging.

```go
logging.InfoWithContext(ctx, "Starting host service",
    "service", svc.Name, "command", command, "path", svc.Path)
```

### `internal/audit` — authoritative history

Append-only JSONL at `<state>/audit.log` (rotated to `.1` at 10
MiB; see ADR-020). One event per line, schema is
[`audit.Event`](../internal/audit/audit.go).

- **MUST audit:** anything the user might query later, or anything
  with downstream consequence. Service started, dependency added,
  config root changed, dev mode promoted/reverted, drift detected,
  conflict resolved.
- **MUST NOT audit:** debug/internal events that aren't actions
  (file watch ticks, docker command invocations, env var
  resolution, health probes).

The litmus test: *"if a teammate asks me in six months what
happened, would this entry help?"* If yes → audit. If "you'd need
to re-run with debug logging to see this" → log only.

Use the typed helpers (`audit.LogDependencyAdded`,
`audit.LogConfigChanged`, …) rather than raw `audit.Log` — each
helper enforces the payload shape its consumer expects.

### `internal/notify` — desktop interrupt

`notify.Send(title, message)`. Cross-platform (notify-send on
Linux, osascript on macOS); silently no-ops if the underlying
tool is missing.

- **Use for:** events the user wants to be told about while
  they're working in another window. Currently: `raioz up`
  completion (ready / failed). Good candidates: watch promoted a
  service after the first restart, dev mode flipped, a long
  snapshot finished.
- **Don't use for:** routine status (every file-change tick, every
  config reload, every health check). Notifications must remain
  rare enough that users don't filter them out.

Notifications are best-effort and **already silent in tests**
(`testing.Testing()` guard). Never branch business logic on
`notify.Send` succeeding.

```go
if err := uc.Execute(ctx); err == nil {
    notify.Send("Raioz", i18n.T("up.notify_ready"))
} else {
    notify.Send("Raioz", i18n.T("up.notify_failed"))
}
```

### `internal/output` — interactive feedback

The user-facing terminal voice. `PrintSuccess`, `PrintWarning`,
`PrintError`, `PrintInfo`, plus formatters (`FormatDuration`,
status tables) and the bubbletea TUI for `raioz dashboard`.

- **Use for:** anything the user reads as the command runs —
  progress, success ticks, errors. Render through
  `output.Print*`, never raw `fmt.Println` (which bypasses color +
  formatting + indentation conventions).
- **Errors** flow through `internal/errors.RaiozError`, which
  already knows how to render itself with suggestion text. Don't
  call `output.PrintError` for an error that's about to be
  returned — let the CLI layer surface it.
- **Strings** must go through `i18n.T()`. No raw English in
  `output.Print*` arguments.

```go
output.PrintSuccess(i18n.T("up.proxy_ready", domain))
output.PrintWarning(i18n.T("up.dep_unhealthy", dep))
```

## Event matrix

`audit` column legend: **yes** — emitted today, **planned** —
target shape, helper missing or unused. Issue 048 tracks the
gap between this matrix and reality.

| Event | logging | audit | notify | output |
|-------|---------|-------|--------|--------|
| `up` lifecycle start | `info` | **yes** | — | progress line |
| `up` lifecycle complete | `info` / `error` | **yes** | success/failure | `[ok]`/`[error]` |
| `down` lifecycle start | `info` | **yes** | — | progress line |
| `down` lifecycle complete | `info` / `error` | **yes** | success/failure | `[ok]`/`[error]` |
| `restart` lifecycle start/complete | `info` | **yes** (YAML mode) | — | progress |
| Recoverable warning | `warn` | planned | — | `[!!]` line |
| Service marked ready by health check | `info` | planned | — | `[ok]` row |
| Service exit inside settle window | `warn` | planned | — | `[!!]` block |
| File-watch event (debounced) | `debug` | — | — | optional progress |
| Watch-triggered restart | `info` | planned | — | `[ok]` row |
| Config reloaded | `info` | planned | — | brief note |
| Network / volume created | `debug` | — | — | — |
| Docker command invoked | `debug` | — | — | — |
| Dev-mode promotion (`raioz dev`) | `info` | **yes** | success | confirmation block |
| Dev-mode revert (`raioz dev --reset`) | `info` | **yes** | — | confirmation line |
| Dependency assist added a dep | `info` | **yes** | — | brief note |
| Sibling project deferred (mode A/B) | `info` | **yes** | — | one-line note |
| Drift detected vs `.raioz.json` | `warn` | **yes** | — | `[!!]` summary |
| Conflict resolved by user choice | `info` | **yes** | — | confirmation |
| Dev-build warning (ADR-021) | — | — | — | stderr warning (once) |
| Migration of legacy state dir (ADR-022) | `debug` | — | — | — |

### Correlation across `raioz` processes

Every audit event carries `correlation_id`, sourced from
`logging.GetRequestID(ctx)` (which honors the
`RAIOZ_CORRELATION_ID` env var). Recursive `raioz up` invocations
(ADR-008 mode A sibling spawn) inherit the parent's ID, so
`jq 'select(.correlation_id == "abc")'` on `audit.log`
reconstructs the whole spawn tree from one query.

## Worked example: `raioz up` brings up `api`

When the api service transitions from "stopped" to "running and
healthy", four channels fire:

```go
// 1. logging — wire-level info, structured
logging.InfoWithContext(ctx, "Service started",
    "service", "api", "runtime", "host", "pid", pid)

// 2. audit — authoritative history. Helper enforces payload shape.
audit.LogConfigChanged(workspace, []string{"api: started"})

// 3. notify — only after the whole `up` finishes, not per service.
// Triggered once from internal/cli/up.go after Execute returns nil.
notify.Send("Raioz", i18n.T("up.notify_ready"))

// 4. output — the user-facing tick the dev sees in the terminal
output.PrintSuccess(i18n.T("up.service_ready", "api"))
```

If health check fails, the substitution is:

- logging: `Error` with the failure context.
- audit: `LogConflictResolved` or similar — whatever explains the
  decision raioz took.
- notify: `up.notify_failed` (once, at end).
- output: `RaiozError` returned from the use case, rendered by
  `errors.FormatError` in `Execute`.

## Adding a new event

1. **Decide channels** using "who reads this, and when?" Most
   lifecycle events hit three or four; most debug events hit one.
2. **Audit lands first.** If it's auditable, add the typed helper
   in `internal/audit/audit.go` (or extend an existing one). The
   audit event type is your one chance to encode the right
   payload — adding fields later is fine, removing them is
   breaking.
3. **Log second.** Match the audit message; use structured fields
   for anything the audit also captures so a `--log-level=debug`
   trace lines up with the JSONL stream.
4. **Output third.** Through `output.Print*` with an `i18n.T()`
   key in both `en.json` and `es.json`.
5. **Notify last and rarely.** Only if the event is rare enough
   that pinging the user is welcome.
6. **Update the matrix in this doc** in the same commit.

## References

- Code: [`internal/logging/README.md`](../internal/logging/README.md)
  for slog internals;
  [`internal/audit/audit.go`](../internal/audit/audit.go) for event
  helpers; [`internal/notify/notify.go`](../internal/notify/notify.go)
  for the cross-platform shim.
- ADRs: [ADR-020](decisions/020-audit-rotation.md) (audit
  rotation), [ADR-021](decisions/021-dev-build-warning.md)
  (dev-build stderr warning).
- Issue: 043.
