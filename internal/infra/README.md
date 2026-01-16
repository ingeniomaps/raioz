# Infrastructure Layer

This package contains concrete implementations of domain interfaces.

## Structure

```
internal/infra/
├── docker/        # Docker implementation
│   └── runner_impl.go
├── git/           # Git implementation
│   └── repository_impl.go
├── workspace/     # Workspace implementation
│   └── manager_impl.go
├── filesystem/    # FileSystem implementation
│   └── os_impl.go
└── exec/          # CommandExecutor implementation
    └── os_impl.go
```

## Principles

- **Implements domain interfaces**: All implementations satisfy interfaces defined in `internal/domain/interfaces/`
- **Concrete implementations**: Uses real external dependencies (os/exec, os.File, etc.)
- **Testable with mocks**: Can be replaced with mocks in tests

## Current State

Currently, implementations are wrappers around existing code in `internal/docker/`, `internal/git/`, etc.

## Future Migration

Full implementations should be moved here:
- `internal/docker/` → `internal/infra/docker/` (complete implementation)
- `internal/git/` → `internal/infra/git/` (complete implementation)
- `internal/workspace/` (infra parts) → `internal/infra/workspace/` (complete implementation)

This is a large refactoring that should be done gradually.
