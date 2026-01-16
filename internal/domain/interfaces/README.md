# Domain Interfaces

This package defines the interfaces (ports) for dependency injection following Clean Architecture principles.

## Interfaces

### DockerRunner
Defines operations for running Docker Compose commands.

### GitRepository
Defines operations for Git repository management.

### WorkspaceManager
Defines operations for workspace management.

### FileSystem
Defines operations for file system access.

### CommandExecutor
Defines operations for executing external commands.

## Usage

These interfaces should be used in the application layer (`internal/app/`) and domain layer. Implementations are provided in `internal/infra/`.

## Example

```go
import (
    "raioz/internal/domain/interfaces"
    "raioz/internal/infra/docker"
)

// In application layer
type UpUseCase struct {
    dockerRunner interfaces.DockerRunner
}

func NewUpUseCase() *UpUseCase {
    return &UpUseCase{
        dockerRunner: docker.NewDockerRunner(),
    }
}

func (u *UpUseCase) Execute(composePath string) error {
    return u.dockerRunner.Up(composePath)
}
```

## Migration Strategy

1. Start using interfaces in new code
2. Gradually refactor existing code to accept interfaces
3. Use dependency injection in constructors
4. Update tests to use mocks of interfaces
