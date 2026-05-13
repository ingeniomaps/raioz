# raioz.yaml corpus

Locked fixtures for the public `raioz.yaml` schema. Every file in this
directory is loaded by `TestConfigCorpus` (see `../../corpus_test.go`)
through `LoadYAML` — the same code path real users hit.

## Why this exists

Every field documented in `docs/CONFIG_REFERENCE.md` must round-trip
through the loader. Without a corpus, a refactor that breaks a
polymorphic shape (e.g. `network:` as a string vs object) only
surfaces when a user files a bug. The corpus turns that into a CI
failure.

The companion script `scripts/check-config-fixtures.sh` fails CI when
`internal/config/yaml_types.go` or `internal/config/yaml_aux_types.go`
changes without a matching fixture diff. That keeps the corpus from
silently rotting behind a growing schema.

## Conventions

1. **One concern per fixture.** Each file isolates ONE combination
   (e.g. `proxy:` as bool, `proxy:` as object, sibling mode A, sibling
   mode B). Combining unrelated cases hides which one broke when a
   test fails.
2. **Header comment is mandatory.** Lead with the filename, then 1-3
   lines stating what combination it covers and why future-you should
   not delete it. The comment is part of the contract.
3. **Numeric prefix is just for ordering.** No semantic meaning — it
   keeps the directory listing aligned with reading order
   (minimal → full → individual combinations).
4. **Fixtures must parse cleanly.** A "this should fail" case belongs
   in a regular `_test.go`, not here.
5. **Paths can be fictional.** `LoadYAML` does not stat the filesystem
   — it only validates the YAML shape. Use `./api`, `./frontend`,
   etc., without creating the directories.

## When to add a fixture

Add a new fixture when you:

- Add a public field to the schema. Pair the field addition with a
  fixture that exercises it.
- Add a polymorphic shape (a field that accepts multiple YAML shapes).
  One fixture per shape.
- Add a cross-field interaction with non-obvious semantics (e.g.,
  `network.subnet` enabling `proxy.publish: false`).

You do NOT need to add a fixture when you:

- Rename an internal helper that doesn't change the user-visible shape.
- Fix an off-by-one in `validateYAMLConfig` (the existing fixtures
  already exercise the path).
- Add a private field (`json:"-"` / `yaml:"-"`).

## Running locally

```
make check-configs
```

This runs `TestConfigCorpus` plus the schema-diff guard.

## Charter

The corpus must contain at least 15 fixtures. The minimum is enforced
in `corpus_test.go` so a casual cleanup doesn't accidentally drop
coverage of an unusual combination. See issue 027 for the original
plan.
