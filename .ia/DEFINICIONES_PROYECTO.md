# Definiciones del Proyecto Raioz

Este documento contiene las definiciones, decisiones de diseño y contexto del proyecto Raioz para agentes IA.

## 🎯 Visión General

**Raioz Local Orchestrator** es una herramienta CLI interna que permite levantar, coordinar y mantener entornos de desarrollo local para proyectos basados en microservicios, a partir de una configuración declarativa (`.raioz.json`, anteriormente `deps.json` para retrocompatibilidad).

### Propósito Principal

Eliminar la fricción entre desarrollo y arquitectura, haciendo que trabajar con microservicios localmente sea tan simple como trabajar con un monolito.

### Objetivo Final

Onboarding en un solo comando: `raioz up`

## 📐 Principios de Diseño

### 1. No Invasión

- Los microservicios NO saben que Raioz existe
- Los microservicios siguen siendo autónomos
- No se modifican repositorios de microservicios
- No se copian archivos dentro de microservicios
- No se requiere modificar Dockerfiles existentes

### 2. Binario Único

- Se instala una sola vez por máquina
- Instalación: `curl -fsSL https://raioz.dev/install | sh`
- No requiere repositorio del orquestador
- No requiere dependencias adicionales

### 3. Configuración Declarativa

- Cada proyecto tiene UN archivo: `.raioz.json` (soporta `deps.json` para retrocompatibilidad)
- El archivo vive con el proyecto, se versiona
- Se revisa en PRs
- Nada más se necesita

### 4. Workspace Centralizado

- Base: `/opt/raioz-proyecto/` (o `~/.raioz/` si no hay permisos)
- Estructura:
  ```
  /opt/raioz-proyecto/
  ├── workspaces/
  │   └── {project-name}/
  │       ├── .state.json
  │       └── docker-compose.generated.yml
  ├── services/
  │   └── {service-path}/  # Repos clonados aquí
  └── env/
      ├── global.env
      ├── services/
      │   └── {service-name}.env
      └── projects/
          └── {project-name}.env
  ```

### 5. Transparencia

- `docker-compose.generated.yml` es legible y ejecutable sin Raioz
- No hay lock-in fuerte
- El estado se guarda pero es recuperable

## 🏗️ Arquitectura

### Estructura del Proyecto

```
raioz/
├── cmd/                    # Comandos CLI (Cobra)
│   ├── root.go            # Comando raíz
│   ├── up.go              # Comando up
│   ├── down.go            # Comando down
│   ├── status.go          # Comando status
│   ├── list.go            # Comando list
│   ├── workspace.go       # Comando workspace
│   └── ...                # Otros comandos
├── internal/
│   ├── app/               # Capa de aplicación (casos de uso)
│   │   ├── down.go        # DownUseCase
│   │   ├── status.go      # StatusUseCase
│   │   └── dependencies.go # Container de dependencias
│   ├── domain/            # Capa de dominio
│   │   └── interfaces/    # Interfaces (puertos)
│   │       ├── docker.go
│   │       ├── git.go
│   │       ├── workspace.go
│   │       └── ...
│   ├── infra/             # Capa de infraestructura
│   │   ├── docker/        # Implementación Docker
│   │   ├── git/           # Implementación Git
│   │   ├── workspace/     # Implementación Workspace
│   │   └── ...
│   ├── config/            # Configuración y schema
│   │   ├── deps.go        # Estructuras y carga
│   │   └── schema.go      # JSON Schema
│   ├── workspace/         # Gestión de workspace (modelos)
│   ├── git/               # Operaciones Git (lógica)
│   ├── docker/            # Docker Compose (lógica)
│   ├── env/               # Variables de entorno
│   ├── validate/          # Validación
│   ├── state/             # Estado persistente
│   ├── lock/              # Sistema de locks
│   ├── root/              # Gestión de raioz.root.json
│   ├── override/          # Sistema de overrides
│   ├── ignore/            # Sistema de ignore
│   ├── link/              # Comando link (symlinks)
│   ├── audit/             # Audit log
│   ├── resilience/        # Retry y circuit breakers
│   └── ...                # Otros módulos
├── docs/                  # Documentación
│   ├── casos-de-uso/      # Casos de uso documentados
│   ├── limits.md          # Límites y decisiones
│   └── ...
├── .ia/                   # Documentación para agentes IA
├── main.go
└── go.mod
```

### Flujo de Ejecución: `raioz up`

1. **Cargar configuración**: Lee `.raioz.json` (o `deps.json` para retrocompatibilidad)
2. **Validar**: JSON Schema + validaciones de negocio
3. **Resolver workspace**: Crea/verifica estructura de directorios
4. **Adquirir lock**: Previene ejecuciones concurrentes
5. **Clonar repos Git**: Solo los servicios con `source.kind == "git"`
6. **Resolver variables de entorno**: Carga archivos .env según configuración
7. **Generar docker-compose**: Crea `docker-compose.generated.yml`
8. **Ejecutar Docker**: `docker compose up -d`
9. **Guardar estado**: Persiste configuración actual

### Flujo de Ejecución: `raioz down`

1. **Resolver workspace**: Obtiene workspace del proyecto
2. **Adquirir lock**: Previene ejecuciones concurrentes
3. **Verificar estado**: Confirma que el proyecto está corriendo
4. **Detener servicios**: `docker compose down`
5. **Limpiar estado**: Elimina `.state.json`
6. **Liberar lock**

### Flujo de Ejecución: `raioz status`

1. **Resolver workspace**: Obtiene workspace del proyecto
2. **Verificar estado**: Lee `.state.json`
3. **Consultar Docker**: Obtiene estado de contenedores
4. **Mostrar información**: Tabla con servicios y estado

## 📄 Formato de `.raioz.json`

### Estructura Base

```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "project-name",
    "network": "network-name"
  },
  "env": {
    "useGlobal": true,
    "files": ["global", "projects/project-name"]
  },
  "services": {
    "service-name": {
      "source": {
        "kind": "git|image",
        // Si git:
        "repo": "git@github.com:org/repo.git",
        "branch": "branch-name",
        "path": "services/service-name"
        // Si image:
        // "image": "org/image",
        // "tag": "1.0.0"
      },
      "docker": {
        "mode": "dev|prod",
        "ports": ["3000:3000"],
        "volumes": ["volume:path"],
        "dependsOn": ["other-service"],
        "dockerfile": "Dockerfile.dev"
      },
      "env": ["services/service-name"],
      "profiles": ["frontend", "backend"]
    }
  },
  "infra": {
    "infra-name": {
      "image": "image-name",
      "tag": "tag",
      "ports": ["5432:5432"],
      "volumes": ["volume:path"],
      "env": ["infra/infra-name"]
    }
  }
}
```

### Validaciones

#### Schema JSON

- Validación estricta con JSON Schema
- Versión de schema debe ser "1.0"
- Campos requeridos según tipo de source

#### Validaciones de Negocio

- Proyecto: name y network requeridos
- Servicios: al menos uno requerido
- Source git: requiere repo, branch, path
- Source image: requiere image, tag
- Dependencias: verificar que existen
- Profiles: solo "frontend" o "backend"

## 🔐 Variables de Entorno

### Estructura

```
/opt/raioz-proyecto/env/
├── global.env                    # Variables globales
├── services/
│   ├── users-service.env
│   └── payments-service.env
└── projects/
    └── billing-dashboard.env
```

### Precedencia

1. `global.env` (si `useGlobal: true`)
2. Archivos de proyecto (según `env.files`)
3. Archivos de servicio (según `service.env` o `infra.env`)

### Resolución

- Se combinan múltiples archivos en uno temporal
- Archivo temporal se guarda en workspace: `.env.{service-name}`
- Se referencia en docker-compose como `env_file`

## 🐳 Docker Compose

### Generación

- Se genera dinámicamente desde `.raioz.json`
- Se guarda en: `{workspace}/docker-compose.generated.yml`
- Formato: YAML 3.9
- Red: externa (se crea antes si no existe)

### Servicios Git

- Build context: `{services-dir}/{path}`
- Dockerfile: especificado en config
- Puede usar wrapper temporal si no hay Dockerfile.dev

### Servicios Image

- Image: `{image}:{tag}`
- Sin build context

### Infraestructura

- Siempre image-based
- Pueden tener variables de entorno

## 🔒 Sistema de Locks

### Propósito

Prevenir ejecuciones concurrentes del mismo proyecto.

### Implementación

- Archivo: `.raioz.lock` en workspace root
- Contenido: PID y timestamp
- Adquisición: exclusiva (O_CREAT | O_EXCL)
- Liberación: al finalizar comando (defer)

### Comportamiento

- Si lock existe: error claro indicando otro proceso
- Lock se libera automáticamente al terminar

## 💾 Estado Persistente

### Archivo: `.state.json`

- Ubicación: `{workspace}/.state.json`
- Contenido: copia de `.raioz.json` usado
- Propósito: comparar cambios, detectar drift

### Uso

- Se guarda después de `raioz up` exitoso
- Se usa en `raioz status` para listar servicios
- Se compara en `raioz check` para detectar cambios
- Se elimina en `raioz down`

## 🌿 Git Operations

### Clonado

- Solo si `source.kind == "git"`
- Ubicación: `{services-dir}/{path}`
- Branch: especificado en config
- Si repo existe: no clona (asume correcto)

### Actualización (TODO)

- Verificar branch actual
- Si cambió: hacer checkout
- Pull si es necesario

### Drift Detection (TODO)

- Comparar branch actual vs esperado
- Advertir (no forzar) si hay diferencia
- Permitir trabajar en otra rama

## 🎨 Profiles

### Propósito

Filtrar servicios según perfil (frontend/backend).

### Uso

```bash
raioz up --profile frontend
raioz up --profile backend
```

### Lógica

- Servicios sin profiles: siempre incluidos
- Servicios con profiles: incluidos si coinciden
- Infra: siempre incluida

## 🔍 Validaciones Importantes

### Antes de `up`

1. ✅ Validar schema JSON
2. ✅ Validar lógica de negocio
3. ✅ Verificar puertos no ocupados (TODO)
4. ✅ Verificar red Docker existe (TODO)
5. ✅ Verificar imágenes Docker (TODO)

### Durante `up`

1. ✅ Adquirir lock
2. ✅ Clonar repos necesarios
3. ✅ Resolver variables de entorno
4. ✅ Generar compose
5. ✅ Ejecutar Docker
6. ✅ Guardar estado

## 🚫 Restricciones y Limitaciones

### NO se debe hacer

- ❌ Clonar repositorio del orquestador
- ❌ Copiar archivos dentro de microservicios
- ❌ Modificar Dockerfiles existentes
- ❌ Requerir tocar .env en cada repo
- ❌ Imponer estructura interna en servicios
- ❌ Forzar cambios en ramas Git

### SI se debe hacer

- ✅ Crear workspace fuera de repos
- ✅ Clonar solo repos necesarios
- ✅ Usar imágenes versionadas cuando es posible
- ✅ Centralizar variables de entorno
- ✅ Generar compose legible y ejecutable
- ✅ Advertir sobre cambios, no forzar

## 🔄 Idempotencia

### Principio

`raioz up` debe ser seguro ejecutar múltiples veces.

### Implementación

- Verificar si servicios ya están corriendo
- Comparar configuración actual vs estado
- Solo recrear si hay cambios significativos
- No forzar si estado es correcto

## 📊 Casos de Uso Principales

### 1. Onboarding Nuevo Dev

```
git clone project-repo
cd project
raioz up
```

Tiempo: 5-10 minutos

### 2. Cambiar de Proyecto

```
cd otro-proyecto
raioz up
```

Sin conflictos, workspace separado

### 3. Agregar Servicio

Editar `.raioz.json`, agregar servicio, commit, PR.
Otros devs: `raioz up` para actualizar.

### 4. Desarrollo en Rama Diferente

```
cd /opt/raioz-proyecto/services/service
git checkout feature/x
# Raioz detecta drift pero no fuerza
```

## 🔧 Extensiones Futuras

### Funcionalidades Implementadas

- ✅ Variables de entorno centralizadas
- ✅ Profiles (frontend/backend)
- ✅ Sistema de locks
- ✅ Detección de conflictos de puertos
- ✅ Validación de imágenes Docker
- ✅ Actualización automática de repos (con detección de drift)
- ✅ Modo readonly para repositorios Git
- ✅ Modo disabled para servicios
- ✅ Mocks y feature flags
- ✅ Integración con CI (`raioz ci`)
- ✅ Validación de compatibilidad entre servicios
- ✅ Sistema de override explícito
- ✅ Resolución asistida de dependencias
- ✅ Archivo `raioz.root.json` para trazabilidad
- ✅ Comando workspace
- ✅ Audit log
- ✅ Sistema de ignore
- ✅ Comando link (symlinks)
- ✅ Logging estructurado
- ✅ Context y timeouts
- ✅ Dependency Injection (parcial)
- ✅ Separación de capas arquitectónicas (Clean Architecture)

### Pendientes (Prioridad Media-Baja)

- ⏳ Migración completa de comandos a capa de aplicación
- ⏳ Aumentar cobertura de tests a 90%+
- 🔲 Paridad con Kubernetes (futuro)

## 📚 Referencias de Diseño

### Documentos Clave

- `README.md`: Documentación principal del proyecto
- `project.md`: Visión y objetivos del proyecto
- `como-funciona.md`: Funcionamiento esperado desde perspectiva del usuario
- `caso-real.md`: Casos de uso reales y validaciones
- `TODO.md`: Tareas pendientes y plan de desarrollo
- `docs/limits.md`: Límites y decisiones conscientes
- `docs/casos-de-uso/`: Casos de uso documentados

### Decisiones Importantes

1. **Workspace centralizado**: Facilita compartir servicios entre proyectos
2. **Estado persistente**: Permite detectar cambios y drift
3. **Locks**: Previene corrupción por ejecuciones concurrentes
4. **Compose generado**: Transparencia y debuggeabilidad
5. **No invasión**: Los microservicios siguen siendo independientes

## 🤖 Notas para Agentes IA

### Al trabajar en este proyecto:

1. **Revisar este archivo primero** para entender contexto
2. **Seguir estándares de código** en `.ia/CODIGO_STANDARDS.md`
3. **Respetar principios de diseño** (no invasión, transparencia)
4. **Verificar TODO.md** para prioridades
5. **Mantener coherencia** con decisiones existentes
6. **Seguir arquitectura Clean Architecture** (domain, app, infra, cmd)
7. **Usar Dependency Injection** para nuevas funcionalidades
8. **Escribir tests** para nueva funcionalidad
9. **Actualizar documentación** cuando corresponda

### Patrones Importantes

- Manejo de errores: usar `errors.New()` con códigos y contexto (ver `internal/errors`)
- Validación temprana: validar antes de ejecutar
- Mensajes claros: errores descriptivos con sugerencias para usuarios
- Testing: tests unitarios para lógica compleja, tests de integración para flujos
- Modularidad: un archivo, un propósito, máximo 400 líneas
- Logging: usar `log/slog` para logging estructurado
- Context: propagar `context.Context` para timeouts y cancelación
- Dependency Injection: inyectar dependencias vía interfaces del dominio
