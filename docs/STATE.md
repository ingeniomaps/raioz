# State files

raioz writes five categories of state. Each has its own ADR, but
nothing single said "here are the files, here's what each one is
for, here's who writes and deletes them." This page is that
matrix. Companion to [LOCKS.md](LOCKS.md) (the serialization
companion) — between the two you can answer "where does this go
and how is it protected" in 30 seconds.

## At a glance

| File | Path | Scope | Purpose | ADR |
|------|------|-------|---------|-----|
| `.raioz.state.json` (LocalState) | `<project>/` (project cwd) | per-project, per-cwd | Runtime overrides Docker can't tell us: host PIDs, dev-mode swaps, ignored services, sibling deferrals, compose path | ADR-011 (origin) |
| `raioz.root.json` | `<RaiozStateDir>/workspaces/<project>/` | per-project | Drift-detection snapshot of the resolved `Deps` after most recent `up` | ADR-023 |
| `audit.log` (+ `.1` rotation tail) | `<RaiozStateDir>/` | per-machine, append-only JSONL | Forensic history of lifecycle / dev / sibling / drift events | ADR-020, ADR-022, ADR-027 |
| `routes/<project>.json` | `<WorkspaceProxyDir>/routes/` | per-project per-workspace | Persisted proxy route entries; merged into the workspace's `Caddyfile` | ADR-005 |
| `cert.pem` + `cert-key.pem` | `~/.raioz/certs/<domain>/` | per-domain (machine) | mkcert-issued local TLS cert + private key | ADR-003 |

Plus transient artifacts (not "state" in the sense of "raioz
reads it later"):

- `<WorkspaceProxyDir>/Caddyfile` — regenerated from
  `routes/*.json` on every `Reload` (ADR-005).
- `<RaiozStateDir>/<workspace>/` — proxy-managed assets per
  workspace (Caddy mount points, generated configs).

## Layout on disk

```text
~/.local/state/raioz/                  ← naming.RaiozStateDir() (ADR-022)
├── audit.log                          ← machine-scope, JSONL, rotated at 10 MiB
├── audit.log.1                        ← rotation tail
├── workspaces/
│   └── <project>/
│       └── raioz.root.json            ← drift-detection snapshot
└── <workspace>/
    ├── Caddyfile                      ← regenerated each Reload
    └── routes/
        └── <project>.json             ← one persisted route block per project

<project>/                             ← user's project directory (cwd)
└── .raioz.state.json                  ← LocalState — runtime overrides

~/.raioz/                              ← legacy root (pre-ADR-022) for certs only
└── certs/
    └── <domain>/
        ├── cert.pem
        └── cert-key.pem
```

`naming.RaiozStateDir()` resolves to (in order): `RAIOZ_HOME` →
`$XDG_STATE_HOME/raioz` → `~/.local/state/raioz` (ADR-022).
`WorkspaceProxyDir()` returns `<RaiozStateDir>/<workspace>/` for
workspace-shared mode (ADR-005). The cert dir sits at the
legacy `~/.raioz/certs/` and is not migrated; the migrator
moves runtime state, not crypto material. See "Open question"
below.

## Quién escribe qué

| File | Writer | Trigger |
|------|--------|---------|
| LocalState | `internal/state/project_state.go::SaveLocalState` | `raioz up` (after dispatch), `raioz dev` (promote/revert), `raioz ignore` |
| `raioz.root.json` | `internal/root/root.go::Save` | `raioz up` (`upcase.saveState`) — once per up, after services + infra are tracked |
| `audit.log` | `internal/audit/audit.go::Log` / `LogWithContext` | every event in the OBSERVABILITY.md matrix that's currently emitted (lifecycle up/down/restart, dev promote/revert, sibling deferred, drift, conflict resolved, service assisted) |
| `routes/<project>.json` | `internal/proxy/routes_persist.go::SaveProjectRoutes` | `raioz up`'s `startProxy` after `AddRoute` for every service |
| `Caddyfile` | `internal/proxy/caddyfile.go::generateCaddyfile` | indirect — every `Reload` and the first `Start` |
| certs | `internal/proxy/certs.go::EnsureCerts` | proxy `Start` when `tlsMode == mkcert` and the SAN-validated cert is missing |

## Quién borra qué

| File | Deleter | Trigger | Notes |
|------|---------|---------|-------|
| LocalState | `internal/state/project_state.go::RemoveLocalState` | `raioz down` (orchestrated path) when `localState.Project == ""` (orphan from old binary); otherwise the file is rewritten with `HostPIDs` cleared so subsequent reads see "not running" | Selective `raioz down <svc>` does NOT remove the file — other services may still be tracked |
| `raioz.root.json` | `internal/root/root.go::Delete` | `raioz down` (orchestrated) when `len(leftovers) == 0` per ADR-023 | Selective down does NOT delete it (only a subset was torn down) |
| `audit.log` | `internal/audit/audit.go::rotateIfOverCap` | size > 10 MiB → renamed to `audit.log.1` (overwriting any prior `.1`) | Never deleted outright; tail is recoverable |
| `routes/<project>.json` | `internal/proxy/routes_persist.go::RemoveProjectRoutes` | `raioz down`'s `stopProxy` | Last project out of the workspace also stops the Caddy container itself (ADR-005) |
| `Caddyfile` | regenerated, not deleted | every `Reload` — old content overwritten | If the last project leaves the workspace the file remains until the workspace dir is cleaned manually |
| certs | manual (`rm -rf ~/.raioz/certs/<domain>`) | n/a — raioz never deletes user-trusted CAs | Per-domain isolation makes manual cleanup safe (ADR-003) |

## LocalState vs raioz.root.json — the project-scoped split

The two files are the most common confusion point: both live
per-project, both get cleaned at down. They are conceptually
different:

| Concern | LocalState (`.raioz.state.json`) | `raioz.root.json` |
|---------|----------------------------------|-------------------|
| Where | next to the user's `raioz.yaml` | under `<RaiozStateDir>/workspaces/<project>/` |
| Survives `cd elsewhere && raioz logs` | No — file is per cwd | Yes — central state dir |
| Read at | every command that needs PIDs or override flags | next `up` only, for drift detection |
| Mutated during | dispatcher start/stop, dev promote, ignore add/remove | `upcase.saveState` (one write per up) |
| Schema | runtime overrides (PIDs, dev swaps, deferred siblings, compose path) | full resolved `Deps` snapshot (services, infra, env, metadata) |
| What it answers | "what did the previous `up` actually start, and where?" | "did the YAML change since last `up`?" |

**Heuristic:** if the new info changes per-up (PIDs, dispatch
results, sibling defer decisions), it's LocalState. If it's a
post-resolution snapshot of `raioz.yaml`, it's `raioz.root.json`.

## Adding a new state file

1. Pick its scope and put it on the right axis. Per-project
   transient → LocalState (extend the struct). Per-project
   snapshot → `raioz.root.json` (extend `RootConfig`). Per-machine
   forensic → `audit.log` (new `EventType`).
2. **State files mirror reality** — ADR-023. Anything you
   write must have a paired delete on `down`. Same rule:
   selective down skips, full down deletes.
3. Put the path under `naming.RaiozStateDir()` (ADR-022) so it
   participates in the unified migration. Avoid hardcoding
   `~/.raioz/...` for new state (the cert dir is the
   grandfathered exception).
4. If the file is workspace-scoped (shared across projects in
   one workspace), put it under `WorkspaceProxyDir()` or a
   parallel `<RaiozStateDir>/<workspace>/` subdir.
5. Update this page (`docs/STATE.md`) and CLAUDE.md's invariants
   list if the new file establishes a new scope.
6. If concurrent processes can write to it, see
   [LOCKS.md](LOCKS.md) for the lock-hierarchy rules.

## Open question: certs under `~/.raioz/` outside `RaiozStateDir()`

`internal/proxy/certs.go::CertsDir()` returns
`<home>/.raioz/certs/` — hardcoded, predating ADR-022. The
migrator at startup lifts `~/.raioz` content into `RaiozStateDir`,
but certs are explicitly excluded by being read straight from
`~/.raioz/certs/` whether or not the migration ran. The result is
benign — both old and new installs find their certs in the same
place — but the inconsistency is worth noting:

- Pros of keeping certs at `~/.raioz/certs/`: stable across
  raioz versions; mkcert's `CAROOT` env var stays consistent;
  trust-store entries the user already approved don't need
  re-installation.
- Pros of moving to `RaiozStateDir`: single tree to back up,
  single tree to wipe with `rm -rf $(raioz hosts ...)` — same
  story as state.

No decision required for v0.5.x; flag for v0.6 review.

## References

- ADRs: [ADR-003](decisions/003-cert-namespacing.md) (certs),
  [ADR-005](decisions/005-workspace-shared-proxy.md) (workspace
  proxy + routes),
  [ADR-011](decisions/011-runtime-state-single-source.md)
  (LocalState canonical source),
  [ADR-020](decisions/020-audit-rotation.md) (audit rotation),
  [ADR-022](decisions/022-unified-state-paths.md) (unified
  `RaiozStateDir`),
  [ADR-023](decisions/023-state-mirrors-reality.md) (state files
  mirror reality — the `down` deletes contract).
- Code: `internal/state/project_state.go`,
  `internal/root/root.go`,
  `internal/audit/audit.go`,
  `internal/proxy/routes_persist.go`,
  `internal/proxy/certs.go`,
  `internal/naming/paths.go`,
  `internal/naming/naming.go` (`WorkspaceProxyDir`).
- Issue: 052.
