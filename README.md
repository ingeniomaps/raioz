# Raioz Local Orchestrator

[![CI](https://github.com/ingeniomaps/raioz/workflows/CI/badge.svg)](https://github.com/ingeniomaps/raioz/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/ingeniomaps/raioz/branch/main/graph/badge.svg)](https://codecov.io/gh/ingeniomaps/raioz)
[![Go Report Card](https://goreportcard.com/badge/github.com/ingeniomaps/raioz)](https://goreportcard.com/report/github.com/ingeniomaps/raioz)

**Raioz** es una herramienta CLI interna que permite levantar, coordinar y mantener entornos de desarrollo local para proyectos basados en microservicios, a partir de una configuración declarativa (`.raioz.json`).

## 🎯 Propósito

Eliminar la fricción entre desarrollo y arquitectura, haciendo que trabajar con microservicios localmente sea tan simple como trabajar con un monolito.

**Onboarding en un solo comando:** `raioz up`

## ✨ Características

- 🚀 **Configuración declarativa**: Un solo archivo `.raioz.json` por proyecto
- 🔄 **Idempotente**: Seguro de ejecutar múltiples veces
- 🔒 **Locks**: Previene ejecuciones concurrentes
- 🌿 **Git integrado**: Clona y gestiona repositorios automáticamente
- 🐳 **Docker Compose**: Genera y ejecuta automáticamente
- 📦 **Modos Dev/Prod**: Diferentes configuraciones según el modo
- 🔍 **Validación**: JSON Schema + validaciones de negocio
- 📊 **Estado persistente**: Detecta cambios y desalineaciones
- 🧹 **Limpieza**: Comando para limpiar recursos no usados

## ⚠️ Limitaciones Conocidas

Raioz está diseñado para ser simple y predecible. Para entender qué **NO** hace Raioz, qué casos no soporta, y las decisiones conscientes de diseño, consulta el documento de límites:

📖 **[Ver Documento de Límites](./docs/limits.md)**

Este documento cubre:

- Qué Raioz NO hace (Kubernetes, Docker Swarm, gestión de secrets nativa, etc.)
- Casos no soportados (servicios con privilegios especiales, Windows containers, etc.)
- Decisiones conscientes de diseño y sus razones
- Alternativas y workarounds para casos avanzados

## 📦 Instalación

### Instalación rápida (recomendada)

```bash
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
```

El script de instalación automáticamente:

- Detecta tu sistema operativo (Linux/macOS) y arquitectura
- Descarga el binario pre-compilado desde GitHub releases
- Instala raioz en `/usr/local/bin` (requiere sudo)
- Verifica que Docker y Git estén instalados
- Configura los permisos necesarios

### Instalación manual

Si prefieres instalar manualmente:

```bash
# Descargar el binario para tu plataforma desde GitHub releases
# Luego moverlo a un directorio en tu PATH:
sudo mv raioz /usr/local/bin/
sudo chmod +x /usr/local/bin/raioz
```

````

### Instalación manual

```bash
# Descargar el binario para tu OS/arch
# Linux amd64
wget https://github.com/ingeniomaps/raioz/releases/latest/download/raioz-linux-amd64 -O /usr/local/bin/raioz
chmod +x /usr/local/bin/raioz

# macOS amd64
curl -L https://github.com/ingeniomaps/raioz/releases/latest/download/raioz-darwin-amd64 -o /usr/local/bin/raioz
chmod +x /usr/local/bin/raioz
````

### Compilar desde fuente

```bash
git clone https://github.com/ingeniomaps/raioz.git
cd raioz
make build
sudo make install
```

### Verificar instalación

```bash
raioz --help
raioz version
```

## 🚀 Quick Start

1. **Crear archivo `.raioz.json`** en la raíz de tu proyecto:

```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "my-project",
    "network": "my-network"
  },
  "env": {
    "useGlobal": true,
    "files": ["global"]
  },
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/api.git",
        "branch": "main",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "dependsOn": ["database"]
      },
      "env": ["services/api"]
    }
  },
  "infra": {
    "database": {
      "image": "postgres",
      "tag": "15",
      "ports": ["5432:5432"],
      "volumes": ["postgres-data:/var/lib/postgresql/data"]
    }
  }
}
```

2. **Levantar el proyecto**:

```bash
raioz up
```

3. **Verificar estado**:

```bash
raioz status
```

4. **Ver logs**:

```bash
raioz logs api
raioz logs --follow --all
```

5. **Detener el proyecto**:

```bash
raioz down
```

## 📖 Guía de Uso

### Comandos disponibles

#### `raioz up`

Levanta todos los servicios del proyecto.

```bash
# Usar .raioz.json por defecto
raioz up

# Especificar archivo de configuración
raioz up --config custom-raioz.json

# Usar un perfil específico
raioz up --profile frontend
raioz up --profile backend

# Forzar re-clonado de repositorios
raioz up --force-reclone
```

**Qué hace:**

- Valida la configuración
- Clona repositorios Git necesarios
- Resuelve variables de entorno
- Genera `docker-compose.generated.yml`
- Levanta servicios con Docker Compose
- Guarda el estado del proyecto

#### `raioz down`

Detiene todos los servicios del proyecto.

```bash
raioz down
raioz down --project my-project
```

**Qué hace:**

- Detiene servicios con Docker Compose
- Limpia archivos de estado
- Deja redes y volúmenes para reutilización

#### `raioz status`

Muestra el estado detallado de los servicios.

```bash
raioz status
raioz status --json  # Output en formato JSON
```

**Información mostrada:**

- Estado (running/stopped)
- Health status (healthy/unhealthy/starting)
- Uptime
- Uso de recursos (CPU, memoria)
- Versión/commit
- Última actualización

#### `raioz list`

Lista todos los proyectos activos desde el estado global.

```bash
raioz list
raioz list --json  # Output en formato JSON
```

**Información mostrada:**

- Nombre del proyecto
- Ruta del workspace
- Última ejecución (formato relativo: "2 hours ago", "3 days ago", etc.)
- Cantidad de servicios activos
- Cantidad de servicios corriendo

**Ejemplo de salida:**

```
Active Projects:

Project: billing-platform
  Workspace: /opt/raioz-proyecto/workspaces/billing-platform
  Last Execution: 2 hours ago
  Active Services: 5
  Running: 5/5

Project: auth-service
  Workspace: /opt/raioz-proyecto/workspaces/auth-service
  Last Execution: 1 day ago
  Active Services: 3
  Running: 2/3
```

#### `raioz logs`

Muestra logs de servicios.

```bash
# Ver logs de un servicio
raioz logs api

# Ver logs de múltiples servicios
raioz logs api frontend

# Ver logs de todos los servicios
raioz logs --all

# Seguir logs en tiempo real
raioz logs --follow api

# Ver últimas 100 líneas
raioz logs --tail 100 api
```

#### `raioz ports`

Lista todos los puertos activos.

```bash
raioz ports
```

#### `raioz check`

Verifica alineación entre configuración y estado.

```bash
raioz check
```

**Detecta:**

- Cambios de configuración
- Drift de ramas (cambios manuales)
- Cambios de versiones
- Desalineaciones

#### `raioz workspace`

Gestiona workspaces para organizar múltiples proyectos.

```bash
# Cambiar workspace activo
raioz workspace use empresa-x

# Listar workspaces disponibles
raioz workspace list
```

#### `raioz override`

Sobrescribe un servicio para usar una ruta local.

```bash
# Sobrescribir servicio con ruta local
raioz override api --path ~/dev/api

# Listar overrides
raioz override list

# Eliminar override
raioz override remove api
```

#### `raioz ignore`

Gestiona servicios ignorados durante resolución de dependencias.

```bash
# Ignorar un servicio
raioz ignore add legacy-service

# Listar servicios ignorados
raioz ignore list

# Dejar de ignorar
raioz ignore remove legacy-service
```

#### `raioz link`

Gestiona symlinks para edición externa de servicios.

```bash
# Crear symlink
raioz link add api ~/dev/api

# Listar symlinks
raioz link list

# Eliminar symlink
raioz link remove api
```

#### `raioz compare`

Compara configuración local con producción.

```bash
raioz compare docker-compose.prod.yml
```

#### `raioz migrate`

Convierte Docker Compose a formato `.raioz.json`.

```bash
raioz migrate docker-compose.yml
```

#### `raioz version`

Muestra información de versión.

```bash
raioz version
```

#### `raioz clean`

Limpia recursos no usados.

```bash
# Limpiar proyecto actual
raioz clean

# Limpiar todos los proyectos
raioz clean --all

# Limpiar imágenes no usadas
raioz clean --images

# Limpiar volúmenes no usados (requiere confirmación)
raioz clean --volumes

# Limpiar redes no usadas
raioz clean --networks

# Dry-run (ver qué se limpiaría)
raioz clean --dry-run

# Limpiar todo con confirmación
raioz clean --all --images --volumes --networks
```

#### `raioz ci`

Comando optimizado para CI/CD con validaciones rápidas y output en JSON.

```bash
# Ejecutar CI con validaciones y setup completo
raioz ci

# Solo validaciones (no levanta servicios)
raioz ci --only-validate

# Validaciones y compose, sin build
raioz ci --skip-build

# Saltar pull de imágenes
raioz ci --skip-pull

# Usar entorno efímero (auto-cleanup)
raioz ci --ephemeral

# Entorno efímero con job ID específico
raioz ci --ephemeral --job-id $CI_JOB_ID

# Mantener entorno efímero después (para debugging)
raioz ci --ephemeral --keep

# Configuración personalizada
raioz ci --config custom-raioz.json
```

**Características:**

- Output siempre en formato JSON (parseable)
- Validaciones rápidas (solo checks críticos)
- Entornos efímeros con limpieza automática
- Exit code 0 si éxito, 1 si falla
- Validaciones paso a paso con estado (passed/failed/skipped)

**Output JSON:**

```json
{
  "success": true,
  "startTime": "2024-01-01T12:00:00Z",
  "endTime": "2024-01-01T12:05:00Z",
  "duration": 300.5,
  "message": "CI run completed successfully",
  "workspace": "my-project-ci-123456",
  "composeFile": "/path/to/docker-compose.generated.yml",
  "stateFile": "/path/to/.state.json",
  "services": ["api", "frontend"],
  "infra": ["database"],
  "validations": [
    {
      "check": "preflight",
      "status": "passed"
    },
    {
      "check": "load_config",
      "status": "passed"
    },
    {
      "check": "validation",
      "status": "passed"
    }
  ],
  "warnings": [],
  "errors": []
}
```

## 📚 Documentación Adicional

- **[Guía Completa de Comandos](./docs/COMMANDS.md)** - Documentación detallada de todos los comandos con ejemplos
- **[Guía de Troubleshooting](./docs/TROUBLESHOOTING.md)** - Solución de problemas comunes
- **[Documento de Límites](./docs/limits.md)** - Qué Raioz NO hace y casos no soportados

### Estructura de `.raioz.json`

#### Campos principales

**`schemaVersion`** (requerido)

- Versión del schema JSON Schema
- Debe ser `"1.0"`

**`project`** (requerido)

- `name`: Nombre del proyecto (a-z, 0-9, -)
- `network`: Nombre de la red Docker (a-z, 0-9, -)

**`env`** (requerido)

- `useGlobal`: Si usar `global.env`
- `files`: Lista de archivos .env a cargar (relativos a `env/`)

**`services`** (requerido)

- Objeto con servicios del proyecto
- Al menos un servicio requerido

**`infra`** (requerido)

- Objeto con servicios de infraestructura (DB, cache, etc.)
- Puede estar vacío `{}`

#### Configuración de servicios

**`source`** (requerido)

- `kind`: Tipo de fuente (`"git"` o `"image"`)

Si `kind: "git"`:

- `repo`: URL del repositorio Git
- `branch`: Rama a usar
- `path`: Ruta donde clonar (relativo a `services/`)
- `access`: Modo de acceso (`"readonly"` o `"editable"`, por defecto `"editable"`)
  - `"readonly"`:
    - Repositorio protegido, no se actualiza automáticamente (no checkout/pull)
    - Volúmenes montados como read-only (`:ro`) - Docker impide escribir
    - Servicio descartable: se recrea automáticamente si hay problemas (`restart: unless-stopped`)
    - Útil para servicios de otros equipos que no debes modificar
  - `"editable"`:
    - Repositorio editable, se actualiza automáticamente (checkout/pull)
    - Volúmenes montados normalmente (read-write)
    - Útil para servicios que desarrollas activamente

Si `kind: "image"`:

- `image`: Nombre de la imagen Docker
- `tag`: Tag de la imagen

**`docker`** (requerido)

- `mode`: Modo de ejecución (`"dev"` o `"prod"`)
- `ports`: Lista de puertos `["host:container"]`
- `volumes`: Lista de volúmenes
- `dependsOn`: Lista de dependencias (otros servicios/infra)
- `dockerfile`: Nombre del Dockerfile (opcional)
- `command`: Comando a ejecutar (opcional, requiere `runtime`)
- `runtime`: Runtime (`node`, `go`, `python`, `java`, `rust`)

**`env`** (opcional)

- Lista de archivos .env a cargar para el servicio

**`profiles`** (opcional)

- Lista de perfiles (`"frontend"`, `"backend"`)

**`enabled`** (opcional)

- Boolean que habilita o deshabilita explícitamente el servicio
- Por defecto: `true` si no se especifica
- Servicios con `enabled: false` no se clonan, construyen ni inician
- Tiene precedencia sobre `featureFlag`

**`mock`** (opcional)

- Configuración para usar un servicio mock en lugar del real
- `enabled`: Si usar mock (boolean)
- `image`: Imagen Docker del mock (requerido si enabled=true)
- `tag`: Tag del mock (opcional)
- `ports`: Puertos del mock (opcional, sobrescribe puertos del servicio)
- `env`: Variables de entorno del mock (opcional)

**`featureFlag`** (opcional)

- Configuración de feature flag para habilitar/deshabilitar servicios
- `enabled`: Habilitar directamente (boolean)
- `disabled`: Deshabilitar directamente (boolean, tiene precedencia)
- `envVar`: Variable de entorno a verificar (string)
- `envValue`: Valor requerido de la variable (string, opcional)
- `profiles`: Perfiles donde está habilitado (array de strings)

#### Configuración de infra

**`image`** (requerido)

- Nombre de la imagen Docker

**`tag`** (opcional)

- Tag de la imagen

**`ports`** (opcional)

- Lista de puertos `["host:container"]`

**`volumes`** (opcional)

- Lista de volúmenes
- Formatos soportados:
  - **Named volumes**: `"volume-name:/container/path"` - Volúmenes con nombre que persisten datos
  - **Bind mounts**: `"./path:/container/path"` o `"/absolute/path:/container/path"` - Montajes de directorios del host
  - **Anonymous volumes**: `"/container/path"` - Volúmenes anónimos

**`env`** (opcional)

- Lista de archivos .env a cargar

#### Volúmenes Compartidos

Raioz detecta automáticamente cuando múltiples servicios comparten el mismo volumen con nombre (named volume) y muestra una advertencia informativa.

**Ejemplo de advertencia:**

```
⚠️  Warning: 1 volume(s) are shared between multiple services:

  • Volume 'shared-data' is used by: [service-a, service-b]

  ℹ️  Note: Shared volumes can be intentional (e.g., shared database data).
     Ensure that services are designed to handle concurrent access to shared volumes.
```

**Consideraciones:**

- Los volúmenes compartidos pueden ser intencionales (por ejemplo, datos compartidos de base de datos)
- Asegúrate de que los servicios estén diseñados para manejar acceso concurrente a volúmenes compartidos
- Los bind mounts no se detectan como compartidos (cada servicio puede tener su propio bind mount del mismo directorio)
- Esta advertencia es informativa y no impide que el proyecto se levante

### Servicios Deshabilitados

Los servicios con `enabled: false` están explícitamente deshabilitados y no se clonan, construyen ni inician.

**Características:**

- No se clonan repositorios Git
- No se incluyen en `docker-compose.generated.yml`
- No se inician contenedores
- Se muestran en `raioz status` como "disabled"
- Tiene precedencia sobre `featureFlag`

**Ejemplo:**

```json
{
  "services": {
    "old-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/old-service.git",
        "branch": "main",
        "path": "services/old-service"
      },
      "docker": {
        "mode": "dev",
        "ports": ["8080:8080"]
      },
      "enabled": false
    }
  }
}
```

### Ejemplo completo de `.raioz.json`

```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-dashboard",
    "network": "billing-network"
  },
  "env": {
    "useGlobal": true,
    "files": ["global", "projects/billing-dashboard"]
  },
  "services": {
    "frontend": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/frontend.git",
        "branch": "develop",
        "path": "services/frontend"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "dependsOn": ["api"],
        "command": "npm run dev",
        "runtime": "node"
      },
      "env": ["services/frontend"],
      "profiles": ["frontend"]
    },
    "api": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/api.git",
        "branch": "main",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["8080:8080"],
        "dependsOn": ["database", "redis"],
        "dockerfile": "Dockerfile.dev"
      },
      "env": ["services/api"],
      "profiles": ["backend"]
    },
    "payments-service": {
      "source": {
        "kind": "image",
        "image": "org/payments-service",
        "tag": "v1.2.3"
      },
      "docker": {
        "mode": "prod",
        "ports": ["8081:8080"],
        "dependsOn": ["database"]
      },
      "mock": {
        "enabled": true,
        "image": "org/payment-mock",
        "tag": "latest",
        "ports": ["8081:8080"]
      }
    },
    "experimental-service": {
      "source": {
        "kind": "image",
        "image": "org/experimental",
        "tag": "latest"
      },
      "docker": {
        "mode": "dev",
        "ports": ["8082:8080"]
      },
      "featureFlag": {
        "enabled": true,
        "profiles": ["backend"]
      }
    },
    "disabled-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/disabled.git",
        "branch": "main",
        "path": "services/disabled"
      },
      "docker": {
        "mode": "dev",
        "ports": ["8083:8080"]
      },
      "enabled": false
    }
  },
  "infra": {
    "database": {
      "image": "postgres",
      "tag": "15",
      "ports": ["5432:5432"],
      "volumes": ["postgres-data:/var/lib/postgresql/data"],
      "env": ["infra/database"]
    },
    "redis": {
      "image": "redis",
      "tag": "7-alpine",
      "ports": ["6379:6379"],
      "volumes": ["redis-data:/data"]
    }
  }
}
```

### Mocks y Feature Flags

#### Mocks

Los mocks permiten reemplazar servicios reales con imágenes mock para desarrollo o testing.

**Ejemplo:**

```json
{
  "services": {
    "payment-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/payment-service.git",
        "branch": "main",
        "path": "services/payment"
      },
      "docker": {
        "mode": "dev",
        "ports": ["8080:8080"]
      },
      "mock": {
        "enabled": true,
        "image": "org/payment-mock",
        "tag": "latest",
        "ports": ["8080:8080"]
      }
    }
  }
}
```

Cuando `mock.enabled` es `true`, el servicio real se reemplaza con la imagen mock especificada.

#### Feature Flags

Los feature flags permiten habilitar/deshabilitar servicios basándose en:

- Flags directos (`enabled`/`disabled`)
- Variables de entorno
- Perfiles

**Ejemplos:**

```json
{
  "services": {
    "experimental-service": {
      "source": {
        "kind": "image",
        "image": "org/service",
        "tag": "latest"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"]
      },
      "featureFlag": {
        "enabled": true,
        "profiles": ["backend"]
      }
    },
    "optional-service": {
      "source": {
        "kind": "image",
        "image": "org/optional",
        "tag": "latest"
      },
      "docker": {
        "mode": "dev"
      },
      "featureFlag": {
        "envVar": "ENABLE_OPTIONAL_SERVICE",
        "envValue": "true"
      }
    }
  }
}
```

**Prioridad de feature flags:**

1. `disabled: true` siempre deshabilita el servicio
2. `enabled: true` habilita (pero verifica `profiles` si está especificado)
3. `envVar` verifica variable de entorno
4. `profiles` habilita solo para perfiles especificados
5. Por defecto: habilitado (compatibilidad hacia atrás)

### Variables de Entorno

#### Estructura

Las variables de entorno se organizan en:

```
/opt/raioz-proyecto/env/  (o ~/.raioz/env/ si no hay permisos)
├── global.env                    # Variables globales
├── services/
│   ├── frontend.env
│   └── api.env
└── projects/
    └── billing-dashboard.env
```

#### Precedencia

1. `global.env` (si `useGlobal: true`)
2. Archivos de proyecto (según `env.files`)
3. Archivos de servicio (según `service.env`)

#### Ejemplo

**`env/global.env`**:

```bash
NODE_ENV=development
LOG_LEVEL=debug
```

**`env/services/api.env`**:

```bash
API_PORT=8080
DATABASE_URL=postgres://user:pass@database:5432/dbname
```

### Permisos de Archivos y Seguridad

Raioz aplica permisos restrictivos para proteger información sensible:

- **Archivos de lock** (`.raioz.lock`): Permisos `0600` (lectura/escritura solo para el propietario)
- **Archivos de estado** (`.state.json`, `state.json` global): Permisos `0600` (solo propietario)
- **Archivos de entorno** (`.env.*`, archivos combinados): Permisos `0600` (contienen secrets, solo propietario)
- **Directorios de workspace**: Permisos `0700` (acceso solo para el propietario)
  - `/opt/raioz-proyecto/workspaces/{project}/` (o `~/.raioz/workspaces/{project}/`)
  - `/opt/raioz-proyecto/env/` (o `~/.raioz/env/`)
  - Subdirectorios `local/`, `readonly/`, `services/`, `projects/`

**Nota**: El directorio base (`/opt/raioz-proyecto` o `~/.raioz`) puede tener permisos `0755` para permitir acceso compartido, pero todos los subdirectorios y archivos creados por Raioz tienen permisos restrictivos (`0700` para directorios, `0600` para archivos).

#### Sanitización de Secrets

Raioz incluye funciones de sanitización para prevenir la exposición de secrets en logs y mensajes de error:

- **Detección automática**: Identifica variables de entorno sensibles basándose en patrones comunes (PASSWORD, SECRET, TOKEN, KEY, API_KEY, etc.)
- **Redacción**: Valores sensibles se reemplazan con `***REDACTED***` antes de ser impresos
- **Disponible en código**: Las funciones de sanitización están disponibles en el paquete `env` para uso futuro en logging estructurado

Las funciones de sanitización están implementadas y listas para usar cuando se implemente logging estructurado o cuando se detecten casos específicos donde se impriman variables de entorno.

### Estructura de Workspace

Raioz organiza los servicios en una estructura de directorios que separa servicios editables de readonly:

```
{base}/workspaces/{project}/
├── local/          # Servicios editables (access: editable o sin access)
│   └── services/   # Repositorios Git editables
├── readonly/       # Servicios readonly (access: readonly)
│   └── services/   # Repositorios Git readonly
└── .state.json     # Estado del proyecto
```

**Migración automática:**

- Si tienes servicios en la estructura antigua (`{base}/services/`), Raioz los migra automáticamente a la nueva estructura
- La migración ocurre la primera vez que ejecutas `raioz up` después de actualizar
- Los servicios se mueven a `local/` o `readonly/` según su configuración de `access`

**Compatibilidad:**

- La estructura antigua sigue siendo compatible para servicios de imagen
- Los servicios Git se migran automáticamente a la nueva estructura

### Estado Global

Raioz mantiene un estado global mínimo en `{base}/state.json` que rastrea información básica de todos los proyectos activos.

**Propósito:**

- **Consistencia**: Saber qué proyectos están activos y cuándo se ejecutaron por última vez
- **Debugging**: Identificar rápidamente qué servicios están corriendo en qué proyectos
- **No es telemetría**: Solo almacena metadatos locales, no información sensible

**Estructura del estado global:**

```json
{
  "activeProjects": ["project1", "project2"],
  "projects": {
    "project1": {
      "name": "project1",
      "workspace": "/opt/raioz-proyecto/workspaces/project1",
      "lastExecution": "2025-01-15T10:30:00Z",
      "services": [
        {
          "name": "api",
          "mode": "dev",
          "version": "abc123def456",
          "image": "",
          "status": "running"
        }
      ]
    }
  }
}
```

**Información almacenada:**

- `activeProjects`: Lista de nombres de proyectos activos
- `projects`: Mapa de proyectos con:
  - `name`: Nombre del proyecto
  - `workspace`: Ruta del workspace
  - `lastExecution`: Timestamp de última ejecución
  - `services`: Lista de servicios con:
    - `name`: Nombre del servicio
    - `mode`: Modo (dev/prod)
    - `version`: Commit SHA (para git) o tag de imagen
    - `image`: Imagen completa (si aplica)
    - `status`: Estado (running/stopped)

**Comandos relacionados:**

- `raioz list`: Lista todos los proyectos activos
- `raioz up`: Actualiza el estado global después de iniciar
- `raioz down`: Elimina el proyecto del estado global después de detener

**Limpiar estado global:**
Si necesitas limpiar el estado global manualmente:

```bash
# El archivo está en {base}/state.json
# Por defecto: /opt/raioz-proyecto/state.json o ~/.raioz/state.json
rm /opt/raioz-proyecto/state.json
```

### Modos Dev vs Prod

#### Dev Mode (`mode: "dev"`)

- Bind mounts para hot-reload
- Healthchecks más permisivos
- Logs más verbosos
- Restart policy: no (sin auto-restart)
- Bind mounts automáticos según runtime

#### Prod Mode (`mode: "prod"`)

- Solo imagen (sin bind mounts)
- Healthchecks estrictos
- Logs estándar
- Restart policy: unless-stopped
- Sin volúmenes de desarrollo

### Profiles

Permiten filtrar servicios por perfil:

```bash
# Solo servicios frontend
raioz up --profile frontend

# Solo servicios backend
raioz up --profile backend
```

Los servicios sin `profiles` siempre se incluyen.

## 🔧 Mejores Prácticas

### 1. Versionar `.raioz.json`

Incluye `.raioz.json` en el repositorio del proyecto. Se revisa en PRs y permite reproducir entornos.

### 2. Usar ramas estables para servicios compartidos

Para servicios compartidos, usa ramas estables (`main`, `develop`) en lugar de feature branches.

### 3. Nombrar redes y volúmenes claramente

Usa nombres descriptivos que incluyan el proyecto:

- Red: `billing-network`
- Volumen: `billing-postgres-data`

### 4. Validar configuración antes de commit

```bash
raioz up --dry-run  # Si implementado
raioz check        # Verificar alineación
```

### 5. Usar modos apropiados

- `dev`: Para desarrollo activo con hot-reload
- `prod`: Para servicios estables o infraestructura

### 6. Servicios Readonly vs Editable

**Cuándo usar `access: "readonly"`:**

- Servicios de otros equipos que no debes modificar
- Dependencias externas que solo consumes
- Servicios que quieres proteger de cambios accidentales
- Servicios que se recrean automáticamente si fallan

**Cuándo usar `access: "editable"` (o no especificar):**

- Servicios que desarrollas activamente
- Servicios que necesitas modificar y hacer commit
- Servicios con hot-reload en modo dev

**Ejemplo de servicio readonly:**

```json
{
  "auth-service": {
    "source": {
      "kind": "git",
      "repo": "git@github.com:org/auth-service.git",
      "branch": "main",
      "path": "services/auth",
      "access": "readonly"
    },
    "docker": {
      "mode": "prod",
      "ports": ["3001:3000"]
    }
  }
}
```

**Comportamiento de servicios readonly:**

- ✅ Se clonan solo si no existen
- ✅ No se actualizan automáticamente (no checkout/pull)
- ✅ Volúmenes montados como `:ro` (read-only)
- ✅ `restart: unless-stopped` (se recrean si fallan)
- ✅ Protegidos de modificaciones accidentales

### 7. Organizar variables de entorno

- Variables comunes → `global.env`
- Variables de proyecto → `projects/{project-name}.env`
- Variables de servicio → `services/{service-name}.env`

### 8. Limpiar recursos regularmente

```bash
# Ver qué se limpiaría
raioz clean --dry-run

# Limpiar recursos no usados
raioz clean --images --networks
```

### 9. Convención de Nombres

Raioz aplica una convención estricta de nombres para facilitar la identificación y debugging de contenedores y recursos Docker.

**Formato de nombres:**

- **Contenedores de servicios**: `raioz-{project}-{service}`
- **Contenedores de infra**: `raioz-{project}-{infra}`
- **Redes**: `{project.network}` (normalizado)
- **Volúmenes**: Nombres según configuración (normalizados si es necesario)

**Ejemplos:**

- Proyecto `billing-platform`, servicio `api` → contenedor: `raioz-billing-platform-api`
- Proyecto `billing-platform`, infra `database` → contenedor: `raioz-billing-platform-database`
- Proyecto `billing-platform`, network `billing-network` → red: `billing-network`

**Normalización automática:**

- Convierte a minúsculas
- Reemplaza caracteres inválidos con guiones
- Elimina guiones duplicados
- Trunca si excede 63 caracteres (límite de Docker)

**Validación:**

- Los nombres de proyecto, servicio e infra se validan automáticamente
- Solo se permiten letras minúsculas, números y guiones
- No pueden empezar o terminar con guión
- No pueden tener guiones consecutivos

**Beneficios:**

- **Debugging rápido**: Fácil identificar contenedores con `docker ps | grep raioz-{project}`
- **Búsqueda simple**: `docker logs raioz-billing-platform-api`
- **Consistencia**: Todos los recursos siguen el mismo patrón
- **Sin conflictos**: Nombres únicos por proyecto

**Ejemplo de uso:**

```bash
# Ver todos los contenedores de un proyecto
docker ps | grep raioz-billing-platform

# Ver logs de un servicio específico
docker logs raioz-billing-platform-api

# Inspeccionar un contenedor
docker inspect raioz-billing-platform-api
```

## 🔍 Troubleshooting

> 💡 **Nota**: Si tu problema no está listado aquí, o si necesitas entender qué casos no soporta Raioz, consulta el [Documento de Límites](./docs/limits.md) para más información sobre limitaciones conocidas y casos no soportados.

### Problemas comunes

#### Error: "port is already in use"

**Problema:** Otro proyecto está usando el mismo puerto.

**Solución:**

```bash
# Ver qué puertos están en uso
raioz ports

# Cambiar el puerto en .raioz.json
"ports": ["3001:3000"]  # Usar puerto alternativo
```

#### Error: "branch does not exist in remote repository"

**Problema:** La rama especificada no existe en el remoto.

**Solución:**

- Verificar el nombre de la rama
- Crear la rama en el repositorio
- Actualizar `.raioz.json` con la rama correcta

#### Error: "uncommitted changes or merge conflicts"

**Problema:** El repositorio tiene cambios sin commit o conflictos.

**Solución:**

```bash
# Opción 1: Resolver manualmente
cd /opt/raioz-proyecto/services/{service-path}
git status
# Resolver conflictos o hacer commit

# Opción 2: Forzar re-clonado (pierde cambios locales)
raioz up --force-reclone
```

#### Error: "lock already exists"

**Problema:** Otro proceso `raioz` está ejecutándose.

**Solución:**

- Esperar a que termine el otro proceso
- Si el proceso falló, eliminar el lock manualmente:
  ```bash
  rm /opt/raioz-proyecto/workspaces/{project}/.raioz.lock
  ```

#### Servicios no inician

**Verificar:**

```bash
# Ver estado detallado
raioz status

# Ver logs
raioz logs --all

# Ver logs de Docker directamente
docker compose -f /opt/raioz-proyecto/workspaces/{project}/docker-compose.generated.yml logs
```

#### Variables de entorno no se cargan

**Verificar:**

1. Archivos `.env` existen en `env/`
2. Rutas en `.raioz.json` son correctas
3. `useGlobal` está configurado correctamente

**Debug:**

```bash
# Ver archivo generado
cat /opt/raioz-proyecto/workspaces/{project}/.env.{service-name}
```

#### Docker network no se crea

**Solución:**

```bash
# Crear manualmente si es necesario
docker network create {network-name}

# O ejecutar raioz up de nuevo
raioz up
```

### Convención de Nombres

Raioz aplica una convención estricta de nombres para facilitar la identificación y debugging de contenedores y recursos Docker.

**Formato de nombres:**

- **Contenedores de servicios**: `raioz-{project}-{service}`
- **Contenedores de infra**: `raioz-{project}-{infra}`
- **Redes**: `{project.network}` (normalizado)
- **Volúmenes**: Nombres según configuración (normalizados si es necesario)

**Ejemplos:**

- Proyecto `billing-platform`, servicio `api` → contenedor: `raioz-billing-platform-api`
- Proyecto `billing-platform`, infra `database` → contenedor: `raioz-billing-platform-database`
- Proyecto `billing-platform`, network `billing-network` → red: `billing-network`

**Normalización automática:**

- Convierte a minúsculas
- Reemplaza caracteres inválidos con guiones
- Elimina guiones duplicados
- Trunca si excede 63 caracteres (límite de Docker)

**Validación:**

- Los nombres de proyecto, servicio e infra se validan automáticamente
- Solo se permiten letras minúsculas, números y guiones
- No pueden empezar o terminar con guión
- No pueden tener guiones consecutivos

**Beneficios:**

- **Debugging rápido**: Fácil identificar contenedores con `docker ps | grep raioz-{project}`
- **Búsqueda simple**: `docker logs raioz-billing-platform-api`
- **Consistencia**: Todos los recursos siguen el mismo patrón
- **Sin conflictos**: Nombres únicos por proyecto

**Ejemplo de uso:**

```bash
# Ver todos los contenedores de un proyecto
docker ps | grep raioz-billing-platform

# Ver logs de un servicio específico
docker logs raioz-billing-platform-api

# Inspeccionar un contenedor
docker inspect raioz-billing-platform-api
```

**Nombres válidos e inválidos:**

✅ **Válidos:**

- `billing-platform`
- `api-v2`
- `service-1`
- `raioz-billing-platform-api`

❌ **Inválidos:**

- `Billing-Platform` (mayúsculas)
- `billing_platform` (guión bajo)
- `billing--platform` (guiones consecutivos)
- `-billing-platform` (empieza con guión)
- `billing-platform-` (termina con guión)

### Debugging

#### Ver configuración generada

```bash
# Ver docker-compose generado
cat /opt/raioz-proyecto/workspaces/{project}/docker-compose.generated.yml

# Ver estado guardado
cat /opt/raioz-proyecto/workspaces/{project}/.state.json
```

#### Ver logs detallados

```bash
# Logs de un servicio específico
raioz logs --follow --tail 100 {service-name}

# Logs de Docker directamente
docker compose -f {compose-path} logs --follow {service-name}
```

#### Verificar estado de contenedores

```bash
# Ver contenedores corriendo
docker ps

# Ver redes
docker network ls

# Ver volúmenes
docker volume ls

# Inspeccionar contenedor
docker inspect raioz-{project}-{service}
```

#### Verificar workspace

```bash
# Ver estructura de workspace
ls -la /opt/raioz-proyecto/workspaces/{project}/
ls -la /opt/raioz-proyecto/services/
ls -la /opt/raioz-proyecto/env/
```

### Logs y cómo interpretarlos

#### Logs de `raioz up`

```
✔ verifying Docker images...
✔ ensuring network my-network
✔ ensuring volume postgres-data
✔ resolving api
⚠️  Configuration changes detected:
  ~ api.source.branch: main -> develop
✔ generating docker-compose.generated.yml
✔ starting services...
✔ Project 'my-project' started successfully
```

**Información importante:**

- `✔` = Operación exitosa
- `⚠️` = Advertencia (revisar pero continúa)
- `🔴` = Error (comando falla)

#### Logs de servicios

Los logs siguen el formato estándar de Docker Compose:

- Prefijo con nombre del servicio
- Timestamps (si están habilitados)
- Output stdout/stderr combinado

**Ejemplo:**

```
api_1    | 2024-01-15 10:30:00 INFO Server starting on port 8080
api_1    | 2024-01-15 10:30:01 INFO Connected to database
frontend_1 | 2024-01-15 10:30:02 INFO Dev server running on http://localhost:3000
```

#### Interpretar errores comunes

**Error de conexión a base de datos:**

```
api_1    | ERROR: connection refused (database:5432)
```

→ Verificar que `database` está en `dependsOn` y está corriendo.

**Error de puerto en uso:**

```
Error: port 3000 is already in use by project 'other-project' service 'frontend'
```

→ Cambiar puerto o detener el otro proyecto.

**Error de imagen no encontrada:**

```
Error: image org/image:tag not found
```

→ Verificar tag de imagen o conexión a registry.

## 📚 Recursos adicionales

- [Documento de Límites](./docs/limits.md) - Qué NO hace Raioz, casos no soportados y decisiones de diseño
- [Guía de Desarrollo](DEVELOPMENT.md) - Cómo contribuir al proyecto
- [Definiciones del Proyecto](.ia/DEFINICIONES_PROYECTO.md) - Arquitectura y decisiones
- [Estándares de Código](.ia/CODIGO_STANDARDS.md) - Reglas de desarrollo

## 🐛 Reportar problemas

Si encuentras un bug o tienes una sugerencia:

1. Verifica que no esté ya reportado
2. Revisa los logs y troubleshooting
3. **Consulta el [Documento de Límites](./docs/limits.md)** para verificar si es un caso no soportado
4. Abre un issue con:
   - Descripción del problema
   - Pasos para reproducir
   - Logs relevantes
   - Versión de `raioz`
   - Configuración relevante (sin datos sensibles)

## 📝 Licencia

[Especificar licencia del proyecto]

## 🙏 Contribuciones

Las contribuciones son bienvenidas. Por favor, lee la [Guía de Desarrollo](DEVELOPMENT.md) antes de contribuir.
