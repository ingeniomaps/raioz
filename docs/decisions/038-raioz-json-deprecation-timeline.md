# ADR-038: `.raioz.json` deprecation timeline

- **Status:** Accepted — implemented 2026-05-15
- **Date:** 2026-05-15
- **Drives:** issue 068

## Context

Raioz still accepts `.raioz.json` as a first-class config format
alongside `raioz.yaml`. ADR-031 traced the escalation plan for the
`version:` field inside YAML (v0.6 warning → v0.7 past-version
hard-error → v1.0 any-mismatch hard-error), but the parallel plan
to retire the JSON format itself was never written. Without that
decision, the dual-flow `isYAMLMode(deps)` branching (present in 7+
inspection commands — see issue 069) is permanent debt: every new
feature pays the dual-flow tax, and the loader keeps two parsers
indefinitely.

The JSON loader still hooks in at:

- `internal/cli/up.go` — resolves `.raioz.json` by default when no
  yaml is present.
- `internal/app/down.go::resolveDownConfigPath` — lists
  `raioz.yaml`, `raioz.yml`, `.raioz.json` as candidates.
- `internal/config/dependency_assist.go` — scans subdirectories
  looking for `.raioz.json` to auto-discover dependencies.
- `internal/config/deprecated.go` — emits warnings about
  **fields** deprecated inside JSON, never about the JSON format
  itself.

A team that onboards in 2026-Q3 with `.raioz.json` has no signal
that the format is legacy: the loader is silent, the JSON-specific
warnings only talk about individual fields. Quality auditor (issue
068) called the consequence: there is no published date on which
JSON support breaks, so nobody migrates.

## Decision

`.raioz.json` is on a four-release ramp toward removal. Each
release ratchets the signal louder; the deadline is published in
the warning itself so users see when the warning becomes an error.

### Timeline

- **v0.6** (already shipped) — silent; field-level warnings only.
- **v0.7** (this ADR, shipping 2026-05-15) — loud one-shot
  `output.PrintWarning` the first time any JSON loader fires in a
  process. Message:
  > `.raioz.json` is deprecated — run `raioz migrate yaml` to
  > convert. Support is removed in v0.8 (see ADR-038).
  Field-level warnings inside `internal/config/deprecated.go`
  continue to fire independently.
- **v0.8 (slipped — actual state)** — `LoadDeps` still warns +
  loads (no hard error). The v0.8 release cycle prioritised
  ADR-037 (router project) and the audit-triage v0.8.1-v0.8.3
  patches; the JSON hard-error was not implemented. Issue 027
  flagged the gap during the architecture review pass on
  v0.8.3. ADR-038 timeline updated 2026-05-16 to reflect
  reality:
  - **Hard-error target slipped to v0.9.** Bundle with the
    `scripts/dual-flow-baseline.txt` drain (ADR-039) so the two
    behaviours flip together — the dual-flow readers cannot
    drop their YAML-mode branch until `LoadDeps` returns only
    YAML-shaped results.
- **v1.0** — the public JSON loader is deleted. `raioz migrate
  yaml` becomes a stand-alone command with its own mini-loader;
  no other code path can read `.raioz.json`. Unchanged from
  the original plan.

### Why "loud one-shot" instead of "loud always"

The JSON loader is also called by `dependency_assist.go` when
scanning sub-project directories to auto-discover deps. A repo
with five JSON-shaped service dirs would emit the same banner five
times per `raioz up`. The banner fires through
`sync.Once`-equivalent dedup (`warnedJSONDeprecation sync.Once` in
`internal/config/deps.go`) so the signal lands once per process
without spam. Per-process scope; `ResetJSONDeprecationWarningForTest`
clears the dedup so tests can pin the firing order.

### Enforcement mechanism

- `internal/config/deps.go::LoadDeps` calls
  `output.PrintWarning(i18n.T("warning.json_format_deprecated"))`
  inside a `warnedJSONDeprecation.Do(...)`.
- The i18n key `warning.json_format_deprecated` is added to
  `internal/i18n/locales/{en,es}.json` so the message follows
  ADR-027 source discipline.
- The redundant slice-append in
  `internal/infra/config/loader_impl.go` is removed — the loader
  no longer threads a string through the warnings list, because
  the banner is emitted at the source.
- Tests:
  `internal/config/deps_deprecation_test.go::TestLoadDeps_DeprecationWarningFiresOnce`
  pins that the banner fires exactly once across multiple
  `LoadDeps` calls in the same process, and that the message
  contains the migration hint and `ADR-038`.
- `internal/config/deps_deprecation_test.go::TestLoadDeps_DeprecationWarningResets`
  pins that the test seam (`ResetJSONDeprecationWarningForTest`)
  works.

## Consequences

### Positive

- Users see a concrete deadline ("Support is removed in v0.8")
  the first time their `.raioz.json` loads. The migration command
  is named in the same line — no hunt through docs.
- Issue 069 (`isYAMLMode` dual-flow consolidation) has a real
  target: after v0.8 the dual-flow can collapse without a
  back-compat ramp.
- The escalation is published in this ADR so reviewers can hold
  the line — no silent extension of the deadline.

### Negative

- Teams with `.raioz.json` in CI see the warning until they
  migrate. Mitigation: `raioz migrate yaml` is one shot; the
  message names it.
- The migration window from v0.7 (warning) to v0.8 (hard error)
  is short — one minor release. Justified: the field-level
  deprecation warnings have already been live since v0.5; the
  format-level warning is the last reminder, not the first.

### Neutral

- The banner fires once per process. CI logs see it once per
  `raioz up`, not per dep-discovery pass. Field-level warnings
  inside `deprecated.go` still appear inline next to the deps that
  triggered them — different signal, different scope.

## Alternatives considered

- **Skip the v0.7 warning, jump straight to v0.8 hard error.**
  Rejected: matches ADR-027/ADR-029/ADR-031 ratchet style — one
  release of advisory warnings gives hand-edited configs grace
  before the binary refuses them.
- **Per-load banner (no dedup).** Rejected: `dependency_assist`
  scans every subdir, so a five-service JSON repo would print
  five identical banners per `raioz up`. Noise without signal.
- **Keep the JSON loader forever; only enforce via `raioz
  doctor`.** Rejected: doctor is opt-in. The dual-flow tax keeps
  growing with every new field added to YAML, because
  `cloneService`/`cloneInfraEntry` and `isYAMLMode` callers have
  to stay in lockstep (ADR-006). Removing the format is the only
  way out.
- **Move migration into `raioz init --from-json` instead of a
  dedicated `raioz migrate yaml`.** Rejected as out of scope: the
  existing `raioz migrate yaml` command works; reshuffling it
  into init is a separate decision.

## References

- Code:
  `internal/config/deps.go::LoadDeps`,
  `internal/config/deps.go::warnedJSONDeprecation`,
  `internal/infra/config/loader_impl.go::ConfigLoaderImpl.LoadDeps`.
- Tests: `internal/config/deps_deprecation_test.go`.
- i18n: `internal/i18n/locales/en.json`,
  `internal/i18n/locales/es.json` — key
  `warning.json_format_deprecated`.
- Issues: 068 (this ADR), 069 (`isYAMLMode` consolidation,
  unblocked by v0.8 removal).
- Related: ADR-027 (warning-level ratchet pattern), ADR-029
  (baseline ratchet pattern), ADR-031 (`version:` field
  escalation — sibling timeline for YAML schema bumps), ADR-035
  (`sync.Map` dedup pattern for once-per-process warnings).
