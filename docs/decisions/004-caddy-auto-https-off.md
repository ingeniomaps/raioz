# ADR-004: `auto_https off` in Caddyfile when TLS mode is mkcert

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented)

## Context

Caddy's `auto_https` directive controls two things: HTTP→HTTPS
redirects and the ACME pipeline for cert acquisition. Default is
**on**.

When raioz uses mkcert (`tls: mkcert`), there is no public DNS
for the custom domain (`acme.localhost`, `acme.dev`, etc.). ACME
attempts the HTTP-01 challenge, fails, and Caddy hangs retrying.
Logs fill, port 80 may be held indefinitely.

The first fix attempt was `disable_redirects` in the site block.
That silences the HTTP→HTTPS redirect but leaves ACME running —
the hang remained.

## Decision

When raioz generates a Caddyfile with `tls: mkcert`, it writes
`auto_https off` at the global options block. This disables
**both** the redirect and the ACME pipeline.

Do not revert to `disable_redirects` alone; that fixes only half
the problem. Do not enable `auto_https` for any TLS mode that
isn't backed by a publicly resolvable domain.

## Consequences

### Positive

- mkcert flows start instantly with no ACME hang.
- Port 80 stays free for raioz to bind explicitly (or for other
  workspaces under `proxy.publish: false`).
- Log noise gone.

### Negative

- HTTP→HTTPS redirect is also off — raioz must explicitly bind
  port 80 and configure the redirect manually if needed. Today
  we only bind 443 in mkcert mode.
- Future TLS modes (e.g. ACME against a real domain) must NOT
  inherit this directive; the Caddyfile generator branches on
  TLS mode.

## Alternatives considered

- **`disable_redirects` only** — doesn't stop ACME; rejected.
- **`local_certs` directive** — Caddy-internal CA, but loses
  mkcert trust integration with system stores; rejected.
- **Run two Caddy instances** (one for mkcert, one for ACME) —
  complexity > value.

## References

- Code: `internal/proxy/caddyfile.go`
- Related: ADR-003 (cert namespacing — same mkcert path)
