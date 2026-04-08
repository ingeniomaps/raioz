# Schema Reference — `.raioz.json`

Referencia completa de todos los campos del archivo de configuración `.raioz.json`.

## Estructura raíz

```json
{
  "schemaVersion": "1.0",
  "workspace": "mi-workspace",
  "network": "mi-red",
  "profiles": ["backend"],
  "project": { ... },
  "services": { ... },
  "infra": { ... },
  "env": { ... }
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `schemaVersion` | string | Si | Versión del schema. Actualmente: `"1.0"` |
| `workspace` | string | No | Nombre del workspace. Si no se especifica, usa `project.name` |
| `network` | string \| object | No | Red Docker. Puede ser un string (nombre) o un objeto con `name` y `subnet` |
| `profiles` | string[] | No | Perfiles activos por defecto al hacer `raioz up` sin `--profile` |
| `project` | object | Si | Configuración del proyecto |
| `services` | object | No | Mapa de servicios (puede estar vacío) |
| `infra` | object | No | Mapa de infraestructura (puede estar vacío) |
| `env` | object | No | Configuración de variables de entorno |

---

## `network`

Puede ser un string simple o un objeto:

**String:**
```json
"network": "mi-red"
```

**Objeto (con subnet):**
```json
"network": {
  "name": "mi-red",
  "subnet": "150.150.0.0/16"
}
```

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `name` | string | Nombre de la red Docker |
| `subnet` | string | CIDR de la subnet (ej: `"150.150.0.0/16"`) |

---

## `project`

```json
"project": {
  "name": "billing-platform",
  "commands": {
    "up": "docker compose up -d",
    "down": "docker compose down",
    "health": "curl -f http://localhost:3000/health"
  },
  "env": ["project-env"]
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `name` | string | Si | Nombre del proyecto. Debe ser lowercase, alfanumérico con guiones |
| `commands` | object | No | Comandos del proyecto (ejecutados en el host) |
| `commands.up` | string | No | Comando a ejecutar después de levantar servicios |
| `commands.down` | string | No | Comando a ejecutar al detener |
| `commands.health` | string | No | Comando para verificar salud |
| `commands.dev` | object | No | Comandos específicos para modo dev (`up`, `down`, `health`) |
| `commands.prod` | object | No | Comandos específicos para modo prod |
| `env` | string[] \| object | No | Variables de entorno a nivel de proyecto |

---

## `services`

Mapa de nombre → configuración de servicio:

```json
"services": {
  "api": {
    "source": { ... },
    "docker": { ... },
    "dependsOn": ["database"],
    "env": ["services/api"],
    "enabled": true,
    "profiles": ["backend"],
    "commands": { ... },
    "mock": { ... },
    "featureFlag": { ... }
  }
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `source` | object | Si | Origen del servicio |
| `docker` | object | No | Configuración Docker (no requerido si usa `source.command`) |
| `dependsOn` | string[] | No | Dependencias a nivel de servicio |
| `env` | string[] \| object | No | Variables de entorno del servicio |
| `volumes` | string[] | No | Volúmenes para servicios host (formato `SRC:DEST`) |
| `enabled` | boolean | No | Habilitar/deshabilitar servicio (default: `true`) |
| `profiles` | string[] | No | Solo incluir cuando se usa `--profile` matching |
| `commands` | object | No | Comandos custom (`up`, `down`, `health`) |
| `mock` | object | No | Configuración de mock |
| `featureFlag` | object | No | Feature flag |

### `source`

```json
"source": {
  "kind": "git",
  "repo": "git@github.com:org/api.git",
  "branch": "main",
  "path": "services/api",
  "image": "nginx",
  "tag": "alpine",
  "access": "editable",
  "command": "npm start",
  "runtime": "node"
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `kind` | string | Si | Tipo: `"git"`, `"image"`, `"local"` |
| `repo` | string | Si (git) | URL del repositorio Git |
| `branch` | string | Si (git) | Rama a clonar |
| `path` | string | Si (git/local) | Directorio donde clonar o ruta local |
| `image` | string | Si (image) | Nombre de la imagen Docker |
| `tag` | string | No | Tag de la imagen (default: `latest`) |
| `access` | string | No | `"readonly"` o `"editable"` (solo git, default: `"editable"`) |
| `command` | string | No | Comando para ejecutar en el host (sin Docker) |
| `runtime` | string | No | Runtime para ejecución host (`node`, `go`, `python`, etc.) |

**Tipos de servicio:**

| Kind | Docker | Command | Comportamiento |
|------|--------|---------|---------------|
| `git` | Si | No | Clona repo, levanta en Docker |
| `git` | No | Si | Clona repo, ejecuta en host |
| `image` | Si | No | Usa imagen Docker directa |
| `local` | Si | No | Monta ruta local en Docker |
| `local` | No | Si | Ejecuta comando en host desde ruta local |

### `docker`

```json
"docker": {
  "mode": "dev",
  "ports": ["3000:3000", "9229:9229"],
  "volumes": ["./app:/app", "node-cache:/app/node_modules"],
  "dependsOn": ["database", "redis"],
  "command": "npm run dev",
  "runtime": "node",
  "dockerfile": "Dockerfile.dev",
  "ip": "150.150.0.10"
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `mode` | string | No | `"dev"` o `"prod"` (default: `"dev"`) |
| `ports` | string[] | No | Mapeo de puertos (`"host:container"`) |
| `volumes` | string[] | No | Volúmenes y bind mounts |
| `dependsOn` | string[] | No | Dependencias (servicios o infra) |
| `command` | string | No | Comando a ejecutar dentro del contenedor |
| `runtime` | string | No | Runtime para imagen base (`node`, `go`, `python`, etc.) |
| `dockerfile` | string | No | Ruta al Dockerfile (relativa al servicio) |
| `ip` | string | No | IP estática en la red (ej: `"150.150.0.10"`) |

**Diferencias entre modos:**

| Aspecto | Dev | Prod |
|---------|-----|------|
| Bind mounts | Si (código fuente montado) | No (solo named volumes) |
| Restart policy | `no` | `unless-stopped` |
| Healthcheck | Básico | Completo |
| Logging | Verbose | Estructurado |

---

## `infra`

Mapa de nombre → configuración de infraestructura. Cada entrada puede ser un string (path a YAML) o un objeto inline:

**Inline:**
```json
"infra": {
  "postgres": {
    "image": "postgres",
    "tag": "15-alpine",
    "ports": ["5432:5432"],
    "volumes": ["pg-data:/var/lib/postgresql/data"],
    "seed": ["seeds/init.sql", "seeds/test-data.sql"],
    "env": {
      "POSTGRES_PASSWORD": "dev123",
      "POSTGRES_DB": "myapp"
    },
    "ip": "150.150.0.20",
    "healthcheck": {
      "test": ["CMD-SHELL", "pg_isready -U postgres"],
      "interval": "5s",
      "retries": 10
    },
    "profiles": ["backend"]
  }
}
```

**Path a YAML externo:**
```json
"infra": {
  "elasticsearch": "infra/elasticsearch.yml"
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `image` | string | Si | Nombre de la imagen Docker |
| `tag` | string | No | Tag de la imagen (default: `latest`) |
| `ports` | string[] | No | Mapeo de puertos |
| `volumes` | string[] | No | Named volumes |
| `seed` | string[] | No | Archivos/directorios para `/docker-entrypoint-initdb.d/` |
| `env` | string[] \| object | No | Variables de entorno |
| `ip` | string | No | IP estática en la red |
| `healthcheck` | object | No | Healthcheck personalizado |
| `profiles` | string[] | No | Solo incluir con `--profile` matching |

### `seed`

Monta archivos o directorios en `/docker-entrypoint-initdb.d/` para inicialización automática de base de datos. Soportado por PostgreSQL, MySQL, MariaDB y MongoDB.

```json
"seed": ["seeds/init.sql"]
"seed": ["seeds/01-schema.sql", "seeds/02-data.sql"]
"seed": ["seeds/"]
```

- Los paths son relativos al directorio del `.raioz.json`
- Se montan como read-only (`:ro`)
- Solo se ejecutan cuando el volumen de datos es nuevo (comportamiento de Docker)
- Para re-ejecutar: eliminar el volumen con `raioz volumes remove <nombre>`

### `healthcheck`

```json
"healthcheck": {
  "test": ["CMD-SHELL", "pg_isready -U postgres"],
  "interval": "5s",
  "timeout": "5s",
  "retries": 10,
  "start_period": "10s",
  "start_interval": "2s",
  "disable": false
}
```

Si no se especifica, raioz agrega healthchecks por defecto para imágenes comunes:

| Imagen | Healthcheck default |
|--------|-------------------|
| postgres | `pg_isready -U postgres` |
| mysql/mariadb | `mysqladmin ping -h localhost` |
| redis | `redis-cli ping` |
| mongo | `mongosh --eval 'db.adminCommand("ping")'` |
| elasticsearch | `curl -f http://localhost:9200/_cluster/health` |
| rabbitmq | `rabbitmq-diagnostics -q ping` |

---

## `env`

```json
"env": {
  "useGlobal": true,
  "files": ["global", "shared"],
  "variables": {
    "ENVIRONMENT": "development",
    "LOG_LEVEL": "debug"
  }
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `useGlobal` | boolean | No | Usar el archivo `.env` global |
| `files` | string[] | No | Paths a templates de env (relativos a `env/templates/`) |
| `variables` | object | No | Variables inline con valores fijos |

### Estructura de templates

```
env/
  templates/
    global/
      .env.template         # Variables compartidas
    services/
      api/
        .env.template       # Variables del servicio api
    infra/
      postgres/
        .env.template       # Variables de postgres
```

Los templates soportan interpolación:

```env
DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}
API_URL=http://api:${API_PORT}
```

### `env` en servicios

Puede ser un array de paths o un objeto con variables:

**Array (paths a templates):**
```json
"env": ["services/api", "shared/common"]
```

**Objeto (variables directas):**
```json
"env": {
  "NODE_ENV": "development",
  "PORT": "3000"
}
```

---

## Ejemplo completo

```json
{
  "schemaVersion": "1.0",
  "network": {
    "name": "billing-network",
    "subnet": "150.150.0.0/16"
  },
  "project": {
    "name": "billing-platform",
    "commands": {
      "up": "npm run migrate",
      "health": "curl -sf http://localhost:3000/health"
    }
  },
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/billing-api.git",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "command": "npm run dev",
        "runtime": "node",
        "dependsOn": ["postgres", "redis"],
        "ip": "150.150.0.10"
      },
      "env": ["services/api"]
    },
    "worker": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/billing-worker.git",
        "branch": "develop",
        "path": "services/worker"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3001:3000"],
        "command": "npm run dev",
        "runtime": "node",
        "dependsOn": ["postgres", "rabbitmq"]
      },
      "env": ["services/worker"],
      "profiles": ["backend"]
    },
    "frontend": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/billing-ui.git",
        "branch": "develop",
        "path": "services/frontend"
      },
      "docker": {
        "mode": "dev",
        "ports": ["8080:3000"],
        "command": "npm run dev",
        "runtime": "node",
        "dependsOn": ["api"]
      },
      "profiles": ["frontend"]
    }
  },
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "15-alpine",
      "ports": ["5432:5432"],
      "volumes": ["pg-data:/var/lib/postgresql/data"],
      "seed": ["seeds/schema.sql", "seeds/dev-data.sql"],
      "env": {
        "POSTGRES_PASSWORD": "dev123",
        "POSTGRES_DB": "billing"
      }
    },
    "redis": {
      "image": "redis",
      "tag": "7-alpine",
      "ports": ["6379:6379"]
    },
    "rabbitmq": {
      "image": "rabbitmq",
      "tag": "3-management-alpine",
      "ports": ["5672:5672", "15672:15672"],
      "profiles": ["backend"]
    }
  },
  "env": {
    "useGlobal": true,
    "files": ["global"],
    "variables": {
      "ENVIRONMENT": "development"
    }
  }
}
```

### Uso con el ejemplo anterior

```bash
# Levantar todo
raioz up

# Solo backend (api + worker + postgres + redis + rabbitmq)
raioz up --profile backend

# Solo el API y sus dependencias (postgres + redis)
raioz up --only api

# Solo frontend y sus dependencias (api → postgres, redis)
raioz up --only frontend

# Ver grafo de dependencias
raioz graph

# Sobreescribir api con código local
raioz override api --path ~/dev/billing-api
raioz up
```
