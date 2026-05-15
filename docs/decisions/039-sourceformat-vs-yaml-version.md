# ADR-039: `Deps.SourceFormat` is independent of yaml `version:`

- **Status:** Accepted — implemented 2026-05-15
- **Date:** 2026-05-15
- **Drives:** issue 070

## Context

Two distinct "schema version" dimensions had been overloaded onto a
single struct field:

- `Deps.SchemaVersion` (internal) — used as a `"yaml"|"legacy-json"`
  discriminator via the magic literals `"2.0"` and `"1.0"`. Set in
  the yaml bridge and the json loader.
- `version:` (yaml field, exposed to users) — declares the schema
  version the yaml file targets. Today the only valid value is
  `"1"`; future schema breaks bump to `"2"`, `"3"`, ... gated by
  ADR-031.

Both lived as "schema version" in code (`CurrentSchemaVersion = "1"`,
`Deps.SchemaVersion = "2.0"`, `cli.SchemaVersion = "1.0"`). They are
unrelated dimensions:

- **SourceFormat** is invariant of the loader that produced the
  struct. A yaml file in any schema version 1/2/3 always lands as
  `SourceFormat == "yaml"`. A json file always lands as
  `SourceFormat == "legacy-json"`.
- **`version:`** is invariant of the schema fields the file uses.
  When schema breaks bump `"1"` → `"2"`, the source format stays
  `"yaml"` — only the field set changes.

The collision happens because `Deps.SchemaVersion = "2.0"` was
chosen as a magic discriminator when the yaml loader was added in
v0.2. Reusing the legacy field avoided a struct change in the
moment but trained every reader to think of "schema version" as a
single concept.

The seven inline `deps.SchemaVersion == "2.0"` checks (issue 069)
keep the magic literal honest only by convention; if `"2"` ever
needs to mean a real schema bump, the discriminator and the schema
counter collide.

## Decision

Introduce `Deps.SourceFormat` as the canonical "where did this come
from" discriminator. `Deps.SchemaVersion` survives as a deprecated
legacy field until v1.0 so the seven inline `== "2.0"` readers
(issue 069) can be collapsed incrementally through `SelectFlow`
without a big-bang rewrite.

### Type and stamping

```go
// internal/domain/models/config_deps.go
type SourceFormat string

const (
    SourceFormatLegacyJSON SourceFormat = "legacy-json"
    SourceFormatYAML       SourceFormat = "yaml"
)

type Deps struct {
    SchemaVersion string       `json:"schemaVersion"` // Deprecated: use SourceFormat. Removed in v1.0.
    SourceFormat  SourceFormat `json:"-"`
    // ...
}
```

Stamped at every load site:

- `internal/config/yaml_bridge.go::ToDeps` → `SourceFormatYAML`
- `internal/config/auto_detect.go::AutoDetect` → `SourceFormatYAML`
- `internal/config/deps.go::LoadDeps` → `SourceFormatLegacyJSON`
- `internal/testing/helpers.go::CreateMinimalTestDeps` → `SourceFormatLegacyJSON`
- `internal/app/initcase/config.go::BuildDepsConfig` → `SourceFormatLegacyJSON`
- `internal/production/migrate.go` → `SourceFormatLegacyJSON`

Preserved in every clone (per ADR-006):

- `internal/config/deps.go::FilterByProfile` / `FilterByProfiles`
- `internal/config/filter.go::FilterByFeatureFlags`
- `internal/config/ignore_filter.go::FilterIgnoredServices`
- `internal/app/upcase/workspace_project_conflict.go::merge`

### Reader migration policy

**New code MUST read `SourceFormat`.** Adding a new
`SchemaVersion == "2.0"` site is a regression and will be caught
by the issue 069 ratchet.

**Existing readers** (`internal/app/upcase/detect.go::isYAMLMode`,
seven inline call sites in `down.go`, `envshow.go`, `yaml_mode.go`,
etc.) migrate opportunistically as they pass through `SelectFlow`
(issue 069). `isYAMLMode` migrates in this ADR because it is the
canonical one-line helper; the inline readers stay on
`SchemaVersion` until 069 collapses the dual-flow.

`SchemaVersion` is marked deprecated in the struct comment with the
removal version (`Removed in v1.0`).

### Why not delete `SchemaVersion` now

The seven inline readers would all have to migrate in one PR. That
is exactly the big-bang refactor issue 070's "No-go" section
flags. The two-field overlap is bounded (one minor release),
mechanically safe (compiler catches every reader if/when
SchemaVersion is removed), and isolated to one struct.

## Consequences

### Positive

- Source format and schema version are no longer overloaded. A
  future ADR-031 escalation to schema `"2"` won't accidentally
  collide with the `"2.0"` discriminator.
- New readers (e.g. `SelectFlow` from issue 069) have a clean
  field name and a typed enum — no string-magic comparison.
- Stamping is centralized: every loader and every clone is
  responsible for a single field. ADR-006 already covers cloning
  hygiene; this ADR enumerates the stamp sites for grep-discovery.

### Negative

- Brief field overlap: both `SchemaVersion` and `SourceFormat`
  travel together until v1.0. Mitigation: every clone site now
  copies both; the canonical reader (`isYAMLMode`) reads only
  `SourceFormat`.
- `SchemaVersion` still gets read by seven legacy sites until
  issue 069 lands. They keep working because the bridge still
  stamps both fields consistently.

### Neutral

- `cli.SchemaVersion = "1.0"` (the value `raioz version` prints)
  is unrelated to either dimension — it is the *raioz binary*'s
  supported schema literal. Renaming that constant is out of
  scope for this ADR.

## Alternatives considered

- **Delete `SchemaVersion` in the same PR.** Rejected: forces all
  seven inline readers to migrate at once, which is the exact
  big-bang the issue calls out as "No-go". The ratchet in issue
  069 lets readers migrate one-by-one.
- **Encode the source format as a bool (`IsYAML`).** Rejected: a
  third source format (e.g. a future on-the-fly config) would
  force a struct migration. The string-typed enum is one field
  for any number of values.
- **Keep `SchemaVersion` overloaded; document the magic literals
  better.** Rejected: documentation rots. A typed enum survives.

## References

- Code:
  `internal/domain/models/config_deps.go::SourceFormat`,
  `internal/config/deps.go::SourceFormat` (re-export),
  `internal/config/yaml_bridge.go::ToDeps`,
  `internal/config/auto_detect.go::AutoDetect`,
  `internal/app/upcase/detect.go::isYAMLMode`.
- Tests: `internal/config/deps_source_format_test.go`.
- Related: ADR-006 (clone hygiene), ADR-009 (domain/models scope),
  ADR-031 (yaml `version:` escalation — the orthogonal axis),
  ADR-038 (`.raioz.json` removal timeline — the deadline for
  deleting `SchemaVersion`).
- Issues: 070 (this ADR), 069 (`isYAMLMode` consolidation —
  unblocked by SourceFormat).
