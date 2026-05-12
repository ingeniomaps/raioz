# ADR-005: Workspace-shared proxy lifecycle

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented)

## Context

A workspace groups multiple raioz projects on the same Docker
network so they can resolve each other by DNS. Each project
running its own Caddy on host port 443 would collide
immediately. The early workaround was disabling the proxy when
multiple projects shared a workspace — degrading the feature.

We needed a model where one Caddy fronts every project in a
workspace, but where each project's `up` and `down` only
affects its own routes.

## Decision

When `workspace:` is set, raioz runs a single Caddy container
named `{workspace}-proxy`. Each project persists its own routes
to a separate file:

```
${WorkspaceProxyDir()}/<workspace>/routes/<project>.json
```

The Caddyfile served to Caddy is the **union** of every project's
routes, regenerated whenever a project's `up` or `down` mutates
its file. `raioz down` removes the leaving project's routes
file, regenerates the Caddyfile, and reloads Caddy — without
touching other projects' routes.

The Caddy container itself is torn down only when the **last**
project in the workspace runs `down` (detected via Docker
labels: no other `com.raioz.workspace=<ws>` containers remain).

## Consequences

### Positive

- One Caddy per workspace; no host port collision regardless of
  project count.
- Adding/removing projects is incremental — no full proxy
  restart, just a reload.
- Routes survive crashes (persisted to disk).

### Negative

- Concurrent `up` of two projects in the same workspace races
  on the Caddyfile/routes dir. Mitigated by Wave 0 issue 021
  (atomic writes) and Wave 1 issue 025 (proxy lock).
- The routes dir is shared state outside Docker. State location
  has migrated (issue 015) and may migrate again as XDG
  conventions evolve.
- "Last project leaves" detection depends on Docker label
  presence being authoritative — leaks of labelled containers
  would defer the teardown.

### Neutral

- Projects without `workspace:` continue running their own
  Caddy as before.

## Alternatives considered

- **Caddy per project on different host ports** — defeats
  Caddy's role of unifying URLs; users would need to remember
  ports.
- **Single global Caddy across all workspaces** — leaks names
  across workspace boundaries; conflicts on `*.localhost`.
- **Refcount file** — fragile; same reasons as ADR-002.

## References

- Code: `internal/proxy/routes_persist.go`,
  `internal/proxy/caddyfile.go`,
  `internal/app/upcase/orchestration_proxy.go`,
  `internal/naming/naming.go` (`WorkspaceProxyDir`)
- Related: ADR-002 (shared deps lifecycle), Wave 0 issue 021,
  Wave 1 issue 025
