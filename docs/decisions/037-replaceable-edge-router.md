# ADR-037: Workspace can replace the internal Caddy with a sibling project

- **Status:** Proposed — 2026-05-14
- **Drives:** issue 066

## Context

Raioz auto-spawns its bundled Caddy whenever any sub-project declares
`proxy: <domain>`. That choice is fine for the small / hobbyist case
but breaks teams whose **production edge is not Caddy** — nginx,
HAProxy, Traefik, Envoy. Two paths exist today, both costly:

- **Accept Caddy in dev.** Routing diverges from production: cookie
  handling, header rewrites, path normalization, TLS termination
  behave differently. Bugs surface in staging or later.
- **Run the real edge in parallel.** Two containers compete for host
  ports and IPs; `/etc/hosts` points at one while the other lingers
  as a ghost container. Nobody is sure which one served the last
  request.

The README states the project's central promise: *"the developer uses
their preferred tools; raioz just connects, starts, and stops
everything."* The hard-wired Caddy is the only place where raioz
**imposes** a tool. ADR-037 corrects that imposition.

Real-world case: hypixo (raioz v0.6.0) — 5 service projects + a
`gateway/` project (HAProxy fronting nginx with DNS templates).
Production uses gateway end-to-end; dev silently uses Caddy because
each sub-yaml has `proxy: hypixo.dev`. When the architect activates
`--meta-profile gateway`, the gateway containers come up but receive
no traffic — `/etc/hosts` still points at Caddy.

## Decision

Introduce a workspace-level `router` field whose value is another
raioz project. When present, raioz:

1. **Skips its internal Caddy.** No proxy container, no per-service
   Caddyfile generation, no auto-allocated subnet IP.
2. **Treats the router project as a hard workspace dependency.** It
   is always brought up first (before any consumer) and torn down
   last. It cannot be excluded via `--meta-profile` or any other
   filter — the rest of the workspace depends on it being ready.
3. **Polls the router project's declared `health:` before consumers
   start.** If `health:` is absent, raioz brings the router up and
   warns; consumers start in parallel.
4. **Does not expose service-discovery to the router in V1.** The
   router project owns its own routing tables (templates,
   configuration files, etc.). V2 may add label-based discovery
   (Traefik-style) once a second concrete case surfaces.

### Schema

```yaml
version: "1"
workspace: hypixo
kind: meta            # or a plain project — both shapes accept `router:`

router:
  project: ./gateway  # path to the sibling raioz project

projects:
  - path: ./api
  - path: ./web
  - path: ./gateway   # the router project also appears here under projects:
```

Constraints:

- `router.project` MUST point at a raioz project (i.e. a directory
  containing `raioz.yaml`). The path is resolved relative to the
  current file.
- The same path MAY also appear in `projects:` — the umbrella tracks
  it for lifecycle just like any other sub-project; `router:` only
  upgrades it from "another sub-project" to "first up, last down,
  cannot be excluded".

### CLI

- `raioz up` — router project first, then consumers; consumers wait
  for router's `health:` if declared.
- `raioz up --router-off` — bypass `router:` for this invocation,
  re-enable the internal Caddy. Useful for debugging routing issues
  in isolation.
- `raioz down` — consumers first (reverse of up), router last.

### Open questions answered

The five open questions in issue 066 resolve as follows for V1:

| Q | Decision |
|---|---|
| Service-discovery contract | **None in V1** — router owns its templates. V2 may add labels. |
| Router healthcheck | **Via existing `health:` field.** Polled with same semantics as service health. |
| Coexistence (Caddy + router) | **No.** One or the other. |
| `proxy:` in sub-yamls when router declared | **Silently ignored.** Sub-yaml declares *what should be routed*, not *how*. V2 may use it as input to label-based discovery. |
| `--router-off` lifecycle | **Per-invocation flag.** No persistent workspace preference. |

## Consequences

### Positive

- Closes a real architectural gap: today raioz has a hardcoded
  routing choice that limits adoption by teams with established
  edge stacks.
- Reinforces the central project promise (use your tooling, not
  raioz's).
- Builds on primitives that already exist:
  - ADR-008 sibling-projects-as-deps — the router is the "special
    sibling that cannot be excluded".
  - ADR-024 pre-up hook + lifecycle — same ordering pattern.
  - Issue 048 meta-profiles — composes cleanly (router is always-on,
    profiles only filter the consumers).
- Migration is opt-in. Default behavior unchanged; existing users
  see zero impact.

### Negative

- New schema surface to document and version.
- Adds a hard ordering constraint to `raioz up` — the router must be
  ready before consumers. Failures of the router project abort the
  whole `up`, with no Caddy fallback (unless `--router-off`).
- Teams that adopt this and then remove `router:` rediscover Caddy.
  Documented as "expected"; the diagnostic in `raioz up` says which
  proxy is being used.

### Neutral

- V1 ships without service-discovery from raioz to the router. That
  is intentional — it cuts the open-design surface significantly.
  Teams with N≥20 services that need dynamic discovery have a workaround
  (the router project can read raioz labels itself, today). V2 may
  formalize that contract once a second case appears.
- `proxy:` in sub-yamls becomes informational when `router:` is set.
  Considered renaming or deprecating, but the field still carries
  useful info (the hostname the service expects to be reachable at)
  that V2 may consume. Keep, document the override.

## Alternatives considered

- **Sibling-dep + manual ordering.** ADR-008 already lets a project
  declare another raioz project as a dependency. The user could
  declare gateway as a dep of every consumer. Rejected because:
  - It does not disable Caddy. Caddy still spawns alongside.
  - The dependency must be declared N times — doesn't scale to
    many consumers.
  - `proxy:` in each sub-yaml continues to trigger Caddyfile
    generation and IP allocation.
- **Plugin model for edge routers.** Define an interface raioz
  implements multiple times (Caddy, nginx, Traefik). Rejected:
  massive scope creep. The router is **already a raioz project** —
  no need for a plugin abstraction; reuse the sibling-project
  pattern.
- **`--meta-profile router-off` to disable Caddy.** Rejected: profiles
  filter projects, not raioz internals. Conflating the two violates
  the meta-profile semantics issue 048 established.
- **Skip Caddy when any sibling-dep declares `kind: router`.** Same
  end-state as `router:` but the trigger is buried in a sub-yaml.
  Less discoverable; the workspace umbrella is the right place to
  declare a workspace-level decision.

## Implementation status

**Not yet implemented.** Target: v0.8.0 (or v0.7.0 if scope allows
alongside issue 067 fase 2/3). Tracked locally as `PLAN-066`.

## References

- Issue 066 — original write-up with the hypixo case.
- ADR-008 — sibling-projects-as-deps (foundational pattern).
- ADR-024 — pre-up hook (lifecycle ordering pattern).
- Issue 048 / v0.6.0 — meta-profiles (composes cleanly).
- README.md — *"the developer uses their preferred tools"* —
  the principle this ADR reinstates.
