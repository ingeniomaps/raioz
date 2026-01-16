# Análisis de Archivos Cobra en cmd/

Fecha: 2026-01-14

## Resumen Ejecutivo

Se analizaron todos los archivos Cobra en `cmd/` según la definición establecida:

**Definición de archivo Cobra:**

- Debe tener entre 50-100 líneas (idealmente < 150)
- NO debe importar paquetes de infraestructura/dominio
- NO debe contener lógica de negocio
- Solo debe importar: `app`, `errors`, `logging`, `cobra`
- Solo debe llamar a UseCases

## Archivos que ya cumplen las reglas ✅

1. **check.go** (55 líneas) - ✅ Usa `CheckUseCase`
2. **down.go** (61 líneas) - ✅ Usa `DownUseCase`
3. **status.go** (57 líneas) - ✅ Usa `StatusUseCase`
4. **up.go** (68 líneas) - ✅ Usa `UpUseCase` (recientemente migrado)
5. **root.go** (65 líneas) - ✅ Comando raíz (especial, aceptable)
6. **version.go** (44 líneas) - ✅ Comando simple, solo muestra versión

## Archivos que violan las reglas ❌

### 1. list.go (185 líneas)

**Violaciones:**

- Importa: `state`, `output`
- Contiene lógica de negocio (filtrado, formateo)
- Más de 100 líneas

**Acción requerida:**

- Crear `ListUseCase` en `internal/app/list.go`
- Migrar lógica de filtrado y formateo
- Simplificar `cmd/list.go` a solo llamar al UseCase

---

### 2. clean.go (178 líneas)

**Violaciones:**

- Importa: `config`, `docker`, `workspace`, `logging`, `output`
- Contiene lógica de negocio compleja
- Más de 100 líneas

**Acción requerida:**

- Crear `CleanUseCase` en `internal/app/clean.go`
- Migrar toda la lógica de limpieza
- Simplificar `cmd/clean.go` a solo llamar al UseCase

---

### 3. compare.go (125 líneas)

**Violaciones:**

- Importa: `config`, `production`, `output`
- Contiene lógica de negocio
- Más de 100 líneas

**Acción requerida:**

- Crear `CompareUseCase` en `internal/app/compare.go`
- Migrar lógica de comparación
- Simplificar `cmd/compare.go` a solo llamar al UseCase

---

### 4. ignore.go (167 líneas)

**Violaciones:**

- Importa: `config`, `ignore`, `output`
- Contiene lógica de negocio
- Más de 100 líneas

**Acción requerida:**

- Crear `IgnoreUseCase` en `internal/app/ignore.go`
- Migrar lógica de gestión de ignore list
- Simplificar `cmd/ignore.go` a solo llamar al UseCase

---

### 5. link.go (244 líneas)

**Violaciones:**

- Importa: `config`, `link`, `workspace`, `output`
- Contiene lógica de negocio compleja
- Muy grande (244 líneas)

**Acción requerida:**

- Crear `LinkUseCase` en `internal/app/link.go`
- Migrar toda la lógica de symlinks
- Simplificar `cmd/link.go` a solo llamar al UseCase

---

### 6. logs.go (109 líneas)

**Violaciones:**

- Importa: `config`, `docker`, `state`, `workspace`
- Contiene lógica de negocio
- Más de 100 líneas

**Acción requerida:**

- Crear `LogsUseCase` en `internal/app/logs.go`
- Migrar lógica de visualización de logs
- Simplificar `cmd/logs.go` a solo llamar al UseCase

---

### 7. migrate.go (143 líneas)

**Violaciones:**

- Importa: `production`, `output`, `errors`
- Contiene lógica de negocio
- Más de 100 líneas

**Acción requerida:**

- Crear `MigrateUseCase` en `internal/app/migrate.go`
- Migrar lógica de migración
- Simplificar `cmd/migrate.go` a solo llamar al UseCase

---

### 8. override.go (211 líneas)

**Violaciones:**

- Importa: `config`, `override`, `audit`, `output`
- Contiene lógica de negocio compleja
- Muy grande (211 líneas)

**Acción requerida:**

- Crear `OverrideUseCase` en `internal/app/override.go`
- Migrar toda la lógica de overrides
- Simplificar `cmd/override.go` a solo llamar al UseCase

---

### 9. ports.go (67 líneas)

**Violaciones:**

- Importa: `docker`, `workspace`, `output`
- Contiene lógica de negocio

**Acción requerida:**

- Crear `PortsUseCase` en `internal/app/ports.go`
- Migrar lógica de listado de puertos
- Simplificar `cmd/ports.go` a solo llamar al UseCase

---

### 10. workspace.go (158 líneas)

**Violaciones:**

- Importa: `workspace`, `root`, `audit`, `output`
- Contiene lógica de negocio
- Más de 100 líneas

**Acción requerida:**

- Crear `WorkspaceUseCase` en `internal/app/workspace.go`
- Migrar lógica de gestión de workspaces
- Simplificar `cmd/workspace.go` a solo llamar al UseCase

---

### 11. ci.go (43 líneas) + ci\_\*.go

**Violaciones:**

- `ci.go` está bien estructurado pero llama a `executeCICommand()` en `ci_execute.go`
- Los archivos `ci_*.go` no son archivos Cobra, son helpers

**Acción requerida:**

- Crear `CiUseCase` en `internal/app/ci.go`
- Mover `ci_execute.go`, `ci_helpers.go`, `ci_validations.go`, `ci_types.go` a `internal/app/` o integrarlos en el UseCase
- Simplificar `cmd/ci.go` a solo llamar al UseCase

---

## Archivos helper eliminados ✅

Los siguientes archivos fueron eliminados porque ya no se usan (migrados a `UpUseCase`):

- `up_bootstrap.go`
- `up_compose.go`
- `up_docker_prepare.go`
- `up_filters.go`
- `up_format.go`
- `up_git.go`
- `up_state.go`
- `up_validation.go`

## Archivos helper que aún existen

- `ci_execute.go`, `ci_helpers.go`, `ci_validations.go`, `ci_types.go` - Deben migrarse a `internal/app/`
- `dependency_assist.go` - Ya existe en `internal/app/`, debe eliminarse de `cmd/`

## Plan de Acción Recomendado

### Fase 1: Limpieza inmediata

- [x] Eliminar archivos `up_*.go` de `cmd/` (ya no se usan)
- [ ] Eliminar `cmd/dependency_assist.go` (ya existe en `internal/app/`)

### Fase 2: Migraciones prioritarias (comandos más usados)

- [ ] Migrar `list.go` → `ListUseCase`
- [ ] Migrar `logs.go` → `LogsUseCase`
- [ ] Migrar `ports.go` → `PortsUseCase`

### Fase 3: Migraciones intermedias

- [ ] Migrar `clean.go` → `CleanUseCase`
- [ ] Migrar `ignore.go` → `IgnoreUseCase`
- [ ] Migrar `workspace.go` → `WorkspaceUseCase`

### Fase 4: Migraciones avanzadas

- [ ] Migrar `link.go` → `LinkUseCase`
- [ ] Migrar `override.go` → `OverrideUseCase`
- [ ] Migrar `compare.go` → `CompareUseCase`
- [ ] Migrar `migrate.go` → `MigrateUseCase`
- [ ] Migrar `ci.go` + `ci_*.go` → `CiUseCase`

## Métricas

- **Total de archivos Cobra analizados:** 17
- **Archivos que cumplen las reglas:** 6 (35%)
- **Archivos que violan las reglas:** 11 (65%)
- **Archivos helper eliminados:** 8
