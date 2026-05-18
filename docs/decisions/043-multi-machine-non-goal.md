# ADR-043: Multi-machine workspaces — non-goal for v1.0

- **Status:** Accepted
- **Date:** 2026-05-16

## Context

`docs/ROADMAP.md` lists "multi-machine workspaces" as a
future / unscheduled item. No ADR documents whether this is an
acceptance ("will do in v2"), a rejection ("won't do"), or a
scoped placeholder. The next contributor or user who asks "can I
split my workspace across laptop + desktop?" has no answer in
the docs.

The current architecture is **local-only by construction**:

- Docker network is local (no overlay / Swarm / Mesh).
- Service discovery hardcodes `host.docker.internal` as the
  container→host bridge — single-host assumption.
- Workspace state lives in `RaiozStateDir()` per machine.
- Workspace + project locks are filesystem locks per machine.
- Audit log is per-machine (`<RaiozStateDir>/audit.log`).
- Caddy + TLS certs are per-machine.

Concretely: a user with a 16 GB laptop wanting to offload
postgres + kafka + elasticsearch to a 64 GB desktop cannot do
that with raioz today. The "no" is true; the "no" is silent.

## Decision

**Multi-machine workspaces are a non-goal for v1.0.** raioz
remains single-host. The ROADMAP item is downgraded to "v2
candidate, no scheduled work".

The "won't do in v1.0" rationale:

1. **Docker network is the fundamental coupling.** Multi-host
   container networking requires Swarm, K3s, or an overlay
   network (WireGuard / Tailscale mesh). All three are
   operational complexity raioz explicitly avoids — the project
   principle is *complement* existing tools, not replace them.

2. **Service discovery via `host.docker.internal`** is the
   single-host assumption baked into every runner
   (`internal/discovery/discovery.go:86`). Cross-host discovery
   would require either a service mesh (out of scope) or
   raioz-managed DNS (huge new surface).

3. **Workspace state coordination** would need a control plane:
   S3, etcd, or custom sync server. Lock semantics across
   machines require distributed consensus or strict
   active/passive. Multiplies complexity 10× for a use case
   that affects a minority of users.

4. **Audit log + certs are per-machine by design.** Multi-host
   forensics would require log shipping (Loki, Vector,
   anything) which is exactly the operational tool raioz aims
   to complement, not replace.

If multi-machine becomes a v2 candidate, the sketch (not a
commitment, just a starting point):

- Service-to-machine mapping declarative
  (`services.<n>.host:`).
- Discovery via WireGuard / Tailscale mesh DNS (operator
  responsibility — raioz documents the contract).
- Local raioz proxy bridges queries to remote services via the
  mesh.
- Workspace catalog in a remote blob (git repo / S3) — pluggable
  backend.
- Lock semantics: optimistic, with conflict-detection on state
  reconciliation.

## Consequences

### Positive

- Users with multi-machine workflows understand the limit
  explicitly instead of discovering it by trial and error.
- Tickets asking "why doesn't raioz work across hosts" have a
  canonical answer.
- Roadmap commitment honoured: v2 candidate, not abandoned, but
  not on the v1.0 critical path either.

### Negative

- Some users will adopt alternatives (Docker Compose remote
  contexts, custom orchestration). That's a real loss for those
  users — but accepting the loss is preferable to shipping a
  half-finished multi-host story.

### Neutral

- Re-opening this ADR is a deliberate v2 trigger. Concrete
  demand from multiple users would justify revisiting the
  sketch above; absent that, this ADR stands.

## Alternatives considered

- **Docker Swarm integration.** Rejected. Swarm is operational
  complexity raioz exists to avoid; user-friendly raioz on top
  of Swarm is two product surfaces fighting for the same role.
- **Tailscale-mesh assumption.** Tailscale is great but it is a
  commercial dependency. raioz cannot mandate it. Documenting
  it as one *possible* user choice in the v2 sketch is fine.
- **Stay silent (no ADR).** Current state. Costs are unbounded
  re-derivation by every contributor and user who asks.

## References

- Code: `internal/discovery/discovery.go` (host-gateway
  hardcoded), `internal/naming/state.go` (per-machine state)
- Related: ADR-005 (workspace-shared proxy is per-machine),
  ADR-022 (state paths are per-machine), ADR-037 (router project
  is per-workspace, not multi-host)
- Docs: `docs/ROADMAP.md` — downgrade pending update
