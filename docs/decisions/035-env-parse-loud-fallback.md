# ADR-035: Malformed duration env vars warn once + surface in `raioz doctor`

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

`internal/host/process_helpers.go::durationFromEnv` accepted any
value `time.ParseDuration` rejected and silently returned the
default. The user typo

```bash
export RAIOZ_LAUNCHER_TIMEOUT=120   # missing "s"
```

ran with the 60-second default and left no trace. Two symptoms:

- Confused users ("I set the env var, raioz ignores it").
- Bugs hidden during incident response — a value that looked
  like the override was actually the default.

Issue 062 proposed two complementary fixes:

- **A**: warn loudly on the first malformed read, once per
  (process, var) so a hot loop doesn't spam.
- **B**: enumerate the resolution state in `raioz doctor` so the
  user can `doctor` their way out of confusion without grepping
  the logs.

## Decision

Implement both.

### Inspection helper

New `host.InspectDurationEnv(name, def) EnvDurationStatus` returns
`{Raw, Resolved, Default, Malformed}` **without** any logging or
state mutation. Pure function — safe to call from doctor / tests
without polluting the warning dedup map.

### `durationFromEnv` warns once

`durationFromEnv` is now a thin wrapper:

```go
func durationFromEnv(name string, def time.Duration) time.Duration {
    s := InspectDurationEnv(name, def)
    if s.Malformed {
        if _, loaded := warnedEnvOnce.LoadOrStore(name, true); !loaded {
            logging.Warn("invalid duration env var; using default",
                "var", name, "value", s.Raw,
                "default", def.String(),
                "hint", "expected Go duration like 60s, 2m, 1h")
        }
    }
    return s.Resolved
}
```

`warnedEnvOnce` is a `sync.Map` for atomic check-and-set. A test
helper `ResetMalformedEnvWarningsForTest` exists so tests can
exercise the first-call path without depending on test ordering.

### Discovery via `KnownDurationEnvs`

`host.KnownDurationEnvs()` lists every duration env var raioz
reads. Currently:

- `RAIOZ_LAUNCHER_TIMEOUT`
- `RAIOZ_LAUNCHER_DRAIN_TIMEOUT`

**Contract for future contributors:** any new duration env var
must be appended to this list, otherwise it inherits the
silent-fallback bug for the doctor surface.

### `raioz doctor` reports the resolution state

`DoctorUseCase.checkEnvironment` produces a single check line:

| State                             | Status   | Message                                            |
|-----------------------------------|----------|----------------------------------------------------|
| All vars at defaults              | `ok`     | `no overrides (N duration var(s) at default)`      |
| Valid overrides set               | `ok`     | `N override(s): VAR=value, ...`                    |
| Any malformed                     | `error`  | `VAR="raw" (using default X) — expected Go duration like 60s, 2m, 1h` |

Status escalates to `error` when malformed: `Execute` returns a
non-zero exit code, so `raioz doctor` in CI surfaces the typo
loudly.

## Implementation status

Landed in this commit:

- `internal/host/process_helpers.go`: `EnvDurationStatus`,
  `InspectDurationEnv`, `KnownDurationEnvs`,
  `ResetMalformedEnvWarningsForTest`, warn-once-wrapped
  `durationFromEnv`.
- `internal/host/env_inspect_test.go`: covers unset / valid /
  every malformed shape (missing unit, alphabetic, semver,
  negative, whitespace), the dedup map, and the
  `KnownDurationEnvs` enumeration.
- `internal/app/doctor.go::checkEnvironment` + 3 unit tests
  (`no_overrides`, `valid_override`, `malformed_surfaces`).

## Consequences

### Positive

- Typos like `RAIOZ_LAUNCHER_TIMEOUT=60` produce a visible signal
  on every `raioz up`, and `raioz doctor` exits non-zero so it
  hooks naturally into CI.
- Adding a new duration env var is now obvious: append to
  `KnownDurationEnvs`. Without that, the doctor surface stays
  silent.

### Negative

- One extra global (`warnedEnvOnce` `sync.Map`). Trivial.
- `raioz doctor` exits non-zero when an env var is malformed.
  Intentional — that's the whole point. Users running doctor
  expecting "all clean" output will now see the typo as a hard
  failure.

### Neutral

- The log message stays in English (we log via `logging.Warn`,
  not `output.Print*`); structured fields keep it
  machine-readable.
- Existing `TestLauncherWaitTimeout` / `TestLauncherDrainTimeout`
  table tests continue to pass — the new helper preserves the
  exact resolution semantics, only the log side effect changed.

## Alternatives considered

- **Hard error at startup** (issue 062's option C). Rejected:
  breaks the "raioz still works even with typo'd env vars" property.
  Once-per-process warning + doctor surface gives the same signal
  without that cost.
- **Per-call log** instead of once-per-process. Rejected: a hot
  path that re-reads the env var inside `Stop` / `Restart` would
  spam the log. The user only needs to know *once* the value is
  bad.
- **Auto-correct by stripping unit-less digits** (assume the user
  meant seconds). Rejected: introduces magic. Loudly refusing is
  more honest than silently accepting one of two plausible
  interpretations.

## References

- Code: `internal/host/process_helpers.go::InspectDurationEnv` /
  `KnownDurationEnvs` / `durationFromEnv` (warn-once),
  `internal/app/doctor.go::checkEnvironment`.
- Tests: `internal/host/env_inspect_test.go`,
  `internal/app/doctor_test.go`.
- Issue: 062.
- Related: ADR-024 (env-driven hooks), ADR-031 (warning-level
  config gate).
