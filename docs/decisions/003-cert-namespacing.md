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

## Update (2026-05-24): route FQDNs are minted as explicit SANs

`EnsureCerts(domain)` minted exactly `<domain>` + `*.<domain>` and the
validation only checked for those two. That breaks an apex hostname
under a single-label domain — e.g. `conorbi.localhost` (from
`proxy.domain: localhost` + `hostname: conorbi`). Its only possible
match is `*.localhost`, a wildcard browsers refuse to honor because the
parent is a single label (mkcert itself warns: "many browsers don't
support second-level wildcards like *.localhost"). `curl`/OpenSSL accept
it, so the cert looked CLI-valid but browser-insecure — a confusing
split diagnosis.

`EnsureCerts(ctx, domain, extraSANs...)` now also takes the exact route
FQDNs and (a) mints them as their own SANs alongside the domain +
wildcard, and (b) requires them in the reuse check, so a cert missing a
new route's FQDN is regenerated instead of silently served. The proxy
gathers the FQDNs via `Manager.routeSANs()` at `Start` (routes are
persisted before `Start`, so they're in scope). In workspace-shared
mode the per-domain cert folds in every sibling project sharing the
domain. SANs are de-duplicated (`certSANs`) to stay under mkcert's
practical SAN ceiling.

## References

- Code: `internal/proxy/certs.go` (`EnsureCerts`, `certSANs`,
  `certMatchesDomain`), `internal/proxy/proxy.go` (`routeSANs`)
- Related: ADR-004 (Caddyfile global options for mkcert mode)
