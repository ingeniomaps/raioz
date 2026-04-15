# Skill: i18n

## Description

Manage internationalization keys, translations, and
catalog consistency for Raioz.

## When to use

Run `/i18n` when:
- Adding new user-facing strings
- Checking if a key already exists
- Adding keys to both catalogs (en.json, es.json)
- Debugging missing translations

## Catalogs

Location: `internal/i18n/locales/`
- `en.json` — English (primary, 700+ keys)
- `es.json` — Spanish (must mirror en.json exactly)

## Key naming conventions

Keys use dot-separated namespaces:

| Prefix | Usage | Example |
|--------|-------|---------|
| `cmd.*` | Cobra command descriptions | `cmd.up.short` |
| `up.*` | raioz up flow messages | `up.cloning_repo` |
| `error.*` | Error messages | `error.config_not_found` |
| `error.*_suggestion` | Error suggestions | `error.config_not_found_suggestion` |
| `check.*` | raioz check messages | `check.drift_detected` |
| `init.*` | raioz init wizard | `init.prompt_name` |
| `status.*` | raioz status output | `status.running` |
| `output.*` | General output messages | `output.done` |
| `prompt.*` | Interactive prompts | `prompt.confirm_override` |

Rules:
- Always lowercase
- Use underscores within segments: `error.port_conflict`
- Imperative mood for actions: `up.starting_services`
- Past participle for states: `status.stopped`
- Suffix `_suggestion` for error fix hints

## How to add a new key

### Step 1: Choose the key name

Follow the naming conventions above. Check if a similar
key already exists:

```bash
grep -i "keyword" internal/i18n/locales/en.json
```

### Step 2: Add to en.json

```json
"error.new_thing": "The new thing failed: %s"
```

Use `%s`, `%d`, `%v` for interpolation (Go fmt verbs).

### Step 3: Add to es.json

```json
"error.new_thing": "La cosa nueva fallo: %s"
```

Both catalogs MUST have identical keys. Different keys
will fail `make check-i18n`.

### Step 4: Use in code

```go
// Simple
msg := i18n.T("error.new_thing")

// With interpolation
msg := i18n.T("error.new_thing", serviceName)
```

### Step 5: Verify

```bash
make check-i18n
```

## Common operations

### Find all keys matching a pattern

```bash
grep '"error\.' internal/i18n/locales/en.json
```

### Find unused keys

```bash
# Extract all keys from en.json
grep -oP '"([^"]+)":' internal/i18n/locales/en.json | tr -d '":' | while read key; do
  if ! grep -rq "\"$key\"" internal/ cmd/; then
    echo "UNUSED: $key"
  fi
done
```

### Find hardcoded strings (should use i18n.T)

```bash
grep -rn 'fmt\.Print' internal/app/ internal/cli/ cmd/ --include='*.go' | grep -v '_test.go' | grep -v 'i18n.T'
```

### Check catalog sync

```bash
make check-i18n
```

This runs `TestCatalogCompleteness` which verifies both
catalogs have the same keys.

## Rules

- NEVER hardcode user-facing strings — always use `i18n.T()`
- ALWAYS add to both en.json AND es.json simultaneously
- Error keys MUST have a matching `_suggestion` key
- Keep translations concise — terminal output, not prose
- Interpolation args must match between languages
- `slog` log messages are internal-only and NOT translated
- Cobra command descriptions use `i18n.T()` inline in each command file
