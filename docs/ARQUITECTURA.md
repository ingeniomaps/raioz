# Arquitectura de Raioz - Analisis Profundo

Este documento complementa la seccion de arquitectura en `CLAUDE.md` con analisis
detallado de patrones de diseno, decisiones arquitectonicas, dependencias entre
paquetes y areas de mejora identificadas.

> **Nota**: Para la vision general de capas, comandos de build y convenciones,
> consultar `CLAUDE.md` en la raiz del proyecto.

---

## 1. Flujo de Dependencias entre Capas

```
cmd/raioz/main.go
  |
  v
internal/cli/             Registra comandos Cobra (thin wrappers)
  |
  v
internal/app/             Casos de uso: UpUseCase, DownUseCase, etc.
  |                       Cada uno recibe *Dependencies (interfaces)
  |--- internal/app/upcase/   Orquestacion compleja de `raioz up` (19 archivos)
  |
  v
internal/domain/
  |--- interfaces/        Puertos: DockerRunner, GitRepository, StateManager, etc.
  |--- models/            Type aliases que re-exportan tipos de config/, state/, etc.
  |
  v
internal/infra/           Adaptadores que implementan domain/interfaces
  |--- delegando a: docker/, git/, workspace/, state/, env/, etc.
```

### Reglas de dependencia actuales

| Capa              | Puede importar                          | NO debe importar        |
|-------------------|-----------------------------------------|-------------------------|
| `cmd/`, `cli/`    | `app/`, `domain/`                       | paquetes concretos      |
| `app/`            | `domain/interfaces`, `domain/models`    | `infra/`, `docker/`, etc|
| `domain/`         | solo stdlib                             | nada externo            |
| `infra/`          | `domain/interfaces`, paquetes concretos | `app/`, `cmd/`          |

### Mapa de dependencias de paquetes concretos

```
internal/docker/   --> config, env, workspace
internal/validate/ --> config, docker
internal/state/    --> config, workspace
internal/git/      --> config
internal/workspace/--> config
internal/env/      --> config
```

La capa `infra/` encapsula estas dependencias detras de interfaces, pero los
paquetes concretos internos siguen acoplados entre si. Esto es aceptable
mientras la capa de aplicacion no los importe directamente.

---

## 2. Patrones de Diseno Implementados

### 2.1 Builder Pattern - Errores estructurados

**Ubicacion**: `internal/errors/types.go`

```go
err := errors.New(ErrCodeInvalidConfig, "message").
    WithContext("key", "value").
    WithSuggestion("suggestion").
    WithError(originalErr)
```

Permite construccion fluida de errores con contexto, sugerencias y wrapping.
Implementa `Unwrap()` para compatibilidad con `errors.Is()` / `errors.As()`.

### 2.2 Strategy Pattern - Filtrado de servicios

**Ubicacion**: `internal/config/filter.go`

```go
FilterByProfile(deps *Deps, profile string) *Deps
FilterByFeatureFlags(deps *Deps, profile string, envVars map[string]string) (*Deps, []string)
```

Estrategias de filtrado intercambiables. Actualmente son funciones; podrian
evolucionar a una interfaz `FilterStrategy` si se necesitan mas variantes.

### 2.3 Factory Pattern - Creacion de workspace

**Ubicacion**: `internal/workspace/workspace.go`

`Resolve()` y `GetBaseDir()` centralizan la creacion de objetos `Workspace`,
encapsulando la logica de resolucion de rutas y directorios.

### 2.4 Facade Pattern - Docker runner

**Ubicacion**: `internal/docker/runner.go`

Funciones `Up()`, `Down()` ocultan la complejidad de construir y ejecutar
comandos `docker compose`, incluyendo manejo de argumentos, paths y variables.

### 2.5 Repository Pattern - Estado persistente

**Ubicacion**: `internal/state/state.go`

`Save()`, `Load()`, `Exists()` abstraen la persistencia de `.state.json`.
La capa de aplicacion accede via la interfaz `StateManager`.

### 2.6 Dependency Injection - Struct de dependencias

**Ubicacion**: `internal/app/dependencies.go`

```go
type Dependencies struct {
    DockerRunner     interfaces.DockerRunner
    GitRepository    interfaces.GitRepository
    WorkspaceManager interfaces.WorkspaceManager
    StateManager     interfaces.StateManager
    // ... 9 interfaces en total
}
```

Cada caso de uso recibe `*Dependencies`. Los comandos en `cmd/` construyen
las dependencias con `app.NewDependencies()` que inyecta implementaciones reales.
Los tests usan mocks de `internal/mocks/`.

### 2.7 Adapter Pattern - Capa infra

**Ubicacion**: `internal/infra/`

Cada adaptador implementa una interfaz de dominio delegando al paquete concreto.
Ejemplo: `infra.DockerRunnerAdapter` implementa `interfaces.DockerRunner`
delegando a funciones de `internal/docker/`.

---

## 3. Decisiones Arquitectonicas Clave

### 3.1 Type aliases en domain/models vs mover tipos

Se decidio usar type aliases que re-exportan tipos de `config/`, `state/`, etc.
en lugar de mover los tipos a `domain/models/`. Esto permite:

- Desacoplar la capa de aplicacion sin un refactor masivo de imports
- Los paquetes concretos mantienen sus tipos internamente
- La capa de dominio expone solo lo necesario

**Trade-off**: Hay una indirection extra, pero evita el Big Bang de mover
cientos de referencias de tipo.

### 3.2 upcase/ como sub-paquete de app/

El flujo de `raioz up` es complejo (19 archivos). Se separo en
`internal/app/upcase/` con su propio `Dependencies` struct. Esto:

- Mantiene cada archivo bajo el limite de 400 lineas
- Separa las fases del flujo (clone, build, compose, network, etc.)
- Permite testear cada fase independientemente

### 3.3 Mocks manuales vs generados

Se usan mocks manuales en `internal/mocks/` en lugar de herramientas como
mockery. Cada mock implementa la interfaz correspondiente con campos `Func`
para inyectar comportamiento en tests:

```go
type MockDockerRunner struct {
    UpFunc   func(string) error
    DownFunc func(string) error
}
```

Esto da control total y evita dependencias externas de generacion.

### 3.4 Strangler Pattern para migracion

La migracion a Clean Architecture se hizo gradualmente:

1. Se creo la estructura `domain/interfaces/` + `infra/` + `app/`
2. Se migraron comandos uno por uno (empezando por los simples)
3. El codigo original coexistio con el nuevo durante la transicion
4. Se eliminaron las implementaciones antiguas al completar cada comando

---

## 4. Analisis de Complejidad por Paquete

### 4.1 docker/ - El paquete mas complejo

Responsabilidades actuales:
- Generacion de docker-compose.yml (`compose.go`)
- Gestion de redes (`networks.go`)
- Gestion de volumenes (`volumes.go`, `volumes_shared.go`)
- Asignacion de puertos (`ports.go`)
- Modos dev vs prod (`mode.go`)
- Healthchecks (`healthcheck_config.go`)
- Manejo de imagenes (`images.go`)
- Ejecucion de comandos docker (`runner.go`)

Este paquete concentra la mayor complejidad porque Docker Compose tiene muchas
dimensiones de configuracion. La separacion actual en multiples archivos mitiga
el problema, pero el acoplamiento interno es alto.

### 4.2 config/ - El hub de tipos

Casi todos los paquetes concretos importan `config/` para acceder a `Deps`,
`Service`, `SourceConfig`, `EnvValue`, `NetworkConfig`, etc. Esto crea un
hub de dependencias donde cambios en tipos de config afectan a muchos paquetes.

La capa `domain/models/` mitiga esto parcialmente al re-exportar tipos,
pero los paquetes concretos siguen acoplados directamente.

### 4.3 validate/ - Multiples niveles de validacion

Realiza tres tipos de validacion distintos:
- **Schema**: Validacion JSON Schema via gojsonschema
- **Semantica**: Reglas de negocio (puertos duplicados, redes invalidas, etc.)
- **Compatibilidad**: Validacion cruzada entre servicios

Estos niveles estan separados en funciones pero viven en el mismo paquete.

---

## 5. Tipos Polimorficos y su Complejidad

### EnvValue: array u objeto

```json
// Forma array
"env": ["VAR1=value1", "VAR2=value2"]

// Forma objeto
"env": { "VAR1": "value1", "VAR2": "value2" }
```

Requiere `UnmarshalJSON` custom que detecta el tipo y normaliza. Esto afecta
a `config/`, `env/`, `docker/` y `validate/`.

### NetworkConfig: string u objeto

```json
// Forma string
"network": "my-network"

// Forma objeto
"network": { "name": "my-network", "external": true }
```

Mismo patron de unmarshalling polimorfico. Agrega complejidad a la generacion
de compose y validacion.

### SourceConfig: 4 variantes (git, image, local, command)

Cada variante tiene campos distintos y flujos de procesamiento diferentes en
`raioz up`. El paquet `upcase/` maneja esta complejidad con funciones
especializadas por tipo de source.

---

## 6. Areas de Mejora Identificadas

### 6.1 Acoplamiento de docker/ con workspace/

`docker/` importa `workspace/` para resolver rutas de montaje. Idealmente,
las rutas deberian resolverse en la capa de aplicacion y pasarse como
parametros simples. Esto reduciria el acoplamiento sin necesidad de interfaces
adicionales.

### 6.2 validate/ importa docker/

La validacion de compatibilidad necesita conocer reglas de Docker (puertos
validos, nombres de red, etc.). Seria mas limpio que `docker/` exponga
funciones de validacion puras y `validate/` las consuma, en lugar de importar
el paquete completo.

### 6.3 Interfaces potencialmente grandes

Algunas interfaces en `domain/interfaces/` tienen muchos metodos. Si crecen
mas, aplicar Interface Segregation (dividir en interfaces mas pequenas y
componerlas) mejoraria la flexibilidad.

Ejemplo de composicion:

```go
type GitCloner interface {
    Clone(repo, branch, target string) error
}

type GitUpdater interface {
    Pull(repoPath, branch string) error
}

type GitRepository interface {
    GitCloner
    GitUpdater
}
```

### 6.4 Separacion de generacion y ejecucion en docker/

Actualmente `docker/` mezcla generacion de compose (puro, testeable) con
ejecucion de comandos (side effects). Separar en `docker/compose/` (generacion)
y `docker/runner/` (ejecucion) haria el codigo mas testeable.

### 6.5 Ausencia de sistema de eventos

Para flujos complejos como `raioz up`, un sistema de eventos ligero permitiria
desacoplar la orquestacion del reporting de progreso. Evaluar solo si la
complejidad de reporting crece significativamente.

---

## 7. Metricas y Limites

| Metrica                    | Valor actual            | Limite           |
|----------------------------|-------------------------|------------------|
| Lineas por archivo         | Variable, max ~400      | 400 (sin tests)  |
| Caracteres por linea       | Variable                | 120              |
| Cobertura de tests         | >= 80%                  | 80% minimo       |
| Interfaces de dominio      | 9                       | -                |
| Archivos en upcase/        | 19                      | -                |
| Claves i18n                | 503+                    | Sincronizadas    |

### Complejidad por area

| Paquete      | Acoplamiento | Cohesion | Notas                              |
|--------------|--------------|----------|------------------------------------|
| `docker/`    | Alto         | Media    | Muchas responsabilidades, bien dividido en archivos |
| `config/`    | Bajo (hub)   | Alta     | Tipos bien definidos, muchos dependientes |
| `validate/`  | Medio        | Media    | Tres niveles de validacion mezclados |
| `state/`     | Bajo         | Alta     | Responsabilidad clara              |
| `workspace/` | Bajo         | Alta     | Responsabilidad clara              |
| `app/`       | Bajo         | Alta     | Solo interfaces, bien desacoplado  |
| `errors/`    | Ninguno      | Alta     | Utilidad independiente             |

---

## 8. Principios SOLID - Estado Actual

| Principio                    | Estado   | Detalle                                                    |
|------------------------------|----------|------------------------------------------------------------|
| Single Responsibility (SRP)  | Bueno    | Archivos pequenos, limite de 400 lineas ayuda              |
| Open/Closed (OCP)            | Parcial  | Agregar source kinds requiere tocar multiples archivos     |
| Liskov Substitution (LSP)    | Bueno    | Interfaces bien definidas, mocks sustituyen sin problemas  |
| Interface Segregation (ISP)  | Parcial  | Interfaces podrian ser mas granulares                      |
| Dependency Inversion (DIP)   | Bueno    | Capas respetan la regla de dependencia via interfaces      |

---

## 9. Referencias Arquitectonicas

- [Clean Architecture - Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture - Alistair Cockburn](https://alistair.cockburn.us/hexagonal-architecture/)
- [Strangler Fig Pattern - Martin Fowler](https://martinfowler.com/bliki/StranglerFigApplication.html)
- [SOLID Principles in Go - Dave Cheney](https://dave.cheney.net/2016/08/20/solid-go-design)
