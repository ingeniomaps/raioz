# ADR-045: Schema evolution — deprecation and removal markers

- **Status:** Accepted
- **Date:** 2026-05-16

## Context

`raioz.yaml` fields carry a `// since: vX.Y.Z` marker that
`make check-since` enforces (every public field declares the
release that introduced it). This covers the addition axis — new
fields can never ship without a discoverable introduction date.

The removal / rename axis is empty. When a field needs to change
semantics (rename `siblingProject` to `sibling.project`), get
deprecated (a v0.6 experiment that didn't pan out), or vanish at
v1.0, there is:

- No machinery to warn at-load when a user's yaml still uses the
  old name.
- No machinery to hard-error past a deprecation window.
- No auto-translate during load.
- No CI ratchet ensuring deprecation always precedes removal.

ADR-031 promised v1.0 hard-errors on past `version:` mismatches.
ADR-038 / ADR-039 cover the legacy JSON loader timeline. But
NEITHER applies field-level. The next rename or removal would
either: (a) break users silently, (b) emit ad-hoc warnings whose
text drifts release-to-release, or (c) get blocked indefinitely
on "we'll do it at v1.0 maybe". Issue 039.

## Decision

Extend the `FieldMeta` parser (`internal/config/schema_meta.go`)
to recognize three additional markers alongside the existing
`since:`. All optional; all independent. None mutually exclusive
with each other.

```go
// User-facing comment forms recognized on yaml-tagged struct fields:
//   since:       vX.Y.Z   — introducing version (existing, required)
//   deprecated:  vX.Y.Z   — emits a load-time warning (new)
//   removed:     vX.Y.Z   — emits a load-time hard error (new)
//   replacement: <name>   — suggested target field/feature (new, optional)
```

Loader contract:

1. **Warn on deprecated.** When a user's parsed yaml carries a
   field whose `Deprecated` marker is set AND the current raioz
   version `>=` that marker, the loader emits a one-shot
   warning naming the field, the version it was deprecated,
   and the `Replacement` if declared. Once-per-process dedup
   (`sync.Once`) mirrors the JSON loader pattern (ADR-038).
2. **Hard-error on removed.** When the field is present AND
   `Removed` `<=` current version, `LoadYAML` returns a typed
   error before validation continues. The error names the
   removal version + replacement. No override; users must edit
   the yaml.
3. **Pre-deprecation removal is a CI failure.** The
   `make check-since` ratchet (renamed conceptually to
   "schema markers") will gain an assertion that any field
   with a `Removed:` marker also has a prior `Deprecated:`
   marker. Skipping the deprecation window is forbidden — the
   user never sees a removal without first getting a warning.
4. **Loader integration ships in v0.9.** The marker storage
   (`FieldMeta.Deprecated/Removed/Replacement`) lands in v0.9;
   the load-time enforcement uses the storage immediately when
   the first real deprecation lands. Until then the markers
   are read but unused — exposed via `ExtractFieldMeta` for
   future tooling (`raioz yaml lint`).

## Consequences

### Positive

- Renames and removals get a documented path. v1.0 design
  doesn't have to improvise.
- The two-version deprecation window becomes structural, not
  a per-PR judgement call.
- `raioz yaml lint` can report deprecations without bespoke
  field lookups — same source-of-truth as `since:`.

### Negative

- More marker bookkeeping per field. Discipline depends on the
  ratchet catching missing markers, same trade-off as ADR-027
  (i18n source discipline).
- Marker parsing complexity grows linearly with marker count.
  Current four regexes is well within parser cost budget.

### Neutral

- The actual emission machinery is deferred until first
  deprecation. Markers in source are inert until then; the
  next contributor wiring them up has all the data they need.

## Alternatives considered

- **Use yaml-tag struct option (e.g. `yaml:"foo,deprecated"`).**
  Rejected — overloads the yaml tag, breaks Go convention
  (the tag is the schema target, not metadata). Comment markers
  match the existing `since:` shape.
- **External JSON manifest mapping field → markers.** Rejected
  for the same reason `since:` lives next to the field: drift
  is easier with a sidecar file than with a comment 0-3 lines
  away from the field.
- **Defer all of this until v1.0.** Rejected — the cost of
  improvising under release pressure is high; the cost of
  landing the markers now is low.

## References

- Code: `internal/config/schema_meta.go::extractMarkers` (parser),
  `internal/config/schema_meta.go::FieldMeta` (storage).
- Related: ADR-031 (version warning gate), ADR-038 (raioz.json
  deprecation timeline — same pattern at the format level),
  ADR-039 (SourceFormat semantics — drives the dual-flow
  cleanup whose drain bundles with v0.9 schema changes).
- Issue: 039.
