# ADR-008: Sibling raioz projects can act as dependencies (modes A and B)

- **Status:** Accepted
- **Date:** 2026-05-12 (retroactively documented; introduced for
  issue #26)

## Context

raioz workspaces let multiple projects share infrastructure. But
some teams have a sibling project (e.g. `keycloak/`) that is
itself a full raioz project — not a single image. Forcing the
consumer to declare it as `image:` defeats the point: the
sibling has its own services, compose files, and lifecycle.

We needed a way to say "this dep is another raioz project; if
it's up, use it; if it's not, decide whether to fail or fall
back to an image".

## Decision

A `dependencies.<n>` entry can point to a sibling raioz project
in one of two modes:

- **Mode A — `project: ../sibling`.** The sibling **is** the
  dependency. No `image:`. If the sibling is not active, raioz
  spawns `raioz up` recursively inside its directory.
- **Mode B — `siblingProject: ../sibling` paired with `image:`.**
  Defer to the sibling if it's already up; otherwise fall back
  to the declared `image:`. Useful for CI and contributors
  who don't have the sibling cloned.

Invariants:

1. **`raioz down` never touches the sibling.** Mode A is
   detected via `entry.Inline.Project != ""` and skipped
   unconditionally. Mode B is tracked via
   `LocalState.DeferredToSibling` (overwritten on every up, so
   stale entries clear themselves) and skipped on next down.
2. **Workspace coherence is mandatory.** Both sides must declare
   the same `workspace:`. Cross-workspace siblings fail at
   `decideSibling` time before any spawn or probe.
   - **Transitive note (issue 031)**: the check is pair-wise.
     In a 3-project chain A→B→C, coherence is only enforced
     between A↔B and B↔C separately. If B declares no
     `workspace:` and inherits a default, A and C may end up
     on different Docker networks even when both declare the
     same workspace. The operator is responsible for declaring
     `workspace:` explicitly on every project in a spawn chain
     longer than 2. Transitive enforcement is out of scope for
     v1.0; tracked as a v2 candidate.
3. **Cycle detection via `RAIOZ_SIBLING_STACK`** env var
   (`os.PathListSeparator`-joined directories). Parent appends
   its own dir before exec; child reads it in
   `checkSiblingCycle` and fails fast on A→B→A.
4. **Mode A spawn uses `os.Executable()`** (overridable via
   `spawnRaiozBinary` for tests) and streams stdout/stderr
   line-by-line with a `[sibling: <name>]` prefix. Errors
   include `cd <dir> && raioz up` so the user can diagnose
   without recalling where the spawn came from.
5. **`requiredHostname:` is checked against the sibling's
   declared hostnames** (yaml-declared, not live Caddyfile),
   pre-spawn for mode A and pre-defer for mode B. Skipped when
   mode B falls back to the image.
6. **`config.Infra` clones must include** `Project`,
   `SiblingProject`, `RequiredHostname` — same class of bug as
   ADR-006.

## Consequences

### Positive

- Teams compose their projects naturally without forcing every
  shared service into a Docker image.
- Lifecycle isolation: consumer can't accidentally tumba the
  sibling.
- Cycle detection prevents infinite recursion.

### Negative

- A new failure mode: sibling goes down mid-session, consumer
  silently fails DNS. Documented; addressed in Wave 1 issue 026
  (readiness probe).
- Recursive spawns make logs noisier; the
  `[sibling: <name>]` prefix helps but doesn't eliminate the
  cognitive load.
- Workspace coherence requirement makes setup more rigid than
  pure image deps.

### Neutral

- Both modes are opt-in. Projects without `project:` or
  `siblingProject:` keys behave exactly as before.

## Supersedes the name-collision prompt (removed 2026-09-07)

Before this ADR, `checkDependencyProjects` guessed at the same need:
it walked every service's `dependsOn`, matched each name against the
global state, and — when the match happened to carry no services —
asked whether to replace "the running project" and tried to stop it.

Three things were wrong with it, and this ADR fixes all three by being
explicit. The trigger was a **name collision against global state**,
not a declared relationship, and global state accumulates: on the
maintainer's machine it held 81 projects, 15 of them empty, including
scratch projects created an hour earlier — any of which could fire the
prompt for an unrelated dep that shared a name. Its "no services means
a command-based project" test was `.raioz.json` vocabulary; in YAML an
empty service list just means a project that only declares
dependencies. And its stop ran the sibling's `commands.down`, so it
had been unable to stop anything since ADR-038 without anyone
noticing.

`dependencies.<name>.project:` (mode A) and `siblingProject:` (mode B)
are the supported way to depend on a sibling: declared rather than
guessed, with cycle detection and workspace coherence, and `down`
never touching the sibling. The old flow and its prompt were removed.

## Alternatives considered

- **Sibling must export an image** — defeats the point;
  rejected.
- **Single mode (mode A only, no image fallback)** — blocks CI
  use cases where the sibling isn't cloned; rejected.
- **No cycle detection, trust the user** — easy to footgun
  with mutually-referencing projects; rejected.

## References

- Code: `internal/app/upcase/sibling_dispatch.go`,
  `internal/app/upcase/sibling_spawn.go`,
  `internal/docker/sibling_probe.go`,
  `internal/config/sibling_resolver.go`
- Related: ADR-006 (clone-sync — same risk class),
  ADR-010 (workspace lock the child must still take),
  ADR-026 (Pdeathsig — kernel-level reap of the spawned child),
  Wave 1 issue 026 (readiness probe), GitHub issue #26.
- Cross-lock interactions: see [docs/LOCKS.md](../LOCKS.md) for
  the matrix — `RAIOZ_SIBLING_STACK` is documented there as the
  bypass signal for the project lock.
