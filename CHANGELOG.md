# Changelog

Todos los cambios notables de este proyecto se documentan aquí.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/).

## [Unreleased] — v0.9.0-beta

### Added
- **`raioz exec`** — Ejecutar comandos dentro de contenedores de servicio en ejecución
  - Soporte para flags del subcomando (`psql -U postgres`, `mongosh --eval "..."`)
  - Detección de servicios host con error claro y sugerencia
  - Búsqueda en compose generado y ProjectComposePath
  - Flag `--interactive` / `-i` para control de TTY
- **`raioz restart`** — Reiniciar servicios selectivamente
  - `--all` para reiniciar todos, `--include-infra` para incluir infraestructura
  - `--force-recreate` para recrear contenedores desde cero
  - Detección de servicios host y búsqueda en ProjectComposePath
- **`raioz volumes`** — Gestión granular de volúmenes del proyecto
  - `volumes list` — Listar volúmenes con origen (servicio/infra) y uso compartido
  - `volumes remove [nombre]` — Eliminar volúmenes específicos
  - `volumes remove --all` — Eliminar todos los volúmenes del proyecto
  - Protección de volúmenes compartidos con otros proyectos
- **`raioz doctor`** — Diagnóstico del entorno de desarrollo
  - Verificación de Docker, Docker Compose, Git
  - Control de espacio en disco
  - Verificación de directorio de configuración
  - Información del sistema operativo
- **`raioz up --only`** — Levantamiento parcial con resolución de dependencias
  - Resolución transitiva del grafo de dependencias
  - Si `api` depende de `postgres`, `--only api` levanta ambos
  - Compatible con `--profile`
- **Seed de datos en infra** — Campo `seed` en configuración de infraestructura
  - Monta archivos en `/docker-entrypoint-initdb.d/` automáticamente
  - Soporta PostgreSQL, MySQL, MariaDB, MongoDB
  - Paths relativos al directorio del `.raioz.json`
- **Suite de integración E2E** — 24 tests contra Docker real
  - Script `scripts/integration-test.sh` ejecutable localmente
  - Job `integration-docker` en GitHub Actions CI
  - Cubre: up, down, exec, volumes, doctor, host services, ProjectComposePath
- **Ejemplo `13-project-compose`** — Proyecto con su propio docker-compose.yml
- **Documentación**
  - `docs/GETTING_STARTED.md` — Guía paso a paso para nuevos usuarios
  - `docs/SCHEMA_REFERENCE.md` — Referencia completa de `.raioz.json`
  - `docs/ROADMAP.md` — Plan de desarrollo con features futuras
  - `docs/FEATURE_WATCH.md` — Spec de hot-reload
  - `docs/FEATURE_SNAPSHOT.md` — Spec de snapshot/restore
  - `docs/FEATURE_GRAPH.md` — Spec de visualización de dependencias
  - `docs/FEATURE_TUI.md` — Spec de dashboard interactivo
  - `docs/FEATURE_TUNNEL.md` — Spec de tunneling

### Changed
- **Errores estructurados** — Migración de `fmt.Errorf` a `errors.New` con códigos, contexto e i18n en:
  - `logs.go`, `clean.go`, `override.go`, `ignore.go`, `link.go`
  - `workspace_cmd.go`, `ports.go`, `list.go`, `status.go`
  - `exec.go`, `restart.go`, `volumes.go`
- **`docs/COMMANDS.md`** — Actualizado con documentación de `restart`, `exec` y `volumes`
- **`README.md`** — Agregada sección de documentación con links

### Fixed
- **55 errores de compilación en tests** — Corregidos en 6 paquetes:
  - `internal/testing` — `Infra` → `InfraEntry`
  - `internal/config` — `DockerConfig` → `*DockerConfig`, `Network` removido de `Project`
  - `internal/docker` — Tipos actualizados, lógica de `mode_test` corregida
  - `internal/git` — `context.Context` agregado a funciones
  - `internal/validate` — Tipos y campos actualizados
  - `internal/root` — Tipos actualizados
- **`filter_test.go`** — Test de perfil inválido usaba nombre válido (`"invalid"` → `"INVALID_PROFILE!"`)
- **`network_test.go`** — Estado JSON usaba estructura vieja de network
- **`mode_test.go`** — Test de prod esperaba eliminación de bind mounts (comportamiento cambió)

## [0.8.0] — Refactoring arquitectónico

### Added
- Internacionalización (i18n) con soporte para inglés y español (503 keys)
- Detección automática de idioma del sistema
- `raioz lang set/list` para gestión de idioma
- `make check-i18n` para validar catálogos en sync
- Tests unitarios para capa de use cases
- Errores estructurados con `RaiozError` (códigos, contexto, sugerencias)
- Domain interfaces para todas las dependencias

### Changed
- Migración a Clean Architecture: `cmd/` → `internal/app/` → `internal/domain/` → `internal/infra/`
- Business logic extraída de cobra commands a use cases
- Dependency injection via `Dependencies` struct
- Todos los mensajes de usuario a través de `i18n.T()`
- Output estandarizado via `internal/output/`

## [0.7.0] — Features de operación

### Added
- `raioz check` — Detección de drift entre config y estado
- `raioz compare` — Comparación con producción
- `raioz ci` — Modo CI/CD optimizado con output JSON
- `raioz migrate` — Conversión de docker-compose.yml a .raioz.json
- `raioz override` — Sobreescritura de servicios con ruta local
- `raioz ignore` — Ignorar servicios durante up
- `raioz link` — Symlinks para edición externa
- `raioz workspace` — Gestión de workspaces múltiples
- `raioz health` — Verificación de salud del proyecto local
- `raioz clean` — Limpieza de recursos Docker no utilizados

## [0.6.0] — Core

### Added
- `raioz up` — Levantamiento completo de servicios e infraestructura
- `raioz down` — Detención de servicios
- `raioz status` — Estado detallado de servicios
- `raioz logs` — Visualización de logs
- `raioz ports` — Listado de puertos activos
- `raioz list` — Listado de proyectos activos
- `raioz init` — Wizard interactivo para crear `.raioz.json`
- `raioz version` — Información de versión
- Soporte para 4 tipos de servicio: git, image, local, command
- Docker Compose generation automática
- State management con `.state.json`
- Detección de conflictos de puertos
- Healthcheck automático para infra común
- Modos dev/prod con configuraciones diferenciadas

## [0.1.0] — 2024

### Added
- Commit inicial
- Estructura básica del proyecto
- Build con goreleaser
