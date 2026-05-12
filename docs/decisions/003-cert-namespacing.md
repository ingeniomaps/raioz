# ADR-003: Certificates are namespaced per domain

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented)

## Context

raioz uses mkcert to generate local-trust certificates for the
proxy domain (`*.localhost` by default; user-overridable via
`proxy.domain`). Initially, certs lived in a flat
`~/.raioz/certs/` and were reused if present.

The bug: a user with a workspace on `acme.localhost` and another
on `hypixo.dev` ended up serving the `acme.localhost` SAN cert
for `hypixo.dev` requests because the first cert was reused
without checking what it covered. Browsers screamed; raioz
silently passed it along.

## Decision

Certificates live in a per-domain directory:
`~/.raioz/certs/<domain>/`. `EnsureCerts(domain)` validates
**before reusing** that the on-disk cert's SAN list includes
both `<domain>` and `*.<domain>`. If validation fails (missing
SAN, expired, wrong CN), the cert is regenerated.

The validation is mandatory; reuse without validation is
banned. The function is the only entry point for cert handling
in `internal/proxy/`.

## Consequences

### Positive

- Two workspaces with different proxy domains never collide.
- Regenerating a cert for one domain doesn't touch others.
- mkcert installation status is checked per-domain (no false
  cache hits).

### Negative

- Disk space: one cert+key pair per domain. Negligible (~10KB
  each).
- First-up on a new domain pays the mkcert generation cost
  (~1s).

## Alternatives considered

- **Single shared cert with SAN list for every domain raioz has
  seen** — requires maintaining a growing SAN list and
  regenerating frequently; rejected as complexity > savings.
- **No validation, just regenerate every time** — slow, and
  `mkcert -install` prompts the user if missing; bad UX.
- **Trust-on-first-use without SAN check** — what we had; the
  bug above.

## References

- Code: `internal/proxy/certs.go` (`EnsureCerts`)
- Related: ADR-004 (Caddyfile global options for mkcert mode)
