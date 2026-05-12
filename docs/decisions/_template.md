# ADR-NNN: Short imperative title

- **Status:** Proposed | Accepted | Superseded by ADR-MMM
- **Date:** YYYY-MM-DD
- **Supersedes:** (optional) ADR-MMM

## Context

What forced this decision? Constraints, prior bugs, external
requirements, conflicting tradeoffs. State the problem in
enough detail that a reader who doesn't know the codebase
understands why we couldn't just pick the obvious default.

## Decision

What we chose. State it imperatively: "raioz does X", "new code
must Y". Be precise about scope — say where the rule applies and
where it doesn't.

If the decision involves invariants enforced in code (lint, test,
panic on violation), name the enforcement mechanism explicitly.

## Consequences

### Positive

- What this unlocks.
- What classes of bug it prevents.

### Negative

- What gets harder.
- What workarounds remain in the codebase.

### Neutral

- Behaviors that change without being clearly better or worse.

## Alternatives considered

- **Alternative A** — one-line reason discarded.
- **Alternative B** — one-line reason discarded.

## References

- Code: `path/to/file.go:line`
- Related: ADR-MMM (if relevant)
- Issue/PR: link if there is one
