# Skill: error

## Description

Create structured errors following Raioz conventions.
Ensures consistency in error codes, messages,
suggestions, and i18n compliance.

## When to use

Run `/error` when:
- Creating new error types or codes
- Adding error handling to a new feature
- Unsure about the error pattern to follow

## Error anatomy

```go
errors.New(errors.ErrCodeHere, i18n.T("error.description", args...)).
    WithContext("key", value).
    WithSuggestion(i18n.T("error.description_suggestion"))
```

Every user-facing error has four parts:

| Part | Required | Source |
|------|----------|--------|
| Code | Yes | Constant in `internal/errors/` |
| Message | Yes | `i18n.T("error.xxx")` |
| Context | Optional | `.WithContext(k, v)` for debugging |
| Suggestion | Yes (user-facing) | `i18n.T("error.xxx_suggestion")` |

## Error codes

Defined in `internal/errors/`. Follow the pattern:

```go
const (
    ErrConfigNotFound    = "CONFIG_NOT_FOUND"
    ErrConfigInvalid     = "CONFIG_INVALID"
    ErrPortConflict      = "PORT_CONFLICT"
    ErrDockerNotRunning  = "DOCKER_NOT_RUNNING"
    // ...
)
```

Naming rules:
- ALL_CAPS with underscores
- Prefix by domain: `CONFIG_`, `DOCKER_`, `GIT_`, `STATE_`, `WORKSPACE_`, `ENV_`, `LOCK_`
- Descriptive: what went wrong, not what to do

## How to add a new error

### Step 1: Define the error code

Add to the appropriate file in `internal/errors/`:

```go
const ErrMyNewThing = "MY_NEW_THING"
```

### Step 2: Add i18n keys

In `internal/i18n/locales/en.json`:
```json
"error.my_new_thing": "Failed to do the thing: %s",
"error.my_new_thing_suggestion": "Check that X is configured correctly"
```

In `internal/i18n/locales/es.json`:
```json
"error.my_new_thing": "Fallo al hacer la cosa: %s",
"error.my_new_thing_suggestion": "Verifica que X este configurado correctamente"
```

### Step 3: Use in code

```go
return errors.New(errors.ErrMyNewThing, i18n.T("error.my_new_thing", detail)).
    WithContext("service", serviceName).
    WithSuggestion(i18n.T("error.my_new_thing_suggestion"))
```

### Step 4: Verify

```bash
make check-i18n
```

## Where to use each error style

| Layer | Error style | Example |
|-------|------------|---------|
| `internal/domain/` | Return plain errors or domain error types | `return fmt.Errorf("invalid: %w", err)` |
| `internal/config/`, `internal/docker/`, etc. | Structured errors with codes | `errors.New(code, msg)` |
| `internal/app/` | Structured errors with suggestion | `errors.New(code, msg).WithSuggestion(...)` |
| `internal/cli/` | Propagate — do not create new errors | `return useCase.Execute(ctx, opts)` |

## Context fields

Use `.WithContext()` to add debugging info:

```go
errors.New(errors.ErrPortConflict, i18n.T("error.port_conflict", port)).
    WithContext("port", port).
    WithContext("service", serviceName).
    WithContext("existing_service", existingService).
    WithSuggestion(i18n.T("error.port_conflict_suggestion"))
```

Common context keys:
- `service` — service name
- `port` — port number
- `path` — file or directory path
- `repo` — git repository URL
- `branch` — git branch
- `config` — config file path

## Writing good messages

### Error message (what went wrong)

- Start with what failed: "Failed to clone repository"
- Include the specific detail: "Failed to clone repository: %s"
- Use present tense or past participle
- Keep under 80 characters

### Suggestion (what to do about it)

- Start with an action verb: "Check that...", "Run...", "Verify..."
- Be specific and actionable
- One concrete step, not a troubleshooting guide
- Keep under 100 characters

### Examples

```
Message:    "Port %d is already in use by %s"
Suggestion: "Change the port in .raioz.json or stop the conflicting service"

Message:    "Docker is not running"
Suggestion: "Start Docker Desktop or run 'systemctl start docker'"

Message:    "Config file not found: %s"
Suggestion: "Run 'raioz init' to create a .raioz.json file"
```

## Error wrapping

When wrapping errors from lower layers, preserve the
original error:

```go
result, err := someOperation()
if err != nil {
    return errors.New(errors.ErrSomething, i18n.T("error.something")).
        WithContext("detail", err.Error()).
        WithSuggestion(i18n.T("error.something_suggestion"))
}
```

Do NOT use `fmt.Errorf` with `%w` in app/ layer — use
structured errors instead.

## Rules

- Every user-facing error MUST have a suggestion
- Every error message MUST go through i18n.T()
- Every error MUST have a defined code constant
- Suggestions MUST be actionable (verb first)
- Both en.json and es.json MUST have the key
- cli/ layer MUST NOT create new errors — only propagate
- Internal logging (slog) is NOT translated
- Do not expose internal details in user-facing messages
