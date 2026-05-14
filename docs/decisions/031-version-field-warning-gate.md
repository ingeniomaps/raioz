# ADR-031: `version:` field becomes a warning-level schema gate

- **Status:** Accepted — implemented 2026-05-14
- **Date:** 2026-05-14

## Context

v0.5.0 introduced the top-level `version:` field in `raioz.yaml`
to let raioz "refuse or migrate instead of silently
misinterpreting your config" (CONFIG_REFERENCE.md). The
implementation only warned when the field was **absent** — any
declared value was accepted without comparison against the
binary's `CurrentSchemaVersion = "1"`.

The quality auditor (issue 054) flagged the consequence:

> Field set to anything else (e.g. `"99"`) → accepts silently.
> No warning. No error.
>
> Entrena al usuario a ignorar el campo. Cuando v2 del schema
> exista, los usuarios que pongan `version: "1"` y un binario v2
> los ignore porque el lockstep no se enforza.

The whole point of versioning is signal-propagation when
assumptions break; "loads silently" trains users to ignore the
field, exactly what makes future schema changes painful.

## Decision

Turn `version:` into a **warning-level gate** in v0.6. Three new
warning paths added; no hard errors yet (hard errors land in
later releases — see "Escalation" below).

### Warning paths

`schemaVersionWarnings` in `internal/config/yaml_bridge.go` now
distinguishes four cases:

1. **Missing version** → soft warning ("consider adding"). Same
   as before.
2. **Newer than current** (e.g. config says `"2"`, binary supports
   `"1"`) → loud warning: "Fields introduced in newer schema
   versions will be ignored. Update raioz."
3. **Older than current** → loud warning: "Field semantics may
   have changed across the bump. Run `raioz migrate yaml`."
4. **Malformed** (`"v1"`, `"1.0"`, `"abc"`, negative integers) →
   loud warning naming the bad value. The config still loads as
   if `version: "1"` was declared — the schema number is an
   integer, the doc says so, and we don't want to fail-closed in
   v0.6.

### Comparison helper

`compareSchemaVersion(declared, current string) (int, bool)`
parses both as non-negative integers. Returns -1/0/+1 like
`strings.Compare` plus a boolean for malformed input. The integer
restriction is deliberate: SemVer-shaped values like `"1.0"` are
rejected — schema versions in raioz are integers (`"1"`, `"2"`,
`"3"`).

### Escalation plan

The warnings are advisory in v0.6. Future releases tighten:

- **v0.7** — past-version configs hard-error and force `raioz
  migrate yaml` before load. Safe to error first because the
  migration path is automated.
- **v1.0** — any mismatch is a hard error.
- Newer-version warnings stay advisory longer than older-version
  ones because the migration path is "update raioz" not "edit
  the file"; we can't fix the binary version mismatch from
  inside the config loader.

Documented in CONFIG_REFERENCE.md.

## Implementation status

Landed in this commit:

- `internal/config/yaml_bridge.go`: extended `schemaVersionWarnings`
  with version comparison + four warning paths; added private
  helper `compareSchemaVersion`.
- `internal/config/yaml_version_test.go`: three new tests
  (`TestSchemaVersionWarnings_Mismatch`,
  `TestSchemaVersionWarnings_CurrentNoWarning`,
  `TestCompareSchemaVersion`) pinning the warning fragments and
  the comparison semantics across 10 input pairs.
- `docs/CONFIG_REFERENCE.md`: updated the "Current behavior"
  table with the four cases and added the escalation plan.

## Consequences

### Positive

- The `version:` field finally signals something. Users
  declaring `"99"` see a warning instead of silent acceptance.
- The escalation plan is published — users see when warnings
  become errors and adjust ahead of time.
- The comparison helper is unit-tested for malformed inputs
  (semver-shaped, alphabetic, negative, whitespace-padded) so
  the parsing rules don't drift.

### Negative

- Configs that worked silently in v0.5.x (with anything other
  than `"1"`) emit warnings in v0.6. Intended signal but
  hand-edited configs see the warning first time. Mitigation:
  `raioz init` and `raioz migrate yaml` write the correct value.
- Warning-level only — a determined user can still ignore the
  warning. The escalation to errors in v0.7+ is the durable fix.

### Neutral

- The warning text uses the field literal `version:` (with the
  colon) so it grep-matches the config syntax users see. Test
  pins the text fragments to prevent silent drift.

## Alternatives considered

- **Hard error immediately on any mismatch.** Rejected: configs
  in v0.5 with typo'd values would break without a pre-warning
  grace. Warning-level in v0.6 gives one minor release for
  repos to catch up.
- **Accept semver-shaped values (`"1.0"`).** Rejected: the
  schema version is per-release integer breaking count. Semver
  is for the raioz binary; the schema number bumps only when a
  field's semantics break.
- **Defer the gate to v0.7 with hard errors.** Rejected: the
  silent-acceptance bug exists now; one release of advisory
  warnings matches ADR-027 (i18n) and ADR-029 (app-infra) which
  used the same ratchet path.

## References

- Code: `internal/config/yaml_bridge.go::schemaVersionWarnings`,
  `internal/config/yaml_bridge.go::compareSchemaVersion`.
- Tests: `internal/config/yaml_version_test.go`.
- Docs: `docs/CONFIG_REFERENCE.md#versioning`.
- Issue: 054.
- Related: ADR-027 (warning-level ratchet pattern), ADR-029
  (warning-level baseline pattern).
