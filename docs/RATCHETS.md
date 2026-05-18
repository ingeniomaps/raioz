# Shrinking-baseline ratchets

raioz uses **shrinking-baseline ratchets** to track and shrink
existing violations of an architectural rule without breaking CI on
the first migration PR. The pattern was introduced by ADR-027 and
ADR-029 and reused since.

Each ratchet has:

- a **baseline file** under `scripts/` listing current violators
- a **lint script** that fails when a new file enters the list, and
  also fails when an entry no longer fires (forcing the dev to prune)
- a `make check-*` target wired into the `check` aggregate and CI
- a **target-zero ADR or issue** that names the conditions under
  which the baseline becomes empty

A ratchet without a target is permanent drift in disguise. Every new
ratchet must publish one.

## Active ratchets

| Baseline | Lint | Make target | Target zero | Current size |
| --- | --- | --- | --- | --- |
| `scripts/i18n-source-baseline.txt` | `scripts/check-i18n-source.sh` | `check-i18n-source` | ADR-027 (every user-facing string through `i18n.T`) | per-file caps |
| `scripts/app-infra-imports-baseline.txt` | `scripts/lint-app-infra-imports.sh` | `check-app-infra-imports` | ADR-029 (all app code routes through `internal/domain/interfaces/`) — explicit per-cluster drain plan in ADR-029 § "Drain plan" | 22 files |
| `scripts/dual-flow-baseline.txt` | `scripts/lint-dual-flow.sh` | `check-dual-flow` | ADR-038 (JSON loader removed in v0.8; ADR-039 `SchemaVersion` field deleted in the same release) — cleanup marker lives at top of `internal/app/flow.go` | 5 files |
| `scripts/errorlint-baseline.txt` | `scripts/lint-errorlint.sh` | `check-errorlint` | every `fmt.Errorf` chained with `%w` / every error compared via `errors.Is` / every type-assertion through `errors.As` | 25 sites |

## When to add a new ratchet

- A reviewer or auditor surfaces a class of violation that exists
  across the codebase in non-trivial numbers (≥ 5 sites).
- An ADR establishes an invariant that new code must respect, but
  retro-fitting existing code in one PR is infeasible.
- The fix is **mechanically detectable** — a regex, a `grep`, a
  `golangci-lint` linter, a small AST walk. If detection requires
  human judgement, this is not a ratchet, it is a review checklist.

## When to retire a ratchet

When the baseline file is empty (or contains only the header
comment), promote the rule to a hard error: delete the baseline,
delete the script's allowlist branch, keep the lint as a regular
`make check`. Document the promotion in the target-zero ADR.
