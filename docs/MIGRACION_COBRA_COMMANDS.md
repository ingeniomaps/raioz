# Guía de Migración: Comandos Cobra a UseCases

Fecha: 2026-01-14

## Definición de Archivo Cobra

Un archivo Cobra en `cmd/` debe cumplir los siguientes criterios:

### Reglas Obligatorias

1. **Tamaño**: Entre 50-100 líneas (idealmente < 150)
2. **Imports permitidos**: Solo `app`, `errors`, `logging`, `cobra`
3. **NO debe importar**: Paquetes de infraestructura/dominio (`config`, `docker`, `workspace`, `state`, `output`, `validate`, etc.)
4. **NO debe contener**: Lógica de negocio
5. **Solo debe**: Llamar a UseCases desde `internal/app`

### Estructura Esperada

```go
package cmd

import (
    "raioz/internal/app"
    "raioz/internal/errors"
    "raioz/internal/logging"
    "github.com/spf13/cobra"
)

var (
    // Flags del comando (si aplica)
    commandFlag string
)

var commandCmd = &cobra.Command{
    Use:   "command",
    Short: "Brief description",
    Long:  "Long description",
    RunE: func(cmd *cobra.Command, args []string) (err error) {
        // Panic recovery
        defer func() {
            if panicErr := errors.RecoverPanic("raioz command"); panicErr != nil {
                err = panicErr
            }
        }()

        // Inicializar dependencias
        deps := app.NewDependencies()

        // Crear UseCase
        useCase := app.NewCommandUseCase(deps)

        // Construir opciones (si aplica)
        options := app.CommandOptions{
            // ... campos basados en flags
        }

        // Ejecutar UseCase
        return useCase.Execute(cmd.Context(), options)
    },
}

func init() {
    // Registrar flags
    commandCmd.Flags().StringVarP(&commandFlag, "flag", "f", "default", "Description")
    // Registrar comando en root (si es necesario)
    // rootCmd.AddCommand(commandCmd)
}
```

## Pasos para Migrar un Comando a UseCase

### Paso 1: Analizar el Comando Actual

1. Identificar todas las importaciones que violan las reglas
2. Identificar toda la lógica de negocio
3. Identificar las dependencias externas (archivos, configuración, etc.)
4. Identificar los flags y parámetros del comando

### Paso 2: Crear el UseCase en `internal/app/`

**Opción A: UseCase Simple (un solo archivo)**

1. **Crear archivo**: `internal/app/{command}.go`
2. **Definir estructura Options**:
   ```go
   type CommandOptions struct {
       // Campos basados en flags del comando
       Flag1 string
       Flag2 bool
       // ...
   }
   ```
3. **Definir estructura UseCase**:
   ```go
   type CommandUseCase struct {
       deps *Dependencies
   }
   ```
4. **Crear constructor**:
   ```go
   func NewCommandUseCase(deps *Dependencies) *CommandUseCase {
       return &CommandUseCase{
           deps: deps,
       }
   }
   ```
5. **Implementar método Execute**:
   ```go
   func (u *CommandUseCase) Execute(ctx context.Context, options CommandOptions) error {
       // Toda la lógica de negocio aquí
       // Usar u.deps para acceder a dependencias
       return nil
   }
   ```

### Paso 3: Migrar la Lógica

1. **Mover lógica de negocio** del comando al UseCase
2. **Usar Dependencies** para acceder a servicios externos
3. **Usar context.Context** para cancelación y timeouts
4. **Manejar errores** usando `errors` package
5. **Usar logging** para mensajes informativos

### Paso 4: Simplificar el Comando Cobra

1. **Eliminar imports** de infraestructura/dominio
2. **Eliminar lógica de negocio**
3. **Agregar panic recovery** (si no existe)
4. **Inicializar Dependencies**
5. **Llamar al UseCase**

### Paso 5: Actualizar Dependencies (si es necesario)

Si el UseCase necesita acceso a nuevos servicios:

1. **Agregar campos** a `Dependencies` en `internal/app/dependencies.go`
2. **Inicializar** en `NewDependencies()`
3. **Usar** en el UseCase a través de `u.deps`

### Paso 6: Probar y Verificar

1. **Compilar**: `go build ./cmd/raioz`
2. **Ejecutar**: Probar el comando con diferentes flags
3. **Verificar**: Que no haya imports prohibidos
4. **Verificar**: Que el tamaño del archivo sea < 150 líneas
5. **Verificar**: Que solo llame al UseCase

## Estructura de Dependencies

El struct `Dependencies` en `internal/app/dependencies.go` debe contener:

- **Interfaces** (NO implementaciones concretas)
- **Acceso a servicios** necesarios para los UseCases
- **Inicialización** en `NewDependencies()`

## Ejemplo Completo: Migración de `init.go`

### Estado Actual (❌ No cumple)

```go
// cmd/init.go
package cmd

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "raioz/internal/config"  // ❌ Prohibido
    "raioz/internal/errors"
    "raioz/internal/output"  // ❌ Prohibido
    "raioz/internal/validate" // ❌ Prohibido

    "github.com/spf13/cobra"
)

// ... 156 líneas con lógica de negocio
```

### Estado Deseado (✅ Cumple)

```go
// cmd/init.go
package cmd

import (
    "context"

    "raioz/internal/app"
    "raioz/internal/errors"

    "github.com/spf13/cobra"
)

var (
    initOutputPath string
)

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize a new .raioz.json configuration file",
    RunE: func(cmd *cobra.Command, args []string) (err error) {
        defer func() {
            if panicErr := errors.RecoverPanic("raioz init"); panicErr != nil {
                err = panicErr
            }
        }()

        deps := app.NewDependencies()
        useCase := app.NewInitUseCase(deps)

        options := app.InitOptions{
            OutputPath: initOutputPath,
        }

        return useCase.Execute(cmd.Context(), options)
    },
}

func init() {
    initCmd.Flags().StringVarP(&initOutputPath, "output", "o", ".raioz.json", "Output path")
}
```

```go
// internal/app/init.go (Wrapper)
package app

import (
    "context"

    initcase "raioz/internal/app/initcase"
)

type InitOptions struct {
    OutputPath string
}

type InitUseCase struct {
    useCase *initcase.UseCase
}

func NewInitUseCase(deps *Dependencies) *InitUseCase {
    return &InitUseCase{
        useCase: initcase.NewUseCase(),
    }
}

func (uc *InitUseCase) Execute(ctx context.Context, opts InitOptions) error {
    options := initcase.Options{
        OutputPath: opts.OutputPath,
    }
    return uc.useCase.Execute(ctx, options)
}
```

```go
// internal/app/initcase/usecase.go (Orquestación)
package initcase

import (
    "context"
)

type Options struct {
    OutputPath string
}

type UseCase struct {
}

func NewUseCase() *UseCase {
    return &UseCase{}
}

func (uc *UseCase) Execute(ctx context.Context, opts Options) error {
    uc.showWelcomeMessage()
    shouldContinue, err := uc.checkFileExists(opts.OutputPath)
    if err != nil {
        return err
    }
    if !shouldContinue {
        return nil
    }
    projectName, networkName, err := uc.promptProjectInfo()
    if err != nil {
        return err
    }
    deps, err := uc.createConfig(projectName, networkName)
    if err != nil {
        return err
    }
    if err := uc.writeConfigFile(opts.OutputPath, deps); err != nil {
        return err
    }
    uc.showSuccessMessage(opts.OutputPath)
    return nil
}
```

```go
// internal/app/initcase/prompts.go (Interacción con usuario)
package initcase

// Funciones: promptProjectInfo, checkFileExists, promptString,
// showWelcomeMessage, showSuccessMessage
```

```go
// internal/app/initcase/config.go (Configuración y validación)
package initcase

// Funciones: createConfig, writeConfigFile
```

**Estructura final**:

- `internal/app/init.go` (32 líneas) - Wrapper
- `internal/app/initcase/usecase.go` (56 líneas) - Orquestación
- `internal/app/initcase/prompts.go` (82 líneas) - Prompts de usuario
- `internal/app/initcase/config.go` (67 líneas) - Configuración y validación

**Nota importante**: El nombre del paquete es `initcase` (no `init`) porque `init` es una palabra reservada en Go y no puede usarse como nombre de paquete.

## Checklist de Migración

- [ ] Analizar comando actual
- [ ] Crear UseCase en `internal/app/{command}.go`
- [ ] Definir `{Command}Options` struct
- [ ] Definir `{Command}UseCase` struct
- [ ] Implementar `New{Command}UseCase`
- [ ] Implementar `Execute`
- [ ] Migrar lógica de negocio
- [ ] Actualizar Dependencies (si necesario)
- [ ] Simplificar comando Cobra
- [ ] Eliminar imports prohibidos
- [ ] Agregar panic recovery
- [ ] Verificar tamaño del archivo (< 150 líneas)
- [ ] Probar compilación
- [ ] Probar ejecución
- [ ] Verificar que solo llama al UseCase

## Comandos Pendientes de Migración

Según `docs/CMD_COBRA_ANALYSIS.md`:

1. **list.go** → `ListUseCase`
2. **clean.go** → `CleanUseCase`
3. **compare.go** → `CompareUseCase`
4. **ignore.go** → `IgnoreUseCase`
5. **link.go** → `LinkUseCase`
6. **logs.go** → `LogsUseCase`
7. **migrate.go** → `MigrateUseCase`
8. **override.go** → `OverrideUseCase`
9. **ports.go** → `PortsUseCase`
10. **workspace.go** → `WorkspaceUseCase`
11. **ci.go** + `ci_*.go` → `CiUseCase` (candidato para subcarpeta)
12. **init.go** → `InitUseCase` ✅ **Completado** (usando subcarpeta `initcase/`)

## Notas Importantes

1. **NO importar paquetes de infraestructura/dominio** en `cmd/`
2. **Solo UseCases** deben tener acceso a infraestructura
3. **Usar Dependencies** para inyección de dependencias
4. **Context.Context** debe propagarse desde el comando
5. **Panic recovery** en todos los comandos
6. **Manejo de errores** usando `errors` package
7. **Tamaño máximo**: 150 líneas por archivo Cobra
