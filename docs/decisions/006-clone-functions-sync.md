# ADR-006: Clone functions mirror config structs explicitly

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented)

## Context

`internal/app/upcase/workspace_project_conflict.go` resolves
conflicts when two projects in a workspace declare the same dep
or service. The resolver works on **copies** of `config.Service`
and `config.Infra` so it can mutate them without touching the
caller's view. The copies are produced by `cloneService` and
`cloneInfraEntry`, hand-written field-by-field.

Hand-written clones diverge from the source struct every time a
new field is added. We hit the same regression three times: add
a field to `config.Infra`, forget the clone, re-up after a
workspace conflict, watch the new field silently vanish. Each
incident produced a hot fix.

## Decision

When adding a field to `config.Service` or `config.Infra` that
affects orchestration, the corresponding clone function
**must** be updated in the same change. This is enforced by
convention and code review, not (yet) by tooling.

Detection aid: `grep -rn "config.Infra{" --include='*.go' .` and
`grep -rn "config.Service{" --include='*.go' .` after any
struct change. The clones are the call sites that build a fresh
struct literal — anywhere else, a missing field is a real bug,
not a clone-sync issue.

Future improvement (tracked in Wave 1 issue 023): when `Deps`
and friends move into `internal/domain/models/`, replace
hand-written clones with a `Clone()` method on each type, or
generate them. Either approach makes the regression impossible.
For now, vigilance + grep.

## Consequences

### Positive

- The clone functions are colocated with their use; reviewers
  can see them in the same diff.
- No reflection magic at runtime.

### Negative

- Pure discipline. Every new field is a regression opportunity.
- The lint can't easily catch this (the missing field is
  silence, not a syntactic violation).

### Neutral

- Once Wave 1 lands the domain-owned models migration, this
  decision may be superseded by a generated-clones ADR.

## Alternatives considered

- **`reflect.DeepCopy`** — works but loses field-level
  intentionality; if the resolver _wanted_ to drop a field
  during clone, reflect can't see it. Discarded for now.
- **`copystructure` library** — same concern + extra
  dependency.
- **Code generation (`go generate`)** — viable; deferred until
  the domain migration (issue 023) settles the source struct
  location.

## References

- Code: `internal/app/upcase/workspace_project_conflict.go`
  (`cloneService`, `cloneInfraEntry`)
- Related: Wave 1 issue 023 (domain models), ADR-009 (when
  written)
