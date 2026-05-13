# ADR-017: CLI files route through `internal/app/` (with a small exempt list)

- **Status:** Accepted
- **Date:** 2026-05-13

## Context

After issues 034 / 035 / 036 routed snapshot / tunnel / proxy through
use cases, every side-effecting `raioz` command goes through the
`cli/ → app/<usecase>/ → domain/interfaces/` chain. Without a rule
codifying that pattern, a new contributor (or future-me) could add a
fresh command directly against `internal/<package>/` and have nothing
flag the regression. The next time `Dependencies` grows, that command
silently fails to pick up the new port, or hardcodes a behavior that
should have been mockable.

We also have a handful of CLI files that legitimately don't need a
use case:

- Scaffolding files (`root.go`, `config_path.go`,
  `zzz_i18n_descriptions.go`) — no command of their own.
- Subcommand parents (`migrate.go`, `yaml.go`) — they just register
  children.
- Pure-visualization commands (`version.go`, `lang.go`,
  `yaml_lint.go`) — read state, format, print, exit.
- Acknowledged tech debt (`migrate_yaml.go`) — the legacy
  `.raioz.json → raioz.yaml` conversion has real logic but no use
  case yet; flagging it as exempt now keeps the lint useful while
  the migration is tracked.

The rule has to allow that list without becoming a loophole.

## Decision

1. **Default rule.** Every file under `internal/cli/*.go` (excluding
   `_test.go`) must import `raioz/internal/app`. The import is the
   structural marker that the command routes through a use case.

2. **Explicit exempt list.** A small, named set of files bypasses
   the rule:

   - `root.go`, `config_path.go`, `zzz_i18n_descriptions.go` —
     scaffolding, no command logic.
   - `version.go`, `lang.go` — pure-viz / user-pref persistence.
   - `migrate.go`, `yaml.go` — subcommand parents.
   - `migrate_yaml.go` — known tech debt, slated for use-case wrap
     when the feature is touched next.
   - `yaml_lint.go` — `raioz yaml lint`; analyzes YAML and prints,
     no side effects. ADR-014/015/016's pattern doesn't pay back here.

   The list lives in three places that must stay in sync:

   - `scripts/lint-cli-layering.sh` (the actual gate).
   - `docs/ARCHITECTURE.md` "CLI thin-viz exception" (the prose).
   - This ADR (the rationale).

3. **Gate.** `make check-cli-layering` runs
   `scripts/lint-cli-layering.sh`, which grep-checks the import
   against the exempt regex. The target is part of `make check`.
   New CLI files appear in CI immediately.

4. **Procedure for new exemptions.** A PR adding a file to EXEMPT
   must edit all three sources (lint script, ARCHITECTURE.md, this
   ADR's exempt table). Reviewers should refuse silent expansions —
   if only the lint changes, the rule is being eroded without
   visibility.

## Three rules a file must satisfy to be exempt

When evaluating whether a new file qualifies for the exempt list:

1. **No side effects** beyond stdout/stderr. Trivial config writes
   (e.g., persisting `raioz lang`) are tolerated because the file
   write is the *only* effect and it's a user-controlled
   single-call.
2. **Reads state via existing ports or pure helpers**; no new port
   gets introduced specifically for this command.
3. **Behavior is a pure function** of CLI inputs plus
   already-port-reachable state.

If any of these falters, the command needs a use case.

## Implementation status

Landed in this commit:

- `scripts/lint-cli-layering.sh` implements the gate.
- `make check-cli-layering` wires it into `make check`.
- `docs/ARCHITECTURE.md` gains the "CLI thin-viz exception"
  section.
- This ADR.

Negative test verified manually:
`echo 'package cli; import "raioz/internal/snapshot"; var _ = snapshot.NewManager' > internal/cli/dummy.go`
makes the lint fail with the actionable message; removing the file
restores green.

## Consequences

### Positive

- New commands default to the right pattern. The cost of doing it
  wrong is a CI failure with a remediation message.
- The exempt list is short, justified, and reviewable. Expanding
  it requires touching three coordinated places, which is enough
  friction to force a discussion.
- Documentation, lint, and ADR all reference each other, so a
  future contributor reading any of the three finds the others.

### Negative

- `migrate_yaml.go` is on the list as tech debt rather than a
  genuine viz command. The exemption masks the underlying issue.
  Mitigated by the inline "tech debt" tag in the ARCHITECTURE.md
  table; the next person who touches that file knows to also lift
  it to a use case.
- The lint is grep-based and doesn't actually check the file
  *uses* `internal/app/` — only that it imports it. A determined
  bypasser could import the package and ignore it. Acceptable: the
  rule is for honest mistakes, not adversarial behavior.

### Neutral

- The current exempt list happens to overlap heavily with files
  that don't carry a Cobra command at all (root, config_path,
  zzz_*). That's fine; including them in the lint regex is
  cheaper than special-casing "files that declare a `*cobra.Command`".

## Alternatives considered

- **No lint, only docs.** The status quo. Relies on reviewers to
  catch bypasses. Already failed once (we wouldn't be writing this
  ADR if it hadn't).
- **AST-based lint instead of grep.** Could verify a Cobra
  `RunE`/`Run` actually constructs a use case. Higher fidelity,
  significant complexity. The grep gate catches the regression
  pattern we've actually seen; revisit if a bypass survives it.
- **Force every file into the layer (no exempt list).** Would mean
  wrapping `version.go`, `lang.go`, and subcommand parents in
  use-case wrappers that delegate immediately. Pure ceremony, no
  testability win.

## References

- Code: `scripts/lint-cli-layering.sh`, `Makefile`
  (`check-cli-layering` target).
- Docs: `docs/ARCHITECTURE.md` ("CLI thin-viz exception" section).
- Related: ADR-014 (snapshot use-case wiring — the pattern this
  ADR enforces), ADR-015, ADR-016.
- Issue: 037.
