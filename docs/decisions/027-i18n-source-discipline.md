# ADR-027: i18n source-level discipline via shrinking baseline

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

raioz advertises bilingual (en/es) support. `make check-i18n`
validates the catalogs (`internal/i18n/locales/{en,es}.json`) are
synchronized — every key in one file exists in the other.

The quality-auditor pass (issue 058) found **57 sites** where
`output.Print*` is called with a raw English literal that never
passed through `i18n.T()`. The catalogs were fine; the source
called them inconsistently. Result: a user running with
`RAIOZ_LANG=es` sees a mixed-language UI. The most painful case was
`internal/app/upcase/orchestration_proxy.go`'s interactive
proxy-failure prompt — entirely English, asking the user to pick
1/2/3.

Why the catalog check missed it: it only verifies key parity in the
JSON files. The actual `i18n.T()` call sites in code are
indirectly checked by tests that exercise specific keys, but
adding a new `output.PrintInfo("Hello")` doesn't trip any gate.

## Decision

Three coordinated changes:

### 1. Phase 1 fix — the interactive prompt

The orchestration_proxy.go prompt (15 user-visible strings) wraps
in `i18n.T(…)` immediately. Spanish translations added to the
catalog at the same time. This is the one interactive surface
where mixed-language output blocks the user from acting; it gets
fixed before the lint gate ratchet starts.

### 2. Lint gate — `scripts/check-i18n-source.sh`

Grep-based check that fails on raw English literals in
`output.Print*` calls. Pattern:

    output.Print<Anything>("<Uppercase letter>...

Catches `output.PrintInfo("Hello")` but ignores dynamic
concatenation (`output.PrintInfo("hello " + name)`) — those still
deserve i18n long-term, but the strict-literal pattern is the
load-bearing one for the prompt class of bugs.

### 3. Shrinking baseline — `scripts/i18n-source-baseline.txt`

A file with `<path>:<max>` entries that snapshots the current
violation count per file. The lint:

- Fails when a **new file** appears with violations (no entry in
  baseline).
- Fails when an **existing file's count grows** past its baseline.
- Succeeds when a file's count **drops below** its baseline, but
  emits a hint to update the baseline (so the ratchet stays
  monotonic in source control).
- Suggests removing entries for files whose count reached 0.

The baseline starts at 45 violations distributed across 18 files
after Phase 1. The ratchet enforces monotonic decrease — every PR
that touches one of these files must either preserve the count or
shrink it; bumping the baseline up is rejected.

Wired into `make check` (`check-i18n-source` target) and CI
(`.github/workflows/ci.yml` test job, alongside `check-i18n`).

## Implementation status

Landed in this commit:

- `internal/i18n/locales/{en,es}.json`: 15 new keys under
  `up.proxy.*` covering the orchestration_proxy.go prompt and
  related messages.
- `internal/app/upcase/orchestration_proxy.go`: every
  `output.Print*` call routes through `i18n.T()`.
- `scripts/check-i18n-source.sh` + `scripts/i18n-source-baseline.txt`:
  lint gate + initial baseline of 45 violations across 18 files.
- `Makefile`: `check-i18n-source` target, wired into `check`.
- `.github/workflows/ci.yml`: new CI step after `check-i18n`.

## Consequences

### Positive

- The proxy-failure interactive prompt now respects the user's
  language. The most impactful single fix from issue 058 ships.
- New files cannot regress the discipline. Every PR adding
  `output.Print*` calls must i18n them or fail CI.
- Existing violations are visible to contributors. The baseline
  file is the TODO list; shrinking it is the contribution path.
- The grep pattern is simple enough that anyone can read the
  script and adjust it.

### Negative

- 45 violations remain on the baseline. They are catalogued but
  not yet fixed — the discipline is `≤ baseline`, not `= 0`.
  Mitigated by the ratchet: every PR that touches an entry shrinks
  it; over time the file empties.
- The pattern only catches literal-starting English. A
  contributor writing `output.PrintInfo("hello " + dynamic)` would
  pass the lint but still bypass i18n. Acceptable trade-off: the
  alternative (parsing Go AST) is overkill for the volume of
  changes raioz sees, and dynamic strings are a much smaller
  class of bug than the static prompts that drove issue 058.
- The baseline file is one more thing to maintain. Contributors
  may forget to shrink it after migrating a file — the lint
  emits a hint but doesn't fail. Optional follow-up: a CI step
  that auto-updates the baseline when the count drops.

### Neutral

- ADR-027 follows the same playbook as ADR-001 / ADR-017
  (label-discipline lint, CLI thin-viz lint): a small grep script
  + a `make check-*` target. The pattern is becoming the project's
  default way to enforce a discipline that's hard to test.

## Alternatives considered

- **Full migration in one commit.** ~100 new i18n keys + 57
  call-site edits. Rejected: the orchestration_proxy fix alone
  needed verification (interactive prompt); doing everything at
  once would bury that signal. Phased migration with a ratchet is
  the standard pattern for cleaning up legacy debt.
- **Parse Go AST instead of grep.** More precise — would catch
  dynamic concatenation. Rejected because the false-positive rate
  on grep is already low (the baseline absorbs them) and the cost
  of AST tooling is high relative to the value.
- **Wait for a contributor to feel the pain.** Rejected: the
  pain is asymmetric. Spanish-speaking users are blocked from the
  proxy prompt; English speakers don't notice. The lint puts
  visibility into the development loop.
- **Hide the baseline file behind an env var.** Rejected: the
  baseline IS the public TODO list. Making it implicit hides what
  the team knows is broken.

## References

- Code: `scripts/check-i18n-source.sh`,
  `scripts/i18n-source-baseline.txt`,
  `internal/app/upcase/orchestration_proxy.go`.
- Catalogs: `internal/i18n/locales/{en,es}.json`
  (15 new `up.proxy.*` keys).
- CI: `.github/workflows/ci.yml` test job.
- Issue: 058.
- Pattern source: similar to ADR-001 (label-discipline lint) and
  ADR-017 (CLI thin-viz lint).
