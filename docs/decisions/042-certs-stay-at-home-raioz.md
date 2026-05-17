# ADR-042: TLS certs stay at `~/.raioz/certs/`

- **Status:** Accepted
- **Date:** 2026-05-16
- **Refines:** ADR-003 (cert namespacing), ADR-022 (unified state paths)

## Context

ADR-003 established `~/.raioz/certs/<domain>/` as the cert storage
layout. ADR-022 later unified all *runtime state* under
`naming.RaiozStateDir()` (`RAIOZ_HOME` → `$XDG_STATE_HOME/raioz` →
`~/.local/state/raioz`). `MigrateLegacyStateDirs` lifts every legacy
`~/.raioz/*` directory into the unified root **except** `certs/`,
and `docs/STATE.md` flagged the open question:

> "No decision required for v0.5.x; flag for v0.6 review"

We are now at v0.8.3. The decision has been deferred through v0.6,
v0.7, and v0.8. Every contributor reading STATE.md asks why certs
are exempt, and the exception silently weakens ADR-022's
"single source of truth" invariant.

## Decision

**TLS certs stay at `~/.raioz/certs/<domain>/`.** The exception in
`MigrateLegacyStateDirs` is intentional, not vestigial. The "open
question" in `docs/STATE.md` is hereby closed in favour of the
status quo.

Reasons:

1. **mkcert CAROOT proximity.** mkcert stores its root CA at
   `~/.local/share/mkcert/` (Linux/macOS) or
   `%LOCALAPPDATA%\mkcert\` (Windows) by default, **outside**
   `RaiozStateDir()`. Certs derived from a CA are tied to that
   CA's lifecycle — if the CAROOT is regenerated (corruption,
   user `mkcert -uninstall`), every cert becomes invalid. Co-
   locating certs with their CA root (both in `~/`) keeps the
   audit story local: "where is my CA?  where are my certs?
   both in $HOME".

2. **raioz does not own mkcert's CAROOT.** Moving certs into
   `RaiozStateDir()` would either force raioz to also manage the
   CAROOT (out of scope — mkcert is an external tool that raioz
   complements, per project principle) or leave the CA and its
   derived certs split across two roots, which is the worst of
   both worlds.

3. **Migration cost outweighs benefit.** Users with existing
   `~/.raioz/certs/` would need either: (a) a migration that
   moves certs but breaks any external tool referencing the old
   path, or (b) a deprecation window with both paths supported.
   Both options add complexity for no architectural win — the
   cert directory is read-mostly, namespaced by domain, and
   doesn't pollute the user's `~/`.

4. **ADR-022's invariant scope.** Re-reading ADR-022, the
   "single source of truth" applies to **raioz-owned runtime
   state**: locks, workspace state, audit log, port allocation
   data. TLS certs are tool-managed artifacts (mkcert produces
   them; raioz reads/serves them via Caddy). They are not
   raioz's state to own.

## Consequences

### Positive

- Closes the dangling open question in STATE.md without
  introducing churn.
- mkcert + raioz cert audit remains co-located under `~/`.
- Existing user setups continue to work without migration.

### Negative

- The exception in `MigrateLegacyStateDirs` requires an inline
  comment justifying why certs aren't lifted — added as part of
  closing this ADR.
- ADR-022's "single source of truth" wording must be qualified
  to "raioz-owned runtime state" so the cert exception isn't
  interpreted as drift.

### Neutral

- A future move (e.g. if mkcert is replaced with a different
  CA tool) would re-open this ADR. Documented as a re-open
  trigger, not a known plan.

## Alternatives considered

- **Option B (migrate to `RaiozStateDir()/certs/`).** Considered
  in issue 022 as the architecturally cleaner option. Rejected
  because it forces raioz into the CAROOT-management business
  (or accepts a split CA/cert layout, which is strictly worse
  than co-located). Migration cost (deprecation window + dual
  path support) is non-trivial for a read-mostly directory.
- **Option C (relocate mkcert CAROOT into RaiozStateDir too).**
  Out of scope — mkcert is an external tool. raioz does not own
  its config root.

## References

- Code: `internal/proxy/certs.go` (cert path resolution),
  `internal/naming/migrate.go::MigrateLegacyStateDirs` (the
  exception)
- Related: ADR-003 (cert namespacing — refined), ADR-022
  (unified state paths — scope qualified)
- Docs: `docs/STATE.md` — open question closed in favour of
  this ADR
