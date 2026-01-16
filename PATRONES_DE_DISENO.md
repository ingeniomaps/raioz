# Análisis de Patrones de Diseño - Raioz

Análisis de los patrones de diseño usados en el código, qué está bien, qué falta y qué mejoras aplicar.

## ✅ Patrones Bien Implementados

### 1. Strategy Pattern (Bien usado)

**Ubicación**: `internal/config/filter.go`

**Implementación**:
```go
// FilterByProfile - Strategy para filtrar por perfil
func FilterByProfile(deps *Deps, profile string) *Deps

// FilterByFeatureFlags - Strategy para filtrar por feature flags
func FilterByFeatureFlags(deps *Deps, profile string, envVars map[string]string) (*Deps, []string)
```

**Análisis**: ✅ Bien implementado. Permite diferentes estrategias de filtrado sin modificar la estructura base.

**Mejora sugerida**: Podría usar interfaces para mayor flexibilidad:
```go
type FilterStrategy interface {
    Filter(deps *Deps) (*Deps, error)
}
```

### 2. Builder Pattern (Bien usado)

**Ubicación**: `internal/errors/types.go`

**Implementación**:
```go
err := errors.New(ErrCodeInvalidConfig, "message").
    WithContext("key", "value").
    WithSuggestion("suggestion").
    WithError(originalErr)
```

**Análisis**: ✅ Excelente implementación. Permite construcción fluida de errores con contexto.

### 3. Factory Pattern (Bien usado)

**Ubicación**: `internal/workspace/workspace.go`

**Implementación**:
```go
func Resolve(project string) (*Workspace, error)
func GetBaseDir() (string, error)
```

**Análisis**: ✅ Bien implementado. Centraliza la creación de objetos Workspace.

### 4. Error Wrapping Pattern (Bien usado)

**Ubicación**: `internal/errors/types.go`

**Implementación**:
```go
type RaiozError struct {
    OriginalErr error
    // ...
}

func (e *RaiozError) Unwrap() error {
    return e.OriginalErr
}
```

**Análisis**: ✅ Excelente. Implementa `Unwrap()` para compatibilidad con `errors.Is()` y `errors.As()`.

### 5. Repository Pattern (Bien usado)

**Ubicación**: `internal/state/state.go`

**Implementación**:
```go
func Save(ws *workspace.Workspace, deps *config.Deps) error
func Load(ws *workspace.Workspace) (*config.Deps, error)
func Exists(ws *workspace.Workspace) bool
```

**Análisis**: ✅ Bien implementado. Abstrae el acceso a datos de estado.

### 6. Facade Pattern (Bien usado)

**Ubicación**: `internal/docker/runner.go`

**Implementación**:
```go
func Up(composePath string) error
func Down(composePath string) error
```

**Análisis**: ✅ Bien implementado. Oculta la complejidad de ejecutar comandos Docker.

## ⚠️ Patrones Parcialmente Implementados (Mejorable)

### 7. Strategy Pattern (Mejorable)

**Problema**: Se usa Strategy pero sin interfaces, lo que limita extensibilidad.

**Mejora sugerida**:
```go
// Definir interfaz
type ServiceFilter interface {
    ShouldInclude(svc Service, profile string, envVars map[string]string) bool
}

// Implementaciones
type ProfileFilter struct { profile string }
type FeatureFlagFilter struct { envVars map[string]string }

// Composición
type CompositeFilter struct {
    filters []ServiceFilter
}
```

### 8. Template Method Pattern (Falta)

**Problema**: Hay código repetitivo en comandos (`up`, `down`, `status`).

**Mejora sugerida**:
```go
type CommandTemplate struct {
    PreExecute func() error
    Execute    func() error
    PostExecute func() error
}

func (c *CommandTemplate) Run() error {
    if err := c.PreExecute(); err != nil {
        return err
    }
    if err := c.Execute(); err != nil {
        return err
    }
    return c.PostExecute()
}
```

## ❌ Patrones Faltantes (ALTA Prioridad)

### 1. Dependency Injection (CRÍTICO)

**Problema actual**: Dependencias directas en lugar de interfaces.

**Ejemplo problemático**:
```go
// ❌ MAL - Dependencia directa
func Up(composePath string) error {
    cmd := exec.Command("docker", "compose", ...)
    return cmd.Run()
}
```

**Solución**:
```go
// ✅ BIEN - Con inyección de dependencias
type DockerRunner interface {
    Up(composePath string) error
    Down(composePath string) error
}

type RealDockerRunner struct{}

func (r *RealDockerRunner) Up(composePath string) error {
    cmd := exec.Command("docker", "compose", ...)
    return cmd.Run()
}

// En tests, usar mock
type MockDockerRunner struct {
    UpFunc   func(string) error
    DownFunc func(string) error
}
```

**Tareas**:
- [ ] Crear interfaces para:
  - `DockerRunner` (docker operations)
  - `GitRepository` (git operations)
  - `WorkspaceManager` (workspace operations)
  - `FileSystem` (file operations)
  - `CommandExecutor` (exec.Command wrapper)
- [ ] Refactorizar funciones para aceptar interfaces
- [ ] Inyectar dependencias en constructores
- [ ] Actualizar tests para usar mocks

### 2. Interface Segregation Principle (ALTA Prioridad)

**Problema actual**: No hay interfaces claras, todo son funciones directas.

**Solución**:
```go
// ✅ BIEN - Interfaces pequeñas y específicas
type GitCloner interface {
    Clone(repo, branch, target string) error
}

type GitUpdater interface {
    Update(repoPath, branch string) error
}

type GitChecker interface {
    IsReadonly(repoPath string) bool
}

// En lugar de una función monolítica
type GitService interface {
    GitCloner
    GitUpdater
    GitChecker
}
```

**Tareas**:
- [ ] Definir interfaces pequeñas y específicas
- [ ] Separar responsabilidades en interfaces
- [ ] Evitar interfaces "fat" (muchos métodos)

### 3. Dependency Inversion Principle (ALTA Prioridad)

**Problema actual**: Código de alto nivel depende de implementaciones concretas.

**Ejemplo problemático**:
```go
// ❌ MAL - Alto nivel depende de bajo nivel
func (c *upCmd) RunE(...) error {
    docker.Up(composePath)  // Dependencia directa
    git.EnsureRepo(...)     // Dependencia directa
}
```

**Solución**:
```go
// ✅ BIEN - Alto nivel depende de abstracciones
type UpCommand struct {
    dockerRunner DockerRunner
    gitService   GitService
    workspace    WorkspaceManager
}

func (c *UpCommand) Execute() error {
    if err := c.gitService.EnsureRepo(...); err != nil {
        return err
    }
    return c.dockerRunner.Up(...)
}
```

**Tareas**:
- [ ] Crear estructuras de comandos con dependencias inyectadas
- [ ] Mover lógica de comandos a estructuras
- [ ] Inyectar dependencias en constructores

### 4. Single Responsibility Principle (MEDIA Prioridad)

**Problema actual**: Algunas funciones hacen demasiado.

**Ejemplo problemático**:
```go
// ❌ MAL - Hace demasiado
func GenerateCompose(deps *config.Deps, ws *workspace.Workspace) (string, error) {
    // Valida dependencias
    // Asegura directorios
    // Extrae volúmenes
    // Crea volúmenes
    // Genera compose
    // Escribe archivo
}
```

**Solución**:
```go
// ✅ BIEN - Responsabilidades separadas
func ValidateDependencies(deps *config.Deps) error { ... }
func EnsureVolumes(deps *config.Deps) error { ... }
func BuildComposeConfig(deps *config.Deps, ws *workspace.Workspace) (*ComposeConfig, error) { ... }
func WriteComposeFile(config *ComposeConfig, path string) error { ... }

func GenerateCompose(deps *config.Deps, ws *workspace.Workspace) (string, error) {
    if err := ValidateDependencies(deps); err != nil {
        return "", err
    }
    if err := EnsureVolumes(deps); err != nil {
        return "", err
    }
    config, err := BuildComposeConfig(deps, ws)
    if err != nil {
        return "", err
    }
    path := GetComposePath(ws)
    return path, WriteComposeFile(config, path)
}
```

**Tareas**:
- [ ] Identificar funciones con múltiples responsabilidades
- [ ] Dividir en funciones más pequeñas
- [ ] Aplicar principio de responsabilidad única

### 5. Command Pattern (MEDIA Prioridad)

**Problema actual**: Lógica de comandos mezclada con Cobra.

**Solución**:
```go
// ✅ BIEN - Separar lógica de comandos
type UpCommand struct {
    configLoader   ConfigLoader
    validator       Validator
    workspace       WorkspaceManager
    dockerRunner    DockerRunner
    gitService      GitService
}

func (c *UpCommand) Execute(ctx context.Context, opts UpOptions) error {
    // Lógica del comando separada de Cobra
}

// En cmd/up.go
var upCmd = &cobra.Command{
    RunE: func(cmd *cobra.Command, args []string) error {
        upCmd := NewUpCommand(deps...)
        return upCmd.Execute(ctx, opts)
    },
}
```

**Tareas**:
- [ ] Crear estructuras de comandos
- [ ] Separar lógica de Cobra
- [ ] Hacer comandos testeables independientemente

### 6. Observer Pattern (BAJA Prioridad)

**Problema actual**: No hay sistema de eventos/notificaciones.

**Uso potencial**: Notificar sobre cambios de estado, progreso, etc.

**Solución**:
```go
type EventType string

const (
    EventServiceStarted EventType = "service.started"
    EventServiceStopped EventType = "service.stopped"
    EventConfigChanged  EventType = "config.changed"
)

type Event struct {
    Type    EventType
    Payload interface{}
}

type EventBus interface {
    Subscribe(eventType EventType, handler func(Event))
    Publish(event Event)
}
```

**Tareas**:
- [ ] Evaluar si se necesita sistema de eventos
- [ ] Implementar solo si hay necesidad real

## 📋 Plan de Refactorización

### Fase 1: Dependency Injection (Semana 1)

**Objetivo**: Introducir interfaces y DI básico.

1. **Crear interfaces principales**:
   - [ ] `DockerRunner` interface
   - [ ] `GitService` interface
   - [ ] `WorkspaceManager` interface
   - [ ] `FileSystem` interface
   - [ ] `CommandExecutor` interface

2. **Refactorizar paquetes**:
   - [ ] `internal/docker/runner.go` → usar `DockerRunner` interface
   - [ ] `internal/git/` → usar `GitService` interface
   - [ ] `internal/workspace/` → usar `WorkspaceManager` interface

3. **Actualizar comandos**:
   - [ ] Inyectar dependencias en comandos
   - [ ] Crear constructores para comandos

### Fase 2: Interface Segregation (Semana 2)

**Objetivo**: Dividir interfaces grandes en pequeñas.

1. **Analizar interfaces existentes**:
   - [ ] Identificar interfaces "fat"
   - [ ] Dividir en interfaces más pequeñas

2. **Refactorizar**:
   - [ ] Crear interfaces específicas
   - [ ] Actualizar implementaciones

### Fase 3: Single Responsibility (Semana 3)

**Objetivo**: Dividir funciones grandes.

1. **Identificar funciones grandes**:
   - [ ] `GenerateCompose()` - dividir en pasos
   - [ ] `cmd/up.go` - extraer lógica a estructuras
   - [ ] Otras funciones con múltiples responsabilidades

2. **Refactorizar**:
   - [ ] Dividir funciones
   - [ ] Crear funciones helper
   - [ ] Aplicar SRP

### Fase 4: Command Pattern (Semana 4)

**Objetivo**: Separar lógica de comandos de Cobra.

1. **Crear estructuras de comandos**:
   - [ ] `UpCommand` struct
   - [ ] `DownCommand` struct
   - [ ] `StatusCommand` struct

2. **Refactorizar**:
   - [ ] Mover lógica a estructuras
   - [ ] Hacer comandos testeables

## 🎯 Recomendaciones Prioritarias

### Prioridad ALTA (Implementar primero)

1. **Dependency Injection**
   - Facilita testing
   - Mejora mantenibilidad
   - Permite mocks reales

2. **Interface Segregation**
   - Mejora flexibilidad
   - Facilita testing
   - Reduce acoplamiento

3. **Dependency Inversion**
   - Mejora arquitectura
   - Facilita cambios
   - Mejora testabilidad

### Prioridad MEDIA (Implementar después)

4. **Single Responsibility**
   - Mejora legibilidad
   - Facilita mantenimiento
   - Reduce complejidad

5. **Command Pattern**
   - Separa lógica de CLI
   - Mejora testabilidad
   - Facilita reutilización

### Prioridad BAJA (Evaluar necesidad)

6. **Observer Pattern**
   - Solo si se necesita sistema de eventos
   - Útil para notificaciones
   - Puede ser overkill

## 📊 Comparación: Antes vs Después

### Antes (Estado Actual)

```go
// ❌ Dependencias directas
func Up(composePath string) error {
    cmd := exec.Command("docker", "compose", ...)
    return cmd.Run()
}

// ❌ Difícil de testear
func TestUp() {
    // Necesita Docker real
}
```

### Después (Con Patrones)

```go
// ✅ Con Dependency Injection
type UpCommand struct {
    dockerRunner DockerRunner
    validator    Validator
}

func (c *UpCommand) Execute(ctx context.Context) error {
    if err := c.validator.Validate(); err != nil {
        return err
    }
    return c.dockerRunner.Up(...)
}

// ✅ Fácil de testear
func TestUp() {
    mockRunner := &MockDockerRunner{}
    cmd := &UpCommand{dockerRunner: mockRunner}
    // Test sin Docker real
}
```

## 🔗 Referencias

- [Go Design Patterns](https://github.com/tmrts/go-patterns)
- [SOLID Principles in Go](https://dave.cheney.net/2016/08/20/solid-go-design)
- [Effective Go - Interfaces](https://go.dev/doc/effective_go#interfaces)
- [Dependency Injection in Go](https://blog.drewolson.org/dependency-injection-in-go)

## 📝 Notas Finales

**Estado actual**: El código tiene una base sólida con algunos patrones bien implementados (Builder, Factory, Error Wrapping). Sin embargo, falta Dependency Injection y Interface Segregation, lo que limita la testabilidad y flexibilidad.

**Recomendación**: Implementar Dependency Injection primero, ya que es la base para mejorar otros aspectos del código y facilitar el testing.
