# Skill: review

## Description

Pre-commit code review for Raioz. Validates all project
rules before code leaves the working tree.

## When to use

Run `/review` after finishing a change and before
committing. Also useful mid-task to catch violations
early.

## What to check

Run checks in this exact order. Stop at the first
category with failures and report them — do not dump
everything at once.

### 1. File size (max 400 lines, tests max 500)

```bash
find . -name "*.go" ! -name "*_test.go" ! -path "*/vendor/*" \
  -exec sh -c 'lines=$(wc -l < "$1"); [ "$lines" -gt 400 ] && echo "$1: $lines lines"' _ {} \;
```

If any file exceeds the limit, suggest a concrete split
strategy (by responsibility, by type, or by domain) with
proposed filenames. Reference `.context/REGLA_400_LINEAS.md`.

### 2. Line length (max 120 characters)

```bash
grep -rn '.\{121\}' --include='*.go' --exclude-dir=vendor .
```

Show the offending lines and suggest how to break them.

### 3. Architecture layer violations

Verify import direction follows:
```
cmd/ -> internal/cli/ -> internal/app/ -> internal/domain/
                                       -> internal/infra/ -> (concrete packages)
```

Violations to flag:
- `cmd/` importing anything except `internal/cli/` or `internal/app/`
- `internal/app/` importing concrete packages (docker/, git/, config/ etc.) except for type references via `domain/models/`
- `internal/domain/` importing anything outside domain/
- Any package importing `cmd/`

### 4. i18n compliance

Every user-facing string must go through `i18n.T()`.
Check for:
- `fmt.Println` / `fmt.Printf` / `fmt.Fprintf` with
  hardcoded strings in `cmd/`, `internal/cli/`, `internal/app/`
- Error messages not using `i18n.T()` in user-facing code
- New `i18n.T("key")` calls where the key does not exist
  in `internal/i18n/locales/en.json`

Run `make check-i18n` to verify catalog sync.

### 5. Error pattern compliance

Errors must follow the structured pattern:
```go
errors.New(code, i18n.T("error.xxx")).WithSuggestion(i18n.T("error.xxx_suggestion"))
```

Flag:
- `fmt.Errorf` in app/ or cli/ layers (should use structured errors)
- `errors.New` without i18n.T for the message
- Missing `.WithSuggestion()` on user-facing errors

### 6. Test presence

If new exported functions were added, verify matching
test functions exist. Do not require tests for trivial
getters or one-line wrappers.

### 7. Dependency injection

New use cases in `internal/app/` must:
- Accept dependencies via a `*Dependencies` struct
- Use domain interfaces, not concrete types
- Have an `Options` struct and `Execute()` method

### 8. Naming conventions

- Packages: lowercase, single word
- Exported: PascalCase
- Unexported: camelCase
- Acronyms: all caps (HTTP, API, ID)
- Interfaces: `-er` suffix when possible

## Output format

```
## Review: [pass | X issues found]

### [Category name]
- file:line — description
  Suggestion: ...

### Summary
[One sentence: what to fix before committing]
```

Keep output concise. No praise, no filler. List problems
and concrete fixes.

## Rules

- Only review changed files (use `git diff --name-only`)
- Do not auto-fix — report only
- Do not add comments, docstrings, or type annotations
- Do not suggest refactors beyond the scope of the change
- If everything passes, say "All checks pass" and nothing more
