# Análisis de Arquitectura - Raioz

Análisis completo de la arquitectura del código, estructura de paquetes, dependencias, capas y problemas arquitectónicos.

## 📐 Estructura Actual

### Organización de Directorios

```
raioz/
├── main.go                 # Punto de entrada
├── cmd/                    # Capa de presentación (CLI)
│   ├── root.go            # Configuración de comandos
│   ├── up.go              # Comando up
│   ├── down.go            # Comando down
│   ├── status.go          # Comando status
│   └── ...                # Otros comandos
└── internal/               # Lógica de negocio
    ├── config/            # Dominio: Configuración
    ├── workspace/         # Dominio: Workspace
    ├── git/               # Infraestructura: Git
    ├── docker/            # Infraestructura: Docker
    ├── env/               # Infraestructura: Variables de entorno
    ├── validate/          # Aplicación: Validación
    ├── state/             # Infraestructura: Persistencia
    ├── lock/              # Infraestructura: Locks
    ├── errors/            # Utilidad: Manejo de errores
    ├── output/            # Utilidad: Formato de salida
    ├── mocks/             # Testing: Mocks
    ├── testing/           # Testing: Helpers
    └── production/        # Utilidad: Comparación con producción
```

## ✅ Aspectos Positivos de la Arquitectura

### 1. Separación Básica de Responsabilidades

**Bien implementado**:
- ✅ `cmd/` separado de `internal/` (presentación vs lógica)
- ✅ Paquetes por dominio (config, workspace, git, docker)
- ✅ Utilidades separadas (errors, output)

### 2. Estructura Clara de Paquetes

**Bien organizado**:
- ✅ Cada paquete tiene un propósito claro
- ✅ Nombres descriptivos
- ✅ Agrupación lógica de funcionalidades

### 3. Separación de Dominio e Infraestructura

**Bien separado**:
- ✅ `config/` - Dominio (modelos de datos)
- ✅ `workspace/` - Dominio (lógica de workspace)
- ✅ `git/`, `docker/` - Infraestructura (implementaciones concretas)

## ⚠️ Problemas Arquitectónicos (ALTA Prioridad)

### 1. Falta de Capas Arquitectónicas Claras

**Problema**: No hay separación clara entre capas (presentación, aplicación, dominio, infraestructura).

**Estado actual**:
```
cmd/ (presentación)
  └──> internal/* (todo mezclado)
```

**Problemas**:
- `cmd/` accede directamente a infraestructura (`docker`, `git`)
- No hay capa de aplicación que orqueste casos de uso
- Lógica de negocio mezclada con infraestructura

**Arquitectura ideal** (Clean Architecture / Hexagonal):
```
cmd/                    # Capa de presentación
  └──> internal/app/   # Capa de aplicación (casos de uso)
        └──> internal/domain/  # Capa de dominio
        └──> internal/infra/    # Capa de infraestructura
```

**Tareas**:
- [ ] Crear capa `internal/app/` para casos de uso
- [ ] Mover lógica de orquestación de `cmd/` a `app/`
- [ ] Separar dominio (`internal/domain/`) de infraestructura (`internal/infra/`)
- [ ] Definir interfaces en dominio, implementaciones en infra

### 2. Dependencias Invertidas Incorrectas

**Problema**: Alto nivel depende de bajo nivel.

**Ejemplo problemático**:
```go
// ❌ MAL - cmd depende directamente de infraestructura
// cmd/up.go
import (
    "raioz/internal/docker"  // Infraestructura
    "raioz/internal/git"     // Infraestructura
)
```

**Solución**:
```go
// ✅ BIEN - cmd depende de aplicación, aplicación depende de interfaces
// cmd/up.go
import "raioz/internal/app"

// internal/app/up.go
type UpUseCase struct {
    dockerRunner DockerRunner  // Interface
    gitService   GitService    // Interface
}

// internal/infra/docker/runner.go
type RealDockerRunner struct {}  // Implementación
```

**Tareas**:
- [ ] Crear interfaces en dominio o aplicación
- [ ] Mover implementaciones a `internal/infra/`
- [ ] Inyectar dependencias en casos de uso
- [ ] `cmd/` solo debe depender de `app/`

### 3. Acoplamiento Fuerte entre Paquetes

**Problema**: Paquetes dependen directamente de otros paquetes.

**Dependencias problemáticas**:
```
docker -> config, env, workspace
validate -> config, docker
state -> config, workspace
git -> config
workspace -> config
```

**Problemas**:
- Cambios en un paquete afectan a muchos otros
- Difícil testear en aislamiento
- Ciclos de dependencia potenciales

**Solución**: Usar interfaces y Dependency Injection.

**Tareas**:
- [ ] Definir interfaces para dependencias
- [ ] Usar Dependency Injection
- [ ] Reducir dependencias directas

### 4. Falta de Capa de Aplicación

**Problema**: Lógica de orquestación está en `cmd/`.

**Ejemplo problemático**:
```go
// ❌ MAL - Lógica de negocio en cmd/up.go
func (c *upCmd) RunE(...) error {
    // 300+ líneas de lógica de orquestación
    deps, _ := config.LoadDeps(...)
    ws, _ := workspace.Resolve(...)
    lock, _ := lock.Acquire(...)
    // ... más lógica
}
```

**Solución**:
```go
// ✅ BIEN - Lógica en capa de aplicación
// internal/app/up.go
type UpUseCase struct {
    configLoader   ConfigLoader
    workspace      WorkspaceManager
    lockManager    LockManager
    gitService     GitService
    dockerRunner   DockerRunner
    stateManager   StateManager
}

func (uc *UpUseCase) Execute(ctx context.Context, opts UpOptions) error {
    // Lógica de orquestación aquí
}
```

**Tareas**:
- [ ] Crear `internal/app/` con casos de uso
- [ ] Mover lógica de `cmd/` a `app/`
- [ ] `cmd/` solo debe llamar a casos de uso

### 5. Mezcla de Responsabilidades en Paquetes

**Problema**: Algunos paquetes hacen demasiado.

**Ejemplos**:
- `internal/docker/compose.go` - Genera compose, valida, crea volúmenes, etc.
- `internal/validate/validate.go` - Valida schema, negocio, compatibilidad, etc.
- `cmd/up.go` - Orquesta todo el flujo

**Solución**: Dividir en responsabilidades más pequeñas.

**Tareas**:
- [ ] Dividir `docker/compose.go` en:
  - `compose/generator.go` - Generación
  - `compose/validator.go` - Validación
  - `compose/builder.go` - Construcción
- [ ] Dividir `validate/validate.go` en:
  - `validate/schema.go` - Validación de schema
  - `validate/business.go` - Validación de negocio
  - `validate/compatibility.go` - Validación de compatibilidad

## 📊 Análisis de Dependencias

### Mapa de Dependencias Actual

```
cmd/
  ├──> config
  ├──> docker
  ├──> git
  ├──> workspace
  ├──> validate
  ├──> state
  ├──> lock
  └──> errors

internal/docker/
  ├──> config
  ├──> env
  └──> workspace

internal/validate/
  ├──> config
  └──> docker

internal/state/
  ├──> config
  └──> workspace

internal/git/
  └──> config

internal/workspace/
  └──> config
```

### Problemas de Dependencias

1. **Ciclos potenciales**:
   - `docker` -> `workspace` -> `config`
   - `validate` -> `docker` -> `config`
   - `state` -> `workspace` -> `config`

2. **Dependencias cruzadas**:
   - `docker` depende de `workspace` y viceversa (indirectamente)
   - `validate` depende de `docker` (debería ser al revés)

3. **Falta de abstracciones**:
   - Todo depende de implementaciones concretas
   - No hay interfaces que rompan dependencias

## 🏗️ Arquitectura Recomendada

### Estructura Propuesta (Clean Architecture)

```
raioz/
├── main.go
├── cmd/                    # Capa de presentación
│   └── [comandos].go      # Solo parsing y llamadas a app
│
├── internal/
│   ├── app/               # Capa de aplicación (casos de uso)
│   │   ├── up.go         # Caso de uso: Up
│   │   ├── down.go       # Caso de uso: Down
│   │   ├── status.go     # Caso de uso: Status
│   │   └── ...
│   │
│   ├── domain/            # Capa de dominio
│   │   ├── config/       # Modelos de configuración
│   │   ├── workspace/    # Modelos de workspace
│   │   └── interfaces/   # Interfaces (puertos)
│   │       ├── docker.go    # DockerRunner interface
│   │       ├── git.go       # GitService interface
│   │       └── workspace.go # WorkspaceManager interface
│   │
│   └── infra/             # Capa de infraestructura
│       ├── docker/       # Implementación Docker
│       ├── git/          # Implementación Git
│       ├── filesystem/   # Implementación FileSystem
│       ├── exec/         # Implementación CommandExecutor
│       └── state/        # Implementación StateRepository
│
└── pkg/                   # Utilidades compartidas (opcional)
    ├── errors/
    └── output/
```

### Flujo de Dependencias Correcto

```
cmd/ (presentación)
  └──> app/ (aplicación)
        ├──> domain/ (dominio)
        └──> infra/ (infraestructura)
              └──> domain/interfaces (implementa interfaces)
```

**Reglas**:
- `cmd/` solo depende de `app/`
- `app/` depende de `domain/` (interfaces)
- `infra/` implementa interfaces de `domain/`
- `domain/` no depende de nada (solo interfaces)

## 🔧 Plan de Refactorización

### Fase 1: Crear Capa de Aplicación (Semana 1)

**Objetivo**: Extraer lógica de orquestación de `cmd/` a `app/`.

1. **Crear estructura**:
   - [ ] Crear `internal/app/`
   - [ ] Crear casos de uso:
     - [ ] `up.go` - `UpUseCase`
     - [ ] `down.go` - `DownUseCase`
     - [ ] `status.go` - `StatusUseCase`

2. **Refactorizar comandos**:
   - [ ] Mover lógica de `cmd/up.go` a `app/up.go`
   - [ ] `cmd/up.go` solo debe llamar a `UpUseCase.Execute()`
   - [ ] Repetir para otros comandos

### Fase 2: Crear Interfaces (Semana 2)

**Objetivo**: Definir interfaces en dominio.

1. **Crear interfaces**:
   - [ ] `domain/interfaces/docker.go` - `DockerRunner`
   - [ ] `domain/interfaces/git.go` - `GitService`
   - [ ] `domain/interfaces/workspace.go` - `WorkspaceManager`
   - [ ] `domain/interfaces/state.go` - `StateRepository`
   - [ ] `domain/interfaces/filesystem.go` - `FileSystem`
   - [ ] `domain/interfaces/exec.go` - `CommandExecutor`

2. **Mover implementaciones**:
   - [ ] Mover `internal/docker/` a `internal/infra/docker/`
   - [ ] Mover `internal/git/` a `internal/infra/git/`
   - [ ] Implementar interfaces en infraestructura

### Fase 3: Dependency Injection (Semana 3)

**Objetivo**: Inyectar dependencias en casos de uso.

1. **Refactorizar casos de uso**:
   - [ ] Agregar campos de interfaces a casos de uso
   - [ ] Crear constructores que reciban dependencias
   - [ ] Inyectar dependencias desde `cmd/`

2. **Actualizar tests**:
   - [ ] Usar mocks en tests de casos de uso
   - [ ] Tests sin dependencias reales

### Fase 4: Separar Dominio (Semana 4)

**Objetivo**: Separar modelos de dominio de infraestructura.

1. **Mover modelos**:
   - [ ] Mover `config/` a `domain/config/`
   - [ ] Mover `workspace/` (modelos) a `domain/workspace/`
   - [ ] Mantener solo interfaces en dominio

2. **Actualizar referencias**:
   - [ ] Actualizar imports
   - [ ] Verificar que no hay dependencias circulares

## 📋 Problemas Específicos por Paquete

### `cmd/` - Capa de Presentación

**Problemas**:
- ❌ Contiene lógica de negocio (300+ líneas en `up.go`)
- ❌ Depende directamente de infraestructura
- ❌ Difícil de testear

**Solución**:
- ✅ Mover lógica a `app/`
- ✅ Solo parsing de flags y llamadas a casos de uso
- ✅ Máximo 50 líneas por comando

### `internal/docker/` - Infraestructura Docker

**Problemas**:
- ❌ Mezcla generación, validación y ejecución
- ❌ Depende de `workspace` y `env`
- ❌ No hay abstracción (interfaces)

**Solución**:
- ✅ Crear interface `DockerRunner` en dominio
- ✅ Separar en sub-paquetes: `compose/`, `runner/`, `validator/`
- ✅ Mover a `infra/docker/`

### `internal/git/` - Infraestructura Git

**Problemas**:
- ❌ Depende de `config` (debería ser al revés)
- ❌ No hay abstracción
- ❌ Funciones directas, no estructuras

**Solución**:
- ✅ Crear interface `GitService` en dominio
- ✅ Crear estructura `GitService` con métodos
- ✅ Mover a `infra/git/`

### `internal/workspace/` - Dominio/Infraestructura Mezclado

**Problemas**:
- ❌ Mezcla modelos (Workspace struct) con infraestructura (Resolve)
- ❌ Depende de `config`
- ❌ Lógica de filesystem mezclada

**Solución**:
- ✅ Separar: `domain/workspace/` (modelos) y `infra/workspace/` (implementación)
- ✅ Crear interface `WorkspaceManager` en dominio
- ✅ Inyectar `FileSystem` interface

### `internal/validate/` - Capa de Aplicación

**Problemas**:
- ❌ Depende de `docker` (debería ser al revés)
- ❌ Mezcla validación de schema, negocio y compatibilidad
- ❌ No está claro si es aplicación o dominio

**Solución**:
- ✅ Mover a `app/validation/` o `domain/validation/`
- ✅ Separar responsabilidades
- ✅ Usar interfaces para dependencias

### `internal/state/` - Infraestructura

**Problemas**:
- ❌ Depende de `workspace` y `config`
- ❌ No hay abstracción
- ❌ Mezcla persistencia con lógica de negocio

**Solución**:
- ✅ Crear interface `StateRepository` en dominio
- ✅ Mover a `infra/state/`
- ✅ Separar lógica de negocio (check, diff) a `app/`

## 🎯 Principios Arquitectónicos Violados

### 1. Dependency Inversion Principle (DIP)

**Violación**: Alto nivel depende de bajo nivel.

**Ejemplo**:
```go
// ❌ cmd depende de infraestructura
cmd/up.go -> internal/docker/runner.go
```

**Solución**: Alto nivel depende de abstracciones.

### 2. Single Responsibility Principle (SRP)

**Violación**: Paquetes con múltiples responsabilidades.

**Ejemplo**:
```go
// ❌ docker/compose.go hace demasiado
- Valida dependencias
- Crea volúmenes
- Genera compose
- Escribe archivo
```

**Solución**: Una responsabilidad por paquete/función.

### 3. Interface Segregation Principle (ISP)

**Violación**: No hay interfaces, todo son funciones directas.

**Solución**: Interfaces pequeñas y específicas.

### 4. Open/Closed Principle (OCP)

**Violación**: Difícil extender sin modificar.

**Ejemplo**: Agregar nuevo tipo de source requiere modificar múltiples lugares.

**Solución**: Usar interfaces y Strategy pattern.

## 📊 Métricas Arquitectónicas

### Acoplamiento

- **Alto**: `cmd/` acoplado a 8+ paquetes
- **Medio**: `docker/` acoplado a 3 paquetes
- **Bajo**: `errors/`, `output/` (utilidades)

### Cohesión

- **Alta**: `config/`, `errors/` (responsabilidad única)
- **Media**: `docker/`, `git/` (múltiples responsabilidades relacionadas)
- **Baja**: `cmd/up.go` (hace demasiado)

### Complejidad Ciclomática

- **Alta**: `cmd/up.go` (flujo complejo)
- **Media**: `docker/compose.go` (múltiples pasos)
- **Baja**: Funciones de utilidad

## 🔄 Migración Gradual

### Estrategia: Strangler Pattern

**No refactorizar todo de golpe**. Migrar gradualmente:

1. **Fase 1**: Crear nueva estructura sin romper existente
2. **Fase 2**: Migrar un comando a la vez
3. **Fase 3**: Migrar paquetes gradualmente
4. **Fase 4**: Eliminar código antiguo

### Ejemplo: Migrar `raioz up`

1. Crear `internal/app/up.go` con nueva implementación
2. Mantener `cmd/up.go` antiguo funcionando
3. Agregar flag `--new-impl` para probar nueva implementación
4. Una vez probado, reemplazar implementación antigua
5. Eliminar código antiguo

## 📝 Checklist de Mejoras

### Prioridad ALTA

- [ ] Crear capa `internal/app/` para casos de uso
- [ ] Mover lógica de `cmd/` a `app/`
- [ ] Crear interfaces en `domain/interfaces/`
- [ ] Mover implementaciones a `infra/`
- [ ] Implementar Dependency Injection

### Prioridad MEDIA

- [ ] Separar responsabilidades en paquetes grandes
- [ ] Reducir acoplamiento entre paquetes
- [ ] Crear abstracciones para dependencias
- [ ] Mejorar cohesión de paquetes

### Prioridad BAJA

- [ ] Reorganizar estructura de directorios
- [ ] Documentar arquitectura
- [ ] Crear diagramas de dependencias
- [ ] Establecer convenciones arquitectónicas

## 🎯 Conclusión

**Estado actual**: Arquitectura funcional pero con problemas de diseño:
- ❌ Falta separación de capas
- ❌ Alto acoplamiento
- ❌ Dependencias incorrectas
- ❌ Mezcla de responsabilidades

**Recomendación**: Implementar Clean Architecture gradualmente:
1. Crear capa de aplicación
2. Definir interfaces
3. Implementar Dependency Injection
4. Separar dominio de infraestructura

**Beneficios esperados**:
- ✅ Mejor testabilidad
- ✅ Menor acoplamiento
- ✅ Mayor flexibilidad
- ✅ Más fácil de mantener
