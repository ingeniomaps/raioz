# ADR-024: Two-phase pre-hook — `pre:` and `preUp:`

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

`raioz.yaml` exposed a single `pre:` hook that ran as the very first
step of `raioz up`, before infra startup, sibling-spawn (ADR-008),
network creation, or service dispatch. The contract worked for the
original use case: render env files, fetch secrets, generate
templates — things that touch only the local filesystem.

A sibling-deps consumer (issue 046 reproduction) declared:

```yaml
dependencies:
  keycloak:
    project: ../keycloak
    requiredHostname: sso

pre: make createdb
```

The `make createdb` script connects to `hypixo-postgres` (a
container spawned by the keycloak sibling project) and runs `CREATE
DATABASE`. The pre-hook ran first, the sibling-spawn had not yet
happened, DNS lookup failed, and `raioz up` aborted before keycloak
could be brought up. The user's only escape was to move bootstrap
into the service `command:` (forcing the dev loop into a container,
breaking host-process workflows like `air -c .air.toml`).

The asymmetry is the architectural problem: a single hook name
collapsed two distinct phases into one. There's no way to express
"this hook needs to talk to my dependencies."

## Decision

Add a second hook, `preUp:`, that fires **after** infra and
sibling-spawn but **before** this project's own services start.
`pre:` keeps its early-phase semantics; new field is additive,
backward compatible.

### Schema

```yaml
pre:   ./scripts/fetch-secrets.sh    # before everything (current)
preUp: make createdb                  # after deps, before services
post:  rm -f .env.*.tmp               # after services (current)
```

Both `pre:` and `preUp:` accept the same string-or-list form,
joined with ` && ` for the bridge (`internal/config/yaml_bridge.go`).
Both bridge into `models.Deps` as plain `string` fields
(`PreHook` / `PreUpHook`). Both run via the same
`sh -c <cmd>`-per-command loop in `internal/app/upcase/hooks.go`.

### Failure semantics

`preUp:` failures abort the run, same as `pre:`. The argument:
services have not started yet, so aborting here is the cheapest
correct response. Continuing would put services up against a
half-bootstrapped state (no DB created, no schema applied), which
is harder to diagnose than a clear "preUp failed" error.

`post:` keeps its "failures are warnings, not errors" rule —
services are already running and the user can fix the post-hook
without redoing the up.

### Invocation point

`preUpHookExec` lives inside `processOrchestration` (the YAML-mode
orchestrator), between the infra-block and the service-block:

```
processOrchestration
├─ Step 0  cleanStaleHostProcesses
├─ Step 1  detectRuntimes
├─ Step 1b/c  port allocation
├─ Step 2  start infra (incl. applySiblingVerdict / spawnSibling)
├─ Step 2.5  preUpHookExec        ← new
├─ Step 3  start services
└─ Step 4  start proxy
```

Legacy mode (host services + generated compose, pre-YAML
orchestrator) does not get `preUp:`. The legacy path is dying and
sibling-deps (ADR-008) are a YAML-only feature, so the value/cost
ratio for extending the legacy path is poor. Users on legacy
configs are not regressed — `preUp:` is opt-in.

### Host DNS caveat

The hook runs on the host via `sh -c`. The host doesn't share the
Docker network's DNS, so `make createdb` cannot reach the dep by
its container name (`hypixo-postgres`). It must use the published
host port (`localhost:5432`) or run the command inside a container
(`docker exec <dep> psql ...`). Documented in CONFIG_REFERENCE.md
and surfaced in the `PreUpHookFailed` error's suggestion block.

## Implementation status

Landed in this commit:

- `internal/domain/models/config_deps.go`: `PreUpHook string` added
  to `Deps`.
- `internal/config/yaml_types.go`: `PreUp YAMLStringOrSlice` added
  to `RaiozConfig` with `since: v0.5.0` marker.
- `internal/config/yaml_bridge.go`: bridge joins `cfg.PreUp` with
  ` && ` into `deps.PreUpHook`.
- `internal/config/{filter,ignore_filter,deps}.go`: PreUpHook
  propagated through all four `Deps{}` clone sites (ADR-006).
- `internal/app/upcase/hooks.go`: `preUpHookExec` mirrors
  `preHookExec` with its own i18n key and error code.
- `internal/app/upcase/orchestration.go`: invocation at Step 2.5.
- `internal/errors/orchestrator.go`: `ErrCodePreUpHookFailed` +
  `PreUpHookFailed(...)` builder with suggestion text.
- `internal/i18n/locales/{en,es}.json`:
  `up.running_pre_up_hook` + `up.pre_up_hook_done`.
- `docs/CONFIG_REFERENCE.md`: new "Pre-up vs pre" section + table
  entry.
- Tests:
  - `internal/config/yaml_pre_up_test.go` (scalar / list / coexistence
    with `pre:`).
  - `internal/app/upcase/more_flows_test.go::TestPreUpHookExec`
    (empty / success / failure / chain).

## Consequences

### Positive

- Sibling-deps consumers can run bootstrap that talks to their
  dependencies without containerizing the dev loop. The hypixo
  reproduction case becomes a single-line config addition.
- The two phases (preparation-local vs preparation-of-deps) get
  distinct verbs, so config intent is readable at a glance.
- Failure surface is preserved — `pre:` users see no change.

### Negative

- One more concept for new users to learn. Mitigated by the
  table entry pointing at the "Pre-up vs pre" section with the
  ASCII flow diagram.
- `preUp:` runs on the host, so the natural reflex ("just use the
  container DNS name") fails. The error's suggestion text and the
  doc surface this explicitly. Users who can't avoid container DNS
  should fall back to `command:` chaining for that one service.

### Neutral

- The legacy (non-YAML) flow doesn't get `preUp:`. Acceptable
  because (a) legacy is dying, (b) sibling-deps don't exist in
  legacy mode, (c) the legacy `processCompose` path doesn't have
  a clean "after deps, before services" gap.

## Alternatives considered

- **Auto-detect siblings and reorder `pre:` silently** (issue 046
  option B). Rejected: same `pre:` keyword silently changes
  semantics based on whether `dependencies:` declares a `project:`
  field. Users would lose the ability to reason about timing
  locally.
- **Document only** (option C). Cheapest, but pushes every
  sibling-deps user to discover the same workaround
  (containerize the bootstrap). Doesn't scale.
- **Three-phase split (`pre:` / `preInfra:` / `preServices:`)**.
  More fine-grained, but the only documented use case is
  "post-infra, pre-services." Splitting further is YAGNI.

## References

- Code: `internal/app/upcase/hooks.go::preUpHookExec`,
  `internal/app/upcase/orchestration.go` Step 2.5,
  `internal/errors/orchestrator.go::PreUpHookFailed`.
- Tests:
  `internal/config/yaml_pre_up_test.go`,
  `internal/app/upcase/more_flows_test.go::TestPreUpHookExec`.
- Docs: `docs/CONFIG_REFERENCE.md` ("Pre-up vs pre" section).
- Issue: 046.
- Predecessor ADRs: [ADR-008](008-sibling-projects-as-deps.md)
  (sibling-deps lifecycle), [ADR-006](006-clone-functions-sync.md)
  (clone-functions invariant the PreUpHook field obeys).
