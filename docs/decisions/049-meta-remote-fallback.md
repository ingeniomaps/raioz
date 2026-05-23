# ADR-049: Meta remote-mode fallback

- **Status:** Accepted
- **Date:** 2026-05-22

## Context

[ADR-048] added auto-clone for meta sub-projects so a new dev gets the
whole stack with one `raioz up`. Auto-clone closes the "fresh checkout"
half of issue 020 but leaves the "partial access" half open: when a
partner or contractor can't authenticate against a private repo (e.g.
`oim-api` is restricted to internal devs), the clone fails and the whole
meta up aborts. The dev wanted to run `web` + `bff` against a
publicly-reachable deploy of `api` — that's a legitimate workflow, but
one with a strictly higher trust profile than auto-clone: raioz must
forward HTTP traffic to a host the user doesn't control, and trust that
host with whatever the client sends.

[ADR-005] already specifies the workspace-shared Caddy reading
`${WorkspaceProxyDir()}/<ws>/routes/<project>.json` per project. So the
mechanism to inject a remote route exists — what's missing is a meta
sub-project mode that produces such a file without actually spawning the
project locally.

This ADR also has to draw a line: remote mode is the **opposite** of
auto-clone in trust posture. Auto-clone takes a URL the parent meta
config explicitly declared and runs a sub-process raioz built. Remote
mode takes a URL the parent meta config explicitly declared and
forwards client requests to it — and from then on, every response is
controlled by that remote host. Treating the two as a single feature
would conflate review categories that deserve separate scrutiny.

## Decision

Meta sub-projects gain a fourth mode (`remote`) and a new optional
field that activates it:

```yaml
projects:
  - path: ./api
    git: git@github.com:cubiko/oim-api.git
    branch: develop
    auth: gh
    remote: https://api.staging.acme.dev    # fallback when clone fails
    remoteHostname: api                      # optional override; default = filepath.Base(path)
```

Six rules govern the mode:

1. **Cascade extends [ADR-048] with one extra step.** The full
   load+bootstrap cascade is now:

   - `PathExists && !ForcedRemote`          → `MetaModeLocal`
   - `PathExists && ForcedRemote`           → `MetaModeRemote`
   - `!PathExists && Git`                   → bootstrap clone; success
     → Local; failure + Remote → Remote; failure + !Remote + Optional
     → Skip; failure + !Remote + !Optional → abort.
   - `!PathExists && Remote && !Git`        → `MetaModeRemote` (no
     clone attempt).
   - `!PathExists && Optional && !Git && !Remote` → `MetaModeSkip`
     (unchanged).
   - `!PathExists && !Optional && !Git && !Remote` → `MetaModeLocal`
     (deferred error at spawn — unchanged).

2. **`ForcedRemote` is per-project, opt-in.** Set by the
   `--force-remote=a,b,c` CLI flag (project names, matched against
   `MetaProject.Name`). Lets a dev with the repo cloned run against
   staging for a single sub-project without editing the meta yaml.
   Specifying an unknown name is rejected (better to typo-error than
   to silently fall through).

3. **Routes are written via a public proxy helper.** The bootstrap
   phase calls `proxy.WritePersistedRemoteProject(workspace,
   projectName, hostname, remoteURL)`. The helper materializes the
   same `persistedProject` JSON shape [ADR-005] defines for local
   projects, with a single `ProxyRoute` whose `Target` is the bare
   remote URL (Caddy's `reverse_proxy https://api.staging.acme.dev`
   accepts a full URL as upstream). `Port` is left at zero so the
   existing `target:port` append heuristic in `writeRouteBlock` stays
   out of the way.

4. **Workspace Caddy must be brought up by a local sub.** A pure-remote
   meta (every sub `Remote`) writes the route files but no Caddy
   reads them — the routes are dead. This is **intentional**: meta
   exists to coordinate sub-projects, and "no sub-project" is a
   degenerate case. The ADR pushes that constraint to the user via
   a warning (`meta.remote_no_local`) when bootstrap detects 0
   would-be-local subs after the cascade.

5. **No new TLS surface for remotes.** Caddy validates the upstream
   TLS certificate by default; users wanting to point at a
   self-signed staging deploy must either fix their cert or run
   raioz with `--insecure-remote` (NOT implemented in this ADR —
   tracked as a follow-up if concrete demand surfaces). Out of the
   box, remote mode refuses to bridge to an upstream raioz can't
   verify. This is the trust-posture floor the ADR commits to.

6. **No request-side auth (`remoteAuth:`) in v1.** Header injection,
   PAT-from-env, basic-auth passthrough — all deferred. The issue
   itself flagged these as follow-up; this ADR ratifies that scope.

CLI surface:

```bash
raioz up --force-remote=api          # force remote even when path exists
raioz up --force-remote=api,web      # comma list
```

## Consequences

### Positive

- The partial-access workflow becomes declarative: same yaml works for
  the dev with full repo access AND the partner with web/bff only.
- Trust posture is explicit: remote mode is its own ADR with its own
  "won't do" list. Future work (insecure-remote, remoteAuth, multi-
  hostname) can debate against this baseline without sliding into
  auto-clone scope.
- Reuses the existing route persistence mechanism — no parallel
  surface to maintain, no new Caddyfile generator.

### Negative

- **Remote mode forwards client traffic to a host the user doesn't
  control.** Any logging, header inspection, body capture, response
  injection on that host applies. The dev's browser will see TLS
  certs and cookies set by `api.staging.acme.dev`, not by raioz.
  The ADR mitigates by refusing self-signed upstreams (rule 5) but
  the residual trust hop is real and intentional.
- Pure-remote meta workspaces don't get a Caddy automatically.
  Documented; warned at bootstrap; not auto-started for v1.
- Hostname inference defaults to `filepath.Base(path)` — works for
  flat layouts (`./api`, `./bff`) but a sub-project at `./services/api`
  resolves to hostname `api`, same as a top-level `./api`. Users with
  collisions must set `remoteHostname:` explicitly.

### Neutral

- The single-hostname-per-remote-project simplification matches the
  most common case (a partner needs `api.localhost` to map to staging).
  Multi-service remote projects (one yaml → 3 hostnames) need each
  hostname declared; the YAML schema supports a list under
  `remoteHostnames:` as a follow-up, not gated on this ADR.

## Alternatives considered

- **Remote mode without TLS verification by default.** Rejected:
  silently downgrading to insecure-by-default would mask MITM in
  staging environments the user thinks are over TLS. Explicit opt-in
  via a future flag keeps the surface auditable.
- **Server-Sent transparent proxy (`l4` plugin).** Rejected: requires
  a Caddy build that includes the l4 module; raioz ships the
  standard Caddy. The HTTP-level `reverse_proxy` covers the common
  case and stays portable.
- **`remoteAuth:` in this ADR.** Rejected: header-injection, OAuth
  bearer, basic-auth all have different threat models (where does
  the secret come from, where can it leak). Folding them into a
  fallback ADR would mix concerns; better to ship the proxy half
  first and grow `remoteAuth:` against concrete demand.
- **Synthetic Caddy spawn for pure-remote workspaces.** Rejected for
  v1: launching a Caddy from the meta runner without a sub-project
  context bypasses the workspace lock + route ownership contract.
  The warning + manual `raioz proxy` is a documented escape hatch.

## Won't do (this ADR)

- **`--insecure-remote` / TLS skip-verify.** Tracked as follow-up.
  Refusing this in v1 keeps the trust floor tight.
- **`remoteAuth:`** — see Alternatives.
- **Auto-start of a Caddy for pure-remote workspaces.** Documented
  warning instead.
- **Multi-hostname per remote project.** Schema permits
  `remoteHostname:` only (singular). `remoteHostnames:` list is a
  YAML evolution candidate, not blocked here.
- **Forward auth headers received from the client.** Out of scope;
  treat the remote upstream as a trust boundary.

## References

- Code: `internal/app/meta.go`, `internal/app/meta_bootstrap.go`,
  `internal/app/meta_remote.go` (new), `internal/proxy/routes_persist.go`,
  `internal/config/yaml_meta.go`, `internal/config/yaml_types.go`
- Related: [ADR-005] (workspace-shared proxy + routes persistence),
  [ADR-037] (router project — trust asymmetry, applies to remote too),
  [ADR-048] (auto-clone bootstrap — sister ADR)
- Issue: `docs/issues/020-meta-auto-clone-and-remote-fallback.md`

[ADR-005]: 005-workspace-shared-proxy.md
[ADR-037]: 037-replaceable-edge-router.md
[ADR-048]: 048-meta-auto-clone-bootstrap.md
