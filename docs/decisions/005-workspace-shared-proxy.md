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

### Orphan route-file GC on `down`

The teardown gate requires **two** signals to agree before
stopping the shared proxy: no persisted route files for any
project (`RemainingProjects() == 0`) **and** no other
workspace-labelled containers alive. The route-file signal counts
files on disk, not live projects — so a file left behind by a
project that crashed (or was killed outside raioz, e.g.
`docker compose down` directly) never gets removed by that
project's own `down` and pins the proxy alive forever, even for
the genuinely-last project out. The same stale file also injects
a route to a dead backend into every Caddyfile reload.

To close this, `handleSharedProxyDown` runs a garbage-collection
pass (`pruneOrphanRouteFiles`) **before** evaluating the gate: it
crosses every persisted route file against the set of live
workspace containers (same Docker-label source of truth as the
"last project leaves" probe) and deletes any file whose owning
project has zero live containers. After the GC, `RemainingProjects()`
reflects only genuinely-live projects and the gate decides correctly.

This deliberately relaxes the "each project only manages its own
route file" invariant: a `down` of project A may delete the
orphan file of crashed project B. That is safe because the file
is *demonstrably* orphaned (B has no containers), and the
alternative — an immortal pin requiring `raioz proxy stop` or a
manual `rm` — is worse and not discoverable.

### Host-run siblings: container absence is not proof of death

A project whose services run via `command:` (host process, or a
container the dev's own tooling creates — no raioz labels) owns
**zero** labelled containers even while fully alive. Its shared
deps do run as raioz containers, but those carry an empty
`com.raioz.project` *by design* (ADR-002). Both gate signals and
the GC were originally blind to such a project: a sibling's
`down` would prune its route file as orphaned, pass the gate, and
tumba the proxy out from under it (issue 021, workspace
`gouduet`). Two rules close the hole; both follow one principle —
**pruning and teardown require positive proof of death, never
mere absence of proof of life**:

1. **Live shared deps veto the teardown.** In
   `otherWorkspaceProjectsActive`, a workspace container with an
   empty project label and `com.raioz.kind=dependency` counts as
   occupancy: by ADR-002's own contract, shared deps survive
   individual downs until the last consumer leaves, so one still
   running means some consumer remains — even an unlabelled one.
   The proxy's own container (`kind=proxy`) still doesn't count.
   No false keep-alive from the leaving project's own deps: the
   orchestrated down stops those before `stopProxy` runs.

2. **The GC probes host-side liveness.** Route files persist the
   owning project's directory (`persistedProject.ProjectDir`,
   written by `SaveProjectRoutes`). Before pruning, the GC reads
   `<ProjectDir>/.raioz.state.json` and keeps the file if any
   recorded host PID is alive (`host.IsProcessAlive`). Files
   without the field — written pre-field, or by remote
   sub-projects (ADR-049), which have no local liveness signal at
   all — are treated as "liveness unknown" and never pruned; a
   re-`up` rewrites the file with the field. The GC prunes only
   when the project dir is known, the state is readable, and no
   recorded PID is alive.

3. **Live route targets veto the prune.** Rules 1 and 2 both miss
   the launcher pattern (ADR-025): a `command:` that shells out to
   `make start` → `docker compose up -d` daemonizes, so the
   recorded host PID is the launcher's and dies within seconds,
   while the resulting containers are user-owned and carry no
   raioz labels. Both probes read "dead" on a fully live project.
   The route file itself holds the missing link: its `Target`
   fields name those containers. Before pruning, the GC asks
   Docker whether any container-name target is running
   (`ProxyManager.RouteTargetsFor` → `docker.AnyContainerRunning`,
   the same by-name probe ADR-008 uses for siblings) and keeps the
   file if one is. `host.docker.internal` targets are excluded —
   they carry no container signal, and rule 2 already covers host
   processes. A file with zero container targets yields no signal;
   the earlier rules decide. A probe error keeps the file
   (fail-closed), same posture as the Docker-unreachable guard
   below.

The accepted cost mirrors the label-leak direction already
declared acceptable below: a workspace where every project
crashed but shared deps survived keeps the proxy alive until
`raioz clean` or a real last-consumer `down`. Keep-alive is
cheap; tearing down a live sibling's HTTPS (plus its persisted
routes) is not.

**Docker-unreachable guard:** the GC distinguishes "daemon
unreachable" from "zero containers" via
`docker.ListContainersByLabelsErr`. If the liveness probe errors,
the GC is skipped entirely (degrading to the pre-GC keep-alive
behaviour) rather than risk deleting a live project's routes on a
transient Docker outage. The error-swallowing
`ListContainersByLabels` is reserved for best-effort callers; any
caller that deletes state on the basis of absence must use the
error-returning variant.

### Atomic route file writes

`SaveProjectRoutes` writes each project file via temp file +
`os.Rename` in the same directory. A concurrent reader sees
either the previous version or the new version of the file —
never a truncated mid-write. Without this, two `raioz up` runs
in the same workspace could race and the Caddyfile regenerator
would `continue` past a half-written file, silently dropping
that project's routes until the next reload.

`loadAllProjectRoutes` still skips files that fail to read or
parse (a corrupt single file shouldn't block the whole
workspace), but now it logs at `Warn` level so the failure is
visible. Atomic writes mean a parse error is now a real
signal that something external touched the file, not normal
operation.

Note: this protects against single-machine concurrent reads vs
writes. A separate proxy-scoped lock for serializing the
Caddyfile reload step itself is planned in
`docs/issues/025-proxy-lock.md`.

## Consequences

### Positive

- One Caddy per workspace; no host port collision regardless of
  project count.
- Adding/removing projects is incremental — no full proxy
  restart, just a reload.
- Routes survive crashes (persisted to disk).

### Negative

- Concurrent `up` of two projects in the same workspace can
  still race on the Caddyfile reload step itself (the routes
  files are now atomic, but the multi-step "regenerate +
  reload" is not). Wave 1 issue 025 adds a proxy-scoped lock.
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

- Code: `internal/proxy/routes_persist.go`
  (`ListProjectsWithRoutes`, `RemoveRoutesFor`),
  `internal/proxy/caddyfile.go`,
  `internal/app/upcase/orchestration_proxy.go`,
  `internal/app/down_proxy.go` (`pruneOrphanRouteFiles`,
  `liveWorkspaceProjects`),
  `internal/naming/naming.go` (`WorkspaceProxyDir`)
- Orphan route-file GC: issue 020. Host-aware liveness for
  host-run siblings (`ProjectDir` in route files, shared-dep
  teardown veto): issue 021.
- Related: ADR-002 (shared deps lifecycle),
  ADR-010 (workspace lock that serializes the routes-dir mutator
  path this ADR introduces), Wave 0 issue 021, Wave 1 issue 025.
- Cross-lock interactions: see [docs/LOCKS.md](../LOCKS.md) for
  the matrix.
