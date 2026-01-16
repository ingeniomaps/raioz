# Migración de Context para Timeouts (Punto 49)

Este documento describe el progreso de la migración de funciones para usar `context.Context` con timeouts.

## Estado Actual

- **Funciones completadas (Git)**:
  - ✅ `git/branch.go` - GetCurrentBranch, CheckoutBranch, PullBranch, EnsureBranch, DetectBranchDrift, UpdateReposIfBranchChanged
  - ✅ `git/remote.go` - BranchExists, ValidateBranch, HasUncommittedChanges, HasMergeConflicts, ForceReclone
  - ✅ `git/version.go` - GetCommitSHA, GetCommitDate
  - ✅ `git/clone.go` - ForceReclone (actualizado para usar context)
  - ✅ `git/readonly.go` - DetectBranchDrift, ValidateBranch, EnsureBranch (actualizado para usar context)
  - ✅ `docker/inspect.go` - GetCommitSHA, GetCommitDate (actualizado para usar context)
  - ✅ `state/check.go` - GetCurrentBranch (actualizado para usar context)
  - ✅ `cmd/up.go` - UpdateReposIfBranchChanged (actualizado para usar context)

- **Funciones pendientes (Docker)**: ~50+ ocurrencias
  - ⏸️ `docker/images.go` - EnsureImage, GetImageInfo
  - ⏸️ `docker/network.go` - EnsureNetwork, GetNetworkProjects
  - ⏸️ `docker/volumes.go` - GetVolumeProjects
  - ⏸️ `docker/status.go` - GetServiceNames
  - ⏸️ `docker/clean.go` - CleanProject
  - ⏸️ `docker/logs.go` - ViewLogs, GetAvailableServices
  - ⏸️ `docker/inspect.go` - GetContainerName, getResourceUsage (funciones internas)
  - ⏸️ `validate/preflight.go` - PreflightCheck

## Patrón de Migración

### Antes:
```go
cmd := exec.Command("docker", "compose", "up", "-d")
if err := cmd.Run(); err != nil {
    return fmt.Errorf("failed: %w", err)
}
```

### Después:
```go
ctx, cancel := exectimeout.WithTimeout(exectimeout.DockerComposeUpTimeout)
defer cancel()

cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
if err := cmd.Run(); err != nil {
    if exectimeout.IsTimeoutError(ctx, err) {
        return exectimeout.HandleTimeoutError(ctx, err, "docker compose up", exectimeout.DockerComposeUpTimeout)
    }
    return fmt.Errorf("failed: %w", err)
}
```

## Timeouts Predefinidos

- Git clone: 10 minutos
- Git pull: 5 minutos
- Git checkout: 2 minutos
- Docker compose up: 5 minutos
- Docker compose down: 2 minutos
- Docker pull: 15 minutos
- Docker inspect/status: 30 segundos
- Default: 5 minutos

## Notas

- Las funciones principales (docker compose, git clone) ya usan context
- Las funciones de git (branch, remote, version) han sido migradas
- Las funciones de docker (images, network, volumes, etc.) están pendientes
- La migración es gradual y pragmática
