# Architecture Decision Records

Each ADR documents one architectural decision: the context that
forced a choice, what we chose, and what that choice costs and
unlocks.

## Format

Every ADR follows the same four sections:

- **Context** — what led to the decision. Constraints, prior
  bugs, external requirements.
- **Decision** — what we decided, stated as imperatively as the
  codebase allows ("raioz does X; new code must Y").
- **Consequences** — what we gained, what we lost, and what
  becomes harder.
- **Alternatives considered** — what we evaluated and discarded,
  with a one-line reason per option.

Use `_template.md` as a starting point.

## Immutability

ADRs are append-only. If a decision changes, write a new ADR that
**supersedes** the previous one (state it explicitly in both
files: the new one says "Supersedes ADR-NNN"; the old one gets a
`Status: Superseded by ADR-MMM` header).

Editing the body of a merged ADR is allowed only for typo fixes
or for adding a `Status:` line at the top.

## Index

| #   | Title                                                                | Status   |
| --- | -------------------------------------------------------------------- | -------- |
| 001 | [Container identity via labels, not names](001-container-identity-labels.md) | Accepted |
| 002 | [Shared deps are workspace-scoped, project label omitted](002-shared-deps-workspace-scoped.md) | Accepted |
| 003 | [Certificates namespaced per domain](003-cert-namespacing.md)         | Accepted |
| 004 | [`auto_https off` for mkcert in Caddyfile](004-caddy-auto-https-off.md) | Accepted |
| 005 | [Workspace-shared proxy lifecycle](005-workspace-shared-proxy.md)     | Accepted |
| 006 | [Clone functions mirror config structs explicitly](006-clone-functions-sync.md) | Accepted |
| 007 | [`IsNonHTTPImage` matches by bare name, not substring](007-image-classification-bare-name.md) | Accepted |
| 008 | [Sibling raioz projects as deps (modes A/B)](008-sibling-projects-as-deps.md) | Accepted |
| 009 | [`domain/models/` holds only leaf-dependency types](009-domain-models-scope.md) | Accepted |
| 010 | [Workspace-shared proxy lock](010-proxy-workspace-lock.md) | Accepted |

## When to write an ADR

Write one when the decision:

- Establishes a cross-cutting invariant that future code must
  respect.
- Closes a class of bug (the rationale matters once the fix is
  invisible in the diff).
- Picks between two non-trivial alternatives that a contributor
  could reasonably re-debate.
- Changes a previously documented decision (supersedes path).

Do **not** write one for:

- Local refactors that don't constrain future code.
- Library choices that are easily reversible.
- Style decisions covered by linters or CONTRIBUTING.md.
