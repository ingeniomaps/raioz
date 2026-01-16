# Domain Layer

This package contains domain models and interfaces following Clean Architecture principles.

## Structure

```
internal/domain/
├── interfaces/    # Interfaces (ports) - define contracts
│   ├── docker.go
│   ├── git.go
│   ├── workspace.go
│   ├── filesystem.go
│   └── exec.go
└── models/        # Domain models (future)
    ├── config.go  # Move from internal/config
    └── workspace.go # Move from internal/workspace
```

## Principles

- **Domain is independent**: Domain layer doesn't depend on any other layer
- **Interfaces define contracts**: All external dependencies are defined as interfaces
- **Domain models are pure**: No infrastructure concerns, just business logic

## Current State

Currently, only `interfaces/` is implemented. The domain models (`config`, `workspace`) are still in their original locations (`internal/config/`, `internal/workspace/`).

## Future Migration

Models will be gradually moved from their current locations:
- `internal/config/` → `internal/domain/models/config/`
- `internal/workspace/` (models) → `internal/domain/models/workspace/`

This migration should be done carefully to avoid breaking changes.
