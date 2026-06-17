# ADR-051: A workspace-shared dep has one host port, fixed by the first `up`

- **Status:** Accepted
- **Date:** 2026-06-17

## Context

A workspace-shared dependency is a single container shared by every
project in the workspace (ADR-002: `{workspace}-{dep}`, project label
omitted, `ImageRunner.Start` idempotent). But port allocation never
learned about sharing.

`AllocateHostPorts` is a pure, per-project function. With
`publish: true` it walks `Expose` and calls `findFreePort`, probing the
host with a TCP listener. So when a 2nd project comes up against an
already-running shared dep:

- **Divergent host port.** The 1st project published the shared
  container on host `6379`. The 2nd project's allocator sees `6379`
  "busy" and bumps to `6380`, then service discovery injects
  `<DEP>_URL=…:6380` — a port nobody serves. A host-process consumer
  silently fails to reach the dep (issue 020 b).
- **False conflict.** If the user pins `publish: [6379]` to force
  consistency, the bind preflight finds `6379` occupied. Because the
  shared container omits `com.raioz.project` (ADR-002),
  `IdentifyPortOccupant` reports project `''` and the preflight treats
  it as a foreign conflict instead of the project's own shared dep
  (issue 020 a).

The shared dep is workspace-owned, so its host port must be a single
workspace-wide value — not re-derived per project.

## Decision

A workspace-shared dependency keeps **one host port across the
workspace, fixed by the first `up`**. Later projects read it from the
running container rather than allocating their own.

- **Source of truth is the live container, not persisted state.**
  `docker.GetPublishedHostPort(ctx, container, containerPort)`
  (`internal/docker/published_port.go`) parses `docker port` and returns
  `0` when the container isn't running / unpublished / docker is
  unreachable, so callers degrade to normal allocation.
- **Pin the allocation.** `reuseSharedDepHostPorts`
  (`internal/app/upcase/port_alloc_locked.go`) runs after
  `AllocateHostPorts`, before the bind check, and overwrites
  `Deps[name].Mappings[].HostPort` with the live port. It only touches
  deps that are auto-published (`!Explicit`) AND shared
  (`naming.IsSharedDep(override)`). Explicit pins stay sacred;
  per-project deps are never rewritten.
- **Reuse, don't conflict, in the preflight.** `IdentifyPortOccupant`
  surfaces the occupant's `com.raioz.workspace` and `com.raioz.service`
  labels; `isOwnContainer` (`port_resolve.go`) treats a port held by a
  shared dep of this workspace that the project also declares as reuse
  (match on workspace + service, since the project label is absent),
  not a conflict against project `''`.

The host port is read live and **not** persisted in
`shared-deps.json` (ADR-050). That registry tracks *which projects
reference* a dep (a fact only raioz knows); the published port is a fact
Docker already owns. Persisting it would add a schema field that can
drift from the running container.

## Consequences

### Positive

- A host-process consumer of a shared dep gets a `<DEP>_URL` that points
  at the real published port, across any number of projects.
- The `publish: [pinned]` shared-dep case no longer raises a false
  `PORT_CONFLICT` against project `''`.

### Negative

- One extra `docker port` call per shared, auto-published dep mapping,
  made inside the ports flock — kept cheap (single invocation, returns
  fast / `0` when absent).

### Neutral

- The published `ports:` string handed to `ImageRunner` for the 2nd
  project is cosmetic: `Start` is a no-op against the already-running
  shared container (ADR-002). What matters is that the pinned `HostPort`
  flows to discovery.

## Alternatives considered

- **Persist the host port in `shared-deps.json` (ADR-050).** Avoids the
  docker call but adds a drift-prone schema field for a fact Docker
  already owns. Rejected to keep Docker the single source of truth for
  published ports.
- **Coordinate ports via a workspace-wide lock at allocation time.**
  Heavier; the live read already serializes on reality without new
  shared state.

## References

- Code: `internal/docker/published_port.go`,
  `internal/app/upcase/port_alloc_locked.go`,
  `internal/app/upcase/port_resolve.go`,
  `internal/docker/ports.go` (`IdentifyPortOccupant`)
- Related: ADR-002 (shared deps workspace-scoped), ADR-050 (shared-dep
  refcount)
- Issue: `docs/issues/020-shared-dep-host-port-inconsistent-across-projects.md`
