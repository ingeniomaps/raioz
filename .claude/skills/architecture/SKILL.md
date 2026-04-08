# Skill: architecture

## Description

Validate and guide Clean Architecture compliance in
Raioz. Ensures dependencies flow inward and layers
remain decoupled.

## When to use

Run `/architecture` when:
- Adding a new command, use case, or adapter
- Moving code between layers
- Unsure where new code should live
- Reviewing imports for layer violations

## Layer map

```
cmd/raioz/main.go          Entry point (8 lines)
         |
internal/cli/               Cobra command wrappers
         |                   - Creates Dependencies
         |                   - Calls use case Execute()
         |                   - Sets flags, parses args
         |                   - i18n.T() for descriptions
         |
internal/app/               Use cases (business logic)
         |                   - Options struct + Execute()
         |                   - Dependencies via DI struct
         |                   - ONLY uses domain interfaces
         |                   - Never imports infra/
         |
internal/domain/            Contracts and models
  interfaces/               - Port interfaces (9 total)
  models/                   - Type aliases to decouple
         |
internal/infra/             Adapters
  docker_adapter.go         - Implement domain interfaces
  git_adapter.go            - Delegate to concrete packages
  ...                       |
                            |
internal/config/            Concrete packages
internal/docker/            - Actual implementations
internal/env/               - Can import each other
internal/git/               - Cannot import app/ or domain/
internal/state/
internal/workspace/
internal/errors/
internal/i18n/
internal/mocks/
```

## Allowed import rules

| From | Can import | Cannot import |
|------|-----------|---------------|
| `cmd/raioz/` | `internal/cli/` | anything else |
| `internal/cli/` | `internal/app/`, `internal/domain/`, `internal/i18n/`, `internal/errors/` | concrete packages directly |
| `internal/app/` | `internal/domain/`, `internal/i18n/`, `internal/errors/` | `internal/config/`, `internal/docker/`, `internal/git/`, `internal/state/`, `internal/workspace/`, `internal/infra/` |
| `internal/domain/` | `internal/domain/` only | everything else |
| `internal/infra/` | `internal/domain/`, concrete packages | `internal/app/`, `internal/cli/` |
| concrete packages | stdlib, each other, `internal/i18n/`, `internal/errors/` | `internal/app/`, `internal/cli/`, `internal/domain/interfaces/` |

**Exception:** `internal/app/` may import concrete
packages for **type references only** via
`internal/domain/models/` re-exports.

## How to add new components

### New CLI command

1. Create `internal/cli/{command}.go`
2. Register in `internal/cli/root.go`
3. Add i18n keys: `cmd.{command}.short`, `cmd.{command}.long`
4. Add translations to both en.json and es.json
5. Wire: create Dependencies -> create use case -> Execute()

```go
// internal/cli/mycommand.go
package cli

func newMyCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycommand",
        Short: i18n.T("cmd.mycommand.short"),
        RunE: func(cmd *cobra.Command, args []string) error {
            deps := app.NewDependencies()
            uc := app.NewMyCommandUseCase(deps)
            return uc.Execute(cmd.Context(), opts)
        },
    }
    return cmd
}
```

### New use case

1. Create `internal/app/{usecase}.go`
2. Define `Options` struct and `Execute()` method
3. Accept `*Dependencies` via constructor
4. Use only domain interfaces for external operations

```go
// internal/app/myusecase.go
package app

type MyOptions struct {
    Name string
}

type MyUseCase struct {
    deps *Dependencies
}

func NewMyUseCase(deps *Dependencies) *MyUseCase {
    return &MyUseCase{deps: deps}
}

func (uc *MyUseCase) Execute(ctx context.Context, opts MyOptions) error {
    // Use uc.deps.Docker, uc.deps.Git, etc.
    return nil
}
```

### New domain interface

1. Add interface in `internal/domain/interfaces/`
2. Add type alias in `internal/domain/models/` if needed
3. Add to `Dependencies` struct in `internal/app/dependencies.go`
4. Create adapter in `internal/infra/`
5. Create mock in `internal/mocks/`
6. Wire in `NewDependencies()`

### New adapter (infra)

1. Create `internal/infra/{name}_adapter.go`
2. Implement the domain interface
3. Delegate to concrete package
4. Wire in `NewDependencies()`

## Validation checklist

When reviewing architecture compliance, check:

1. **Import direction** — no upward imports
2. **DI usage** — no inline dependency creation in app/
3. **Interface compliance** — app/ uses interfaces, not concrete types
4. **Model decoupling** — domain types via models/, not direct config/ imports
5. **i18n in user-facing code** — all strings through i18n.T()
6. **Error structure** — structured errors with codes in app/cli layers
7. **File size** — max 400 lines (500 for tests)
8. **Single responsibility** — one purpose per file

## Quick validation command

```bash
# Find app/ importing concrete packages (violations)
grep -rn '"raioz/internal/\(config\|docker\|git\|state\|workspace\|env\)' \
  internal/app/ --include='*.go' | grep -v '_test.go' | grep -v 'domain/models'
```

## Rules

- Never bypass layers — cli/ must not call concrete packages directly
- Never add business logic to cli/ — that belongs in app/
- Never add infrastructure concerns to app/ — use interfaces
- domain/ is the innermost layer — it depends on nothing external
- When in doubt, follow the existing pattern in the codebase
