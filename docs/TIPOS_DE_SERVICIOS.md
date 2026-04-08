# Tipos de Servicios en Raioz

Este documento explica todos los tipos de servicios que puedes definir en Raioz, sus características, casos de uso y ejemplos.

## 📋 Tabla de Contenidos

1. [Tipos de Servicios por `source.kind`](#tipos-de-servicios-por-sourcekind)
   - [Servicios Git](#1-servicios-git-sourcekind--git)
   - [Servicios Image](#2-servicios-image-sourcekind--image)
   - [Servicios Local](#3-servicios-local-sourcekind--local)
2. [Modos de Ejecución](#modos-de-ejecución)
   - [Ejecución en Docker](#ejecución-en-docker)
   - [Ejecución en Host](#ejecución-en-host)
   - [Comandos Personalizados](#comandos-personalizados)
3. [Infraestructura](#infraestructura-infra)
4. [Modos de Acceso (Git)](#modos-de-acceso-git)
5. [Comparación y Decisiones](#comparación-y-decisiones)

---

## Tipos de Servicios por `source.kind`

Raioz soporta tres tipos de servicios según su origen:

### 1. Servicios Git (`source.kind: "git"`)

Servicios clonados desde un repositorio Git. Raioz gestiona automáticamente el clonado, actualización y construcción.

#### Características

- ✅ Clonado automático desde repositorio Git
- ✅ Actualización automática en `raioz up`
- ✅ Soporte para ramas específicas
- ✅ Build automático desde Dockerfile
- ✅ Modos de acceso: `readonly` o `editable`
- ✅ Hot-reload en modo `dev` (bind mounts)
- ✅ Gestión de estado y cambios

#### Campos Requeridos

```json
{
  "source": {
    "kind": "git",
    "repo": "https://github.com/empresa/mi-servicio.git",
    "branch": "develop",
    "path": "services/mi-servicio"
  }
}
```

#### Campos Opcionales

```json
{
  "source": {
    "kind": "git",
    "repo": "...",
    "branch": "...",
    "path": "...",
    "access": "readonly" | "editable"  // Default: "editable"
  }
}
```

#### Ejemplo Completo

```json
{
  "services": {
    "api-users": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/empresa/users-api.git",
        "branch": "develop",
        "path": "services/users",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3001:3000"],
        "dependsOn": ["postgres"]
      },
      "env": ["services/users"]
    }
  }
}
```

#### Comportamiento

1. **Primera ejecución**: Clona el repositorio en el workspace
2. **Ejecuciones siguientes**: Actualiza el repositorio (si `access: "editable"`) o lo mantiene sin cambios (si `access: "readonly"`)
3. **Build**: Busca Dockerfile en el servicio y construye la imagen
4. **Despliegue**: Ejecuta el contenedor con configuración Docker

#### Casos de Uso

- ✅ Microservicios que desarrollas activamente
- ✅ Servicios compartidos entre proyectos
- ✅ Servicios de terceros que necesitas versionar
- ✅ Proyectos que requieren build desde código fuente

---

### 2. Servicios Image (`source.kind: "image"`)

Servicios que usan directamente una imagen Docker pre-construida, sin necesidad de clonar o construir código.

#### Características

- ✅ No requiere clonado (usa imagen directamente)
- ✅ Inicio rápido (no hay build)
- ✅ Ideal para servicios estables/versionados
- ✅ Útil para servicios externos o de terceros

#### Campos Requeridos

```json
{
  "source": {
    "kind": "image",
    "image": "nginx",
    "tag": "alpine"
  }
}
```

#### Ejemplo Completo

```json
{
  "services": {
    "nginx-proxy": {
      "source": {
        "kind": "image",
        "image": "nginx",
        "tag": "alpine"
      },
      "docker": {
        "mode": "prod",
        "ports": ["80:80", "443:443"],
        "volumes": [
          "./nginx.conf:/etc/nginx/nginx.conf:ro",
          "./logs:/var/log/nginx:rw"
        ]
      },
      "env": ["services/nginx"]
    }
  }
}
```

#### Comportamiento

1. **Pull de imagen**: Descarga la imagen desde Docker Hub/Registry
2. **Sin build**: No construye ninguna imagen
3. **Despliegue directo**: Ejecuta el contenedor inmediatamente

#### Casos de Uso

- ✅ Servicios estables que no modificas
- ✅ Servicios de terceros (nginx, redis, etc.)
- ✅ Servicios que ya tienen imagen publicada
- ✅ Servicios legacy que no quieres clonar

---

### 3. Servicios Local (`source.kind: "local"`)

Servicios que referencian un proyecto local en tu sistema de archivos (no clonado por Raioz).

#### Características

- ✅ Usa código local existente
- ✅ No requiere clonado
- ✅ Ideal para desarrollo de un servicio específico
- ✅ Puede ejecutarse en Docker o en Host
- ✅ Soporte para `docker-compose.yml` existente

#### Campos Requeridos

```json
{
  "source": {
    "kind": "local",
    "path": "."
  }
}
```

#### Campos Opcionales

```json
{
  "source": {
    "kind": "local",
    "path": ".",
    "command": "npm start"  // Ejecuta en host (sin Docker)
  }
}
```

#### Ejemplo 1: Con Docker

```json
{
  "services": {
    "mi-api": {
      "source": {
        "kind": "local",
        "path": "."
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"]
      }
    }
  }
}
```

**Comportamiento**: Busca Dockerfile en el directorio local y construye/ejecuta.

#### Ejemplo 2: Con Comando en Host

```json
{
  "services": {
    "worker": {
      "source": {
        "kind": "local",
        "path": ".",
        "command": "node worker.js",
        "runtime": "node"
      }
    }
  }
}
```

**Comportamiento**: Ejecuta el comando directamente en el host (sin Docker).

#### Ejemplo 3: Con docker-compose.yml Existente

Para proyectos que ya tienen `docker-compose.yml`, usa `project.commands` en lugar de definir el servicio:

```json
{
  "project": {
    "name": "gateway",
    "commands": {
      "up": "docker compose -f docker/docker-compose.yml up -d",
      "down": "docker compose -f docker/docker-compose.yml down"
    }
  }
}
```

**Comportamiento**: Raioz detecta y gestiona el `docker-compose.yml` existente.

#### Casos de Uso

- ✅ Desarrollo activo de un servicio específico
- ✅ Proyectos legacy que ya tienen configuración Docker
- ✅ Servicios que no están en Git (temporales, experimentales)
- ✅ Servicios que requieren ejecución en host (sin Docker)

---

## Modos de Ejecución

Independientemente del tipo de servicio (`source.kind`), hay diferentes formas de ejecutarlo:

### Ejecución en Docker

Cuando defines `docker` en la configuración, el servicio se ejecuta en un contenedor Docker.

#### Configuración Básica

```json
{
  "services": {
    "mi-servicio": {
      "source": { ... },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "volumes": ["./app:/app:rw"]
      }
    }
  }
}
```

#### Modos: `dev` vs `prod`

**Modo Dev (`mode: "dev"`)**:
- ✅ Bind mounts para hot-reload
- ✅ `restart: "no"` (no auto-restart)
- ✅ Logs completos
- ✅ Sin healthchecks estrictos

**Modo Prod (`mode: "prod"`)**:
- ✅ Sin bind mounts (imagen final)
- ✅ `restart: "unless-stopped"`
- ✅ Healthchecks automáticos
- ✅ Logging configurado

#### Campos Docker Disponibles

```json
{
  "docker": {
    "mode": "dev" | "prod",          // Requerido
    "ports": ["3000:3000"],           // Opcional: null = no ports
    "volumes": ["./app:/app:rw"],     // Opcional
    "dependsOn": ["postgres"],        // Opcional: dependencias
    "dockerfile": "Dockerfile.dev",   // Opcional: Dockerfile personalizado
    "command": "npm start",           // Opcional: comando dentro del contenedor
    "ip": "192.160.1.10",            // Opcional: IP estática
    "runtime": "node"                 // Opcional: documentación
  }
}
```

---

### Ejecución en Host

Cuando defines `source.command`, el servicio se ejecuta directamente en el host (sin Docker).

#### Configuración

```json
{
  "services": {
    "monitor": {
      "source": {
        "kind": "local",
        "path": ".",
        "command": "python monitor.py",
        "runtime": "python"
      }
    }
  }
}
```

#### Características

- ✅ Ejecuta directamente en el sistema
- ✅ No requiere Docker
- ✅ Acceso completo a recursos del host
- ✅ Más rápido para desarrollo
- ⚠️ Menos aislado (no es contenedor)

#### Casos de Uso

- ✅ Scripts de monitoreo/background
- ✅ Servicios que necesitan acceso directo al sistema
- ✅ Desarrollo rápido sin Docker
- ✅ Herramientas que ya están instaladas en el host

---

### Comandos Personalizados

Cuando defines `commands` (sin `docker` ni `source.command`), puedes definir comandos personalizados para iniciar/detener el servicio.

#### Configuración

```json
{
  "services": {
    "custom-service": {
      "source": {
        "kind": "local",
        "path": "."
      },
      "commands": {
        "up": "./start.sh",
        "down": "./stop.sh",
        "health": "curl http://localhost:8080/health"
      }
    }
  }
}
```

#### Modos Específicos (dev/prod)

```json
{
  "commands": {
    "up": "./start.sh",  // Default
    "dev": {
      "up": "./start-dev.sh",
      "down": "./stop-dev.sh"
    },
    "prod": {
      "up": "./start-prod.sh",
      "down": "./stop-prod.sh"
    }
  }
}
```

---

## Infraestructura (`infra`)

Los servicios de infraestructura son similares a `source.kind: "image"` pero tienen una sección dedicada por su naturaleza.

#### Características

- ✅ Servicios de infraestructura (DB, cache, etc.)
- ✅ No se clonan ni construyen
- ✅ Usan imágenes directamente
- ✅ Healthchecks automáticos para servicios comunes
- ✅ Variables de entorno simplificadas

#### Ejemplo

```json
{
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "18.1-alpine",
      "ports": null,  // null = no ports exposed
      "volumes": ["postgres_data:/var/lib/postgresql"],
      "ip": "192.160.1.2",
      "env": {
        "POSTGRES_DB": "mi_db",
        "POSTGRES_USER": "admin",
        "POSTGRES_PASSWORD": "secret"
      }
    },
    "redis": {
      "image": "redis",
      "tag": "alpine",
      "volumes": ["redis_data:/data"],
      "ip": "192.160.1.3"
    }
  }
}
```

#### Servicios con Healthchecks Automáticos

Raioz agrega healthchecks automáticos para:
- ✅ **PostgreSQL**: `pg_isready`
- ✅ **Redis**: `redis-cli ping`
- ✅ **MongoDB**: `mongosh ping`
- ✅ **MySQL/MariaDB**: `mysqladmin ping`
- ✅ **PgAdmin**: HTTP healthcheck

#### Variables de Entorno

Puedes usar array (archivos) o objeto (variables directas):

```json
{
  "infra": {
    "postgres": {
      "env": ["services/postgres"]  // Archivos .env
    }
  }
}
```

O:

```json
{
  "infra": {
    "postgres": {
      "env": {
        "POSTGRES_DB": "mi_db",
        "POSTGRES_USER": "admin"
      }
    }
  }
}
```

---

## Modos de Acceso (Git)

Para servicios Git, puedes especificar cómo Raioz gestiona el repositorio:

### `access: "editable"` (Default)

- ✅ Raioz actualiza automáticamente el repositorio
- ✅ Puedes hacer commits y push
- ✅ Ubicación: `{workspace}/local/{path}`
- ✅ Ideal para desarrollo activo

```json
{
  "source": {
    "kind": "git",
    "repo": "...",
    "branch": "develop",
    "path": "services/api",
    "access": "editable"  // Default, puede omitirse
  }
}
```

### `access: "readonly"`

- ✅ Raioz NO actualiza automáticamente el repositorio
- ✅ No puedes hacer commits accidentalmente
- ✅ Ubicación: `{workspace}/readonly/{path}`
- ✅ Ideal para servicios que solo consumes

```json
{
  "source": {
    "kind": "git",
    "repo": "...",
    "branch": "main",
    "path": "services/shared",
    "access": "readonly"
  }
}
```

---

## Comparación y Decisiones

### ¿Qué tipo de servicio usar?

| Característica | Git | Image | Local |
|---------------|-----|-------|-------|
| **Código fuente** | Clonado automático | No aplica | Ya existe localmente |
| **Build** | Automático (Dockerfile) | No aplica | Opcional |
| **Actualización** | Automática (`editable`) | Manual (pull) | Manual |
| **Desarrollo activo** | ✅ Ideal | ❌ No | ✅ Sí |
| **Servicios estables** | ✅ Sí | ✅ Ideal | ⚠️ Puede |
| **Requiere Git** | ✅ Sí | ❌ No | ❌ No |
| **Velocidad inicio** | Media (clone + build) | Rápida (pull) | Media (build) |

### Decision Tree

```
¿Necesitas clonar desde Git?
├─ SÍ → source.kind: "git"
│   ├─ ¿Desarrollas activamente? → access: "editable"
│   └─ ¿Solo consumes? → access: "readonly"
│
├─ NO, ¿Tienes código local?
│   ├─ SÍ → source.kind: "local"
│   │   ├─ ¿Tienes docker-compose.yml? → Usa project.commands
│   │   ├─ ¿Quieres ejecutar en Docker? → Define docker
│   │   └─ ¿Quieres ejecutar en host? → Define source.command
│   │
│   └─ NO, ¿Usas imagen directa?
│       └─ SÍ → source.kind: "image"
│
└─ ¿Es infraestructura (DB, cache)?
    └─ SÍ → Usa sección "infra"
```

### Ejemplo Real: Proyecto Completo

```json
{
  "schemaVersion": "1.0",
  "workspace": "mi-proyecto",
  "project": {
    "name": "plataforma",
    "network": {
      "name": "mi-red",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "docker compose -f docker/docker-compose.yml up -d"
    }
  },
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "18.1-alpine",
      "ip": "192.160.1.2",
      "env": {
        "POSTGRES_DB": "plataforma"
      }
    }
  },
  "services": {
    "api-users": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/empresa/users-api.git",
        "branch": "develop",
        "path": "services/users",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3001:3000"],
        "dependsOn": ["postgres"]
      },
      "env": ["services/users"]
    },
    "nginx": {
      "source": {
        "kind": "image",
        "image": "nginx",
        "tag": "alpine"
      },
      "docker": {
        "mode": "prod",
        "ports": ["80:80"]
      }
    },
    "worker": {
      "source": {
        "kind": "local",
        "path": ".",
        "command": "node worker.js",
        "runtime": "node"
      }
    }
  }
}
```

---

## Resumen

| Tipo | Cuándo Usar | Ejemplo |
|------|-------------|---------|
| **Git + Editable** | Desarrollo activo de microservicio | API en desarrollo |
| **Git + Readonly** | Servicio compartido que solo consumes | Biblioteca compartida |
| **Image** | Servicio estable o de terceros | Nginx, Redis |
| **Local + Docker** | Proyecto local con Dockerfile | Desarrollo local |
| **Local + Command** | Script o servicio en host | Monitoreo, workers |
| **Infra** | Bases de datos, caches | PostgreSQL, Redis |
| **Project Commands** | Proyecto con docker-compose.yml existente | Gateway, proxy |

---

## Referencias

- [Configuración de Servicios](../README.md#configuración-de-servicios)
- [Casos de Uso](./casos-de-uso/)
- [Ejemplos de Conflictos](./ejemplos/conflicto-servicios.md)
- [Documentación de Docker](./COMMANDS.md)
