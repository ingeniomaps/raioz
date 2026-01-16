# Guía de Refactorización Arquitectónica

Este documento describe la estructura arquitectónica objetivo y cómo migrar gradualmente el código existente.

## 🏗️ Arquitectura Objetivo (Clean Architecture)

```
raioz/
├── cmd/                    # Capa de Presentación (CLI)
│   └── [comandos].go      # Solo parsing de flags y llamadas a app/
│
├── internal/
│   ├── app/               # Capa de Aplicación (Casos de Uso)
│   │   ├── up.go         # UpUseCase
│   │   ├── down.go       # DownUseCase
│   │   └── ...
│   │
│   ├── domain/            # Capa de Dominio
│   │   ├── interfaces/   # Interfaces (puertos)
│   │   │   ├── docker.go
│   │   │   ├── git.go
│   │   │   └── ...
│   │   └── models/       # Modelos de dominio (futuro)
│   │       ├── config.go
│   │       └── workspace.go
│   │
│   └── infra/             # Capa de Infraestructura
│       ├── docker/       # Implementaciones concretas
│       ├── git/
│       └── ...
│
└── pkg/                   # Utilidades compartidas
    ├── errors/
    └── output/
```

## 📋 Estado Actual vs Objetivo

### Estado Actual ✅ (Infraestructura lista)

- ✅ `internal/domain/interfaces/` - Interfaces definidas
- ✅ `internal/infra/` - Implementaciones básicas creadas
- ✅ `internal/app/` - Estructura creada con ejemplo
- ⏸️ Código existente aún en ubicaciones originales

### Estado Objetivo

- ⏸️ `internal/app/` - Todos los casos de uso implementados
- ⏸️ `cmd/` - Solo parsing, llama a casos de uso
- ⏸️ `internal/domain/models/` - Modelos movidos desde `config/` y `workspace/`
- ⏸️ `internal/infra/` - Implementaciones completas

## 🔄 Estrategia de Migración (Strangler Pattern)

### Fase 1: Estructura Base ✅ COMPLETADA

- ✅ Crear `internal/domain/interfaces/`
- ✅ Crear `internal/infra/` con implementaciones wrapper
- ✅ Crear `internal/app/` con estructura y ejemplo
- ✅ Documentar arquitectura

### Fase 2: Migración Gradual (Pendiente)

1. **Crear casos de uso uno por uno**:
   - Empezar con casos de uso simples (status, down)
   - Luego migrar casos más complejos (up)
   - Mantener código original funcionando durante migración

2. **Actualizar comandos gradualmente**:
   - Mantener lógica original en `cmd/`
   - Crear caso de uso en paralelo
   - Actualizar `cmd/` para usar caso de uso
   - Eliminar código duplicado

3. **Mover modelos a dominio** (cuando sea necesario):
   - Esto requiere actualizar imports en todo el código
   - Hacer gradualmente, empezando con modelos menos usados

4. **Mover implementaciones a infra** (cuando sea necesario):
   - Similar a modelos, requiere actualizar imports
   - Hacer gradualmente

## 📝 Ejemplo de Migración

### Antes (cmd/up.go)

```go
func (c *upCmd) RunE(...) error {
    // 500+ líneas de lógica de orquestación
    deps, _ := config.LoadDeps(...)
    ws, _ := workspace.Resolve(...)
    docker.Up(...)
    // ...
}
```

### Después (cmd/up.go)

```go
func (c *upCmd) RunE(...) error {
    // Inyectar dependencias
    dockerRunner := docker.NewDockerRunner()
    gitRepo := git.NewGitRepository()
    workspace := workspace.NewWorkspaceManager()

    // Crear caso de uso
    useCase := app.NewUpUseCase(dockerRunner, gitRepo, workspace)

    // Ejecutar caso de uso
    opts := app.UpOptions{
        ConfigPath: configPath,
        Profile:    profile,
    }
    return useCase.Execute(ctx, opts)
}
```

### Caso de Uso (internal/app/up.go)

```go
func (uc *UpUseCase) Execute(ctx context.Context, opts UpOptions) error {
    // Lógica de orquestación aquí
    // Dependencias inyectadas via interfaces
    // Testeable con mocks
}
```

## ⚠️ Consideraciones

1. **No romper funcionalidad existente**: Mantener código original funcionando
2. **Migración gradual**: Hacer un comando a la vez
3. **Tests primero**: Escribir tests para casos de uso antes de migrar
4. **Mocks listos**: Usar mocks existentes o generar nuevos con Mockery
5. **Documentar cambios**: Actualizar documentación durante migración

## 🎯 Prioridades

1. **Alta**: Crear casos de uso simples (status, down) - Menos dependencias
2. **Media**: Migrar casos de uso complejos (up) - Más dependencias
3. **Baja**: Mover modelos a dominio - Requiere cambios masivos en imports
4. **Baja**: Mover implementaciones a infra - Requiere cambios masivos en imports

## 📚 Referencias

- [Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [Strangler Pattern](https://martinfowler.com/bliki/StranglerFigApplication.html)
