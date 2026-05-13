# ADR-020: Audit log rotates at 10 MiB with a single backup

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`internal/audit/audit.go` appends every user-facing event (dependency
added, dev promoted, drift detected, …) to `audit.log` as JSONL.
Before this change, the file grew unbounded — there was no rotation,
no cap, no inspection or cleanup command. On developer machines running
raioz daily, the file could realistically reach hundreds of MiB
within a year.

The issue offered two options:

- **A** — automatic rotation with a soft cap.
- **B** — a `raioz audit show / clear` CLI surface that puts the
  decision on the user.

Option A is the safer default: the failure mode of option B ("disk
fills up because nobody ran `raioz audit clear`") is louder than
option A's ("history beyond 30 days is dropped"). Users who want
inspection get a `raioz audit show` follow-up if/when demand
materializes.

## Decision

`audit.Log` calls `rotateIfOverCap(path, maxAuditSize)` before every
append. The function:

- Stats the file once.
- If the file is at or under 10 MiB (`maxAuditSize`), returns
  immediately.
- Otherwise `os.Rename(path, path+".1")` — on Linux this overwrites
  any existing `.1`, which is exactly the desired behavior (keep
  the most recent rotation, discard the previous one).
- Errors are swallowed: a rotation failure is logged warn but the
  event still appends. Losing one rotation is better than losing an
  audit event.

The constant `maxAuditSize = 10 * 1024 * 1024` lives at the top of
`audit.go`. The threshold was picked at "one month of heavy raioz
use ≈ 1 MiB"; 10 MiB gives ~9 months of headroom on the rare
heavy-use month and stays small enough that the stat-on-every-append
cost stays in the noise.

Tests in `audit_test.go`:

- `TestRotation_NoOpUnderCap` — no .1 created when file fits.
- `TestRotation_TriggersWhenOverCap` — rotation moves contents to .1.
- `TestLogAppendsAfterRotation` — `Log` writes fresh after a rotation.

## What's deliberately not in this ADR

- `audit.maxAuditSize` is a constant, not a config field. Making it
  configurable is a real ask the moment someone needs a different
  policy (CI runners with no human attention, regulated environments
  that want 90 days). Until then, keep it simple.
- No `raioz audit show / clear` CLI. The issue lists it as the
  Option B alternative; revisit when demand surfaces. Users hit the
  file directly today (`tail`, `jq`).
- No locking. Concurrent `raioz` invocations could theoretically
  rotate simultaneously, but the worst case is "one rename loses to
  another" — content remains intact in either ordering. Add file
  locking if the workload ever justifies it.

## Implementation status

Landed in this commit:

- `audit.maxAuditSize` constant.
- `rotateIfOverCap(path, capBytes)` helper.
- `Log` calls it before every append.
- Three rotation tests.
- `docs/ARCHITECTURE.md` gains the "Audit log rotation" section.

## Consequences

### Positive

- Disk usage is bounded at ~20 MiB worst case (`audit.log` ≤ 10 MiB
  + one `audit.log.1` ≤ 10 MiB).
- Behavior is transparent — users running `ls -la $RAIOZ_HOME` see
  the rotated file.
- The stat-on-every-append is cheap (one syscall, single field).

### Negative

- History beyond ~one month of heavy use is gone. Users who need
  long-term audit trails would have to script their own append-only
  copy. Acceptable trade-off for a dev tool; revisit if compliance
  needs surface.
- The rotation is single-backup. A "rotate to .1, .1 to .2" cascade
  would preserve more history at higher disk cost. Skipped because
  it complicates the rotation logic and the use case isn't there.

### Neutral

- Old binaries reading the rotated file post-upgrade will see the
  full history split across `audit.log` and `audit.log.1`. Trivial
  to combine (`cat audit.log.1 audit.log | jq`). Documented in the
  ARCHITECTURE.md section.

## Alternatives considered

- **Daily/size dual rotation (logrotate-style).** The issue
  suggested keeping just one backup; growing the rotation policy
  trades disk for history. No demand yet, keep simple.
- **In-memory ring buffer.** Faster, no disk, but loses the audit
  trail across binary restarts. The whole point of the file is
  durability.
- **Skip rotation, add CLI.** Option B from the issue. Rejected on
  failure-mode comparison (see Context).

## References

- Code: `internal/audit/audit.go`, `internal/audit/audit_test.go`.
- Docs: `docs/ARCHITECTURE.md` ("Audit log rotation" section).
- Issue: 040.
