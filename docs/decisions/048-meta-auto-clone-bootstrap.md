# ADR-048: Meta auto-clone bootstrap

- **Status:** Accepted
- **Date:** 2026-05-22

## Context

A `kind: meta` config (`internal/config/yaml_meta.go`, [ADR-041]) lists N
sub-projects by `path:` and expects every one of them to be already checked
out on disk before `raioz up`. Onboarding a new dev to an umbrella workspace
is therefore manual: clone the umbrella, look up each sub-repo URL in some
other place, authenticate, clone each one, then run `raioz up`. For partners
or contractors with partial access (e.g. the `web` and `bff` repos but not
the private `api`), the flow is impossible without a parallel mock or a
hand-rolled proxy.

This breaks the meta promise — "one command brings the whole stack up" only
holds when the dev already has the equivalent of a monorepo checked out.
Issue 020 (`docs/issues/020-meta-auto-clone-and-remote-fallback.md`) lays
out the full case including the remote-fallback half. This ADR covers the
**auto-clone half only**. The remote half ships in a follow-up (ADR-049,
not yet written) so we can land and validate the clone surface first
without coupling two distinct trust posture changes.

The primitives already exist: `internal/git/EnsureRepo` accepts a
`models.SourceConfig` with `Repo`, `Branch`, `Path`, and `Auth`, and
delegates to the readonly/editable variants. `internal/git/auth/` provides
`strict` (default — public-only hardening), `inherit` (delegate to dev's
global git config), and placeholder `gh`/`ssh` providers from issue 067
fase 1. So the auto-clone glue is the missing piece, not the auth/clone
machinery underneath it.

The constraint that forced the design:

- **`raioz up` already runs network-blocking code (image pulls).** Adding
  `git clone` to that critical path is consistent with the existing
  blast radius, not a new one. But the clone must be opt-in per project
  (declarative via `git:` on the meta entry) — not driven by detection,
  not by config inference, not by some "find a repo for this path" magic.
- **Trust posture has to mirror sibling deps ([ADR-008], [ADR-040]).** A
  meta auto-clone is functionally the same trust hop as a sibling project
  ref: the parent meta config gets to spawn an unknown sub-repo. The
  declarative `git:` URL must be visible in the meta `raioz.yaml`, never
  derived implicitly, so reviewers see exactly what gets fetched.

## Decision

Meta sub-projects gain an opt-in clone declaration. `raioz up` (meta mode)
bootstraps missing sub-projects via `git.EnsureRepo` before the
`runSingle` loop. Six load-bearing rules:

1. **Schema additions on `YAMLMetaProject` are opt-in and additive.**
   Three new fields:

   ```yaml
   projects:
     - path: ./api
       git: git@github.com:cubiko/oim-api.git    # required for auto-clone
       branch: develop                             # optional, default = remote HEAD
       auth: gh                                    # optional, default = strict
   ```

   Missing `git:` keeps the v0.8 behavior: the path must exist before
   `raioz up` or the run fails (or skips, when `optional: true`).
   Backward-compatible by construction.

2. **Resolution decides `MetaProject.Mode` at load time.** `LoadMetaConfig`
   inspects the filesystem and produces one of three modes per project:

   - `MetaModeLocal` — path exists on disk. No clone needed.
   - `MetaModeClone` — path missing AND `git:` declared. Bootstrap will
     attempt the clone.
   - `MetaModeSkip` — path missing, no `git:`, and `optional: true`.
     Bootstrap warns and continues; the project is dropped from the
     run.

   When path is missing, no `git:`, and `optional: false`, the load
   keeps Mode at `MetaModeLocal` and lets the existing v0.8 behavior
   surface the "no such file/directory" error at `runSingle` spawn
   time. This preserves backward compatibility for non-clone
   consumers — `raioz status` / `raioz down` / lint must parse configs
   that reference not-yet-checked-out paths without requiring the
   directory on disk. Load-time hard failures would force every
   read-only call site to special-case the missing path.

3. **Bootstrap runs once, in `MetaRunner.Up`, before the router phase
   AND before the consumer loop.** The bootstrap iterates entries in
   `projects:` and skips the standalone `router:` block — the router
   block alone has no `git:` field (it's a project reference, not a
   clone declaration). When a user wants the router auto-cloned, they
   declare it in both `projects:` (with `git:`) AND `router:` (which is
   supported per [ADR-037]: "The same path MAY also appear under
   `projects:`"). Bootstrap is skipped entirely when `--no-clone` is
   passed (CLI flag added in `up`'s flag set). `down` / `status` never
   trigger a clone — only `up`.

4. **Clone errors follow the existing `Optional` failure semantics from
   [ADR-041] § per-sub-command failure semantics.**

   - `optional: true` + clone fails → warn + audit event +
     `MetaSummary.Skipped: true`, do NOT abort the run.
   - `optional: false` + clone fails → abort the meta up with the clone
     error. No subsequent sub runs.

   This keeps the failure matrix small (no new failure modes; clone
   failures share the optional-skip path).

5. **`git.EnsureRepo` is the clone primitive.** No new package, no
   recreated machinery. The mapping is:

   ```go
   models.SourceConfig{
       Kind:   "git",
       Repo:   p.Git,
       Branch: p.Branch,
       Path:   p.Path,        // relative to baseDir
       Auth:   p.Auth,        // "" (strict) | "inherit" | "gh" | "ssh"
   }
   ```

   `git.EnsureRepo(src, baseDir)` takes care of: directory presence
   check, clone vs pull, auth provider dispatch, readonly vs editable
   pin. The meta layer only handles the loop, the warning ordering, and
   the audit trail.

6. **Audit events use the existing meta lifecycle envelope.** Bootstrap
   emits one `meta_clone` audit event per project attempted, with
   `correlation_id` propagated from the parent `meta_up`. Skipped
   (optional, failed) clones get `best_effort: true` on the event;
   hard-failed clones don't, mirroring the convention in
   `metaSubFailureDetails`.

The CLI surface change is one flag:

```bash
raioz up --no-clone     # skip auto-clone; missing+git: paths fail / skip
                         # per Optional. Useful for offline reproductions.
```

## Consequences

### Positive

- Onboarding becomes one command for the common case: `git clone <umbrella>
  && raioz up` clones everything declared and starts the stack.
- Failure modes are predictable: same `Optional` contract as the rest of
  meta, no new flags for partial-failure tolerance.
- Auth/clone primitives are reused — no duplicate machinery; future
  improvements to `internal/git/auth/` (issue 067 fase 2/3) automatically
  benefit meta bootstrap.
- Trust posture stays declarative: every URL meta clones is visible in
  the meta `raioz.yaml`. No registry, no implicit resolution, no
  side-channel.

### Negative

- `raioz up` (meta mode) now performs network I/O before any spawn.
  First-up time on a fresh checkout grows by the sum of clone durations.
  Caveat: this is the same blast radius as image pulls, which `up`
  already does — not a new category.
- New failure surface for `up`: DNS, auth, repo-not-found. Surfaced via
  `audit` + the existing meta summary; no special error class.
- `--no-clone` is the only way to opt out of the clone phase for a
  config that declared `git:`. Devs who want "don't clone yet, just plan
  the run" must either use `--no-clone` or remove the `git:` field
  locally.

### Neutral

- The router project can be auto-cloned too, with full trust implications
  per [ADR-037] § trust asymmetry. The bootstrap layer makes no
  distinction; ADR-037's guidance ("treat router.project as max-blast-
  radius") applies unchanged.
- Workspace coherence ([ADR-008] § workspace coherence) is unaffected:
  the cloned sub-projects each have their own `raioz.yaml` that gets
  validated by the existing `--audit-siblings` preflight when opt-in
  is requested.

## Alternatives considered

- **Driven by sibling-dep `project:` ([ADR-008]).** Rejected: sibling
  deps are per-service, not per-sub-project. Meta needs a project-level
  knob, and overloading sibling-dep semantics would force every sub-project
  to declare itself as a dep of a placeholder service.
- **Driven by an out-of-band `raioz clone <umbrella-spec>` step.**
  Rejected: an extra explicit step defeats the "one command" promise.
  Devs already type `raioz up` — clone-on-up is the natural extension.
- **Implicit clone based on git remote detection in the umbrella.**
  Rejected for trust posture: meta would have to either guess the
  URL (no, footgun) or read `.git/config` of the umbrella (no, brittle).
  Explicit `git:` URL keeps the contract auditable.
- **Single-binary monorepo bootstrap.** Rejected: meta exists precisely
  because sub-projects keep independent lifecycles, separate repos, and
  possibly separate access boundaries. Forcing a monorepo defeats the
  product.

## Won't do (this ADR)

- **Remote-mode fallback ([ADR-049], not yet written).** The remote
  half of issue 020 — `remote: https://api.staging` — covers a distinct
  trust posture (proxying to a host the dev doesn't control) and ships
  as ADR-049 + a separate PR. Decoupling lets us validate clone before
  taking on the proxy/TLS/auth-to-remote complexity.
- **`remoteAuth:` (header injection for private remotes).** Same trust
  category as ADR-049; deferred until concrete demand surfaces.
- **`--force-remote` flag.** Belongs with the remote-mode ADR; no place
  for it without the remote half.

## References

- Code: `internal/app/meta.go`, `internal/config/yaml_meta.go`,
  `internal/config/yaml_types.go`, `internal/git/clone.go`,
  `internal/git/auth/`
- Related: [ADR-008] (sibling spawn — trust hop precedent),
  [ADR-037] (router project — trust asymmetry),
  [ADR-040] (transitive trust audit),
  [ADR-041] (meta orchestrator — sequential, Optional, env contract)
- Issue: `docs/issues/020-meta-auto-clone-and-remote-fallback.md`

[ADR-008]: 008-sibling-projects-as-deps.md
[ADR-037]: 037-replaceable-edge-router.md
[ADR-040]: 040-sibling-mode-a-trust-transitive.md
[ADR-041]: 041-meta-orchestrator.md
