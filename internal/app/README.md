# Application Layer

This package contains use cases (application services) following Clean Architecture principles.

## Structure

```
internal/app/
├── up.go          # UpUseCase - Start project
├── down.go        # DownUseCase - Stop project
├── status.go      # StatusUseCase - Get project status
└── ...
```

## Principles

- **Use cases contain orchestration logic**: They coordinate between domain interfaces
- **Use cases depend on interfaces, not implementations**: All dependencies are injected via interfaces
- **Use cases are testable**: They can be tested with mocks of interfaces
- **Use cases are independent of presentation**: They don't know about CLI, API, etc.

## Example

```go
package app

import (
    "context"
    "raioz/internal/domain/interfaces"
)

type UpUseCase struct {
    dockerRunner interfaces.DockerRunner
    gitRepo      interfaces.GitRepository
    workspace    interfaces.WorkspaceManager
    // ... other dependencies
}

func NewUpUseCase(
    dockerRunner interfaces.DockerRunner,
    gitRepo interfaces.GitRepository,
    workspace interfaces.WorkspaceManager,
) *UpUseCase {
    return &UpUseCase{
        dockerRunner: dockerRunner,
        gitRepo:      gitRepo,
        workspace:    workspace,
    }
}

func (uc *UpUseCase) Execute(ctx context.Context, opts UpOptions) error {
    // Orchestration logic here
    // 1. Resolve workspace
    // 2. Clone repositories
    // 3. Start Docker Compose
    // etc.
}
```

## Migration Strategy

1. Create use cases in `internal/app/`
2. Gradually move logic from `cmd/` to use cases
3. Update `cmd/` to call use cases
4. Add tests for use cases with mocks
