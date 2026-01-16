# Migración de Logging Estructurado (Punto 48)

Este documento describe el progreso de la migración de `fmt.Printf` a `log/slog` estructurado.

## Estado Actual

- **Total de ocurrencias**: 146+
- **Archivos completados**:
  - `cmd/up.go` ✅ (3 ocurrencias migradas)
  - `cmd/down.go` ✅ (7 ocurrencias migradas)
- **Archivos pendientes**: 21 archivos

## Estrategia de Migración

1. **Warnings/Errors claros** → `logging.Warn()` / `logging.Error()`
2. **Info al usuario** → `output.PrintInfo()` (mantener formato)
3. **Success al usuario** → `output.PrintSuccess()` (mantener formato)
4. **Logs de debugging** → `logging.Debug()` / `logging.Info()`

## Patrón de Migración

### Antes:
```go
fmt.Printf("⚠️  Warning: failed to get service info: %v\n", err)
```

### Después:
```go
logging.Warn("failed to get service info", "error", err)
```

### Output al usuario (mantener formato):
```go
// Antes
fmt.Printf("✔ Project '%s' stopped successfully\n", projectName)

// Después
output.PrintSuccess(fmt.Sprintf("Project '%s' stopped successfully", projectName))
```

## Archivos Pendientes (orden sugerido)

1. `cmd/clean.go` - 13 ocurrencias
2. `cmd/status.go` - 10 ocurrencias
3. `cmd/override.go` - 9 ocurrencias
4. `cmd/list.go` - 9 ocurrencias
5. `cmd/workspace.go` - 8 ocurrencias
6. `cmd/version.go` - 5 ocurrencias
7. `cmd/ports.go` - 3 ocurrencias
8. `cmd/link.go` - 3 ocurrencias
9. `cmd/ignore.go` - 3 ocurrencias
10. `cmd/check.go` - 3 ocurrencias
11. `cmd/dependency_assist.go` - 41 ocurrencias (interactivo, más complejo)
12. `internal/output/format.go` - 15 ocurrencias (output al usuario, mantener formato)
13. `internal/` - 27 ocurrencias

## Notas

- La migración es gradual y pragmática
- Output al usuario (`internal/output`) mantiene formato especial (emojis, etc.)
- Logs estructurados usan `log/slog` con campos estructurados
- Sanitización de secrets se integra cuando se logean variables de entorno
