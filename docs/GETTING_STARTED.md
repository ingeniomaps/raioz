# Getting Started

Guía paso a paso para empezar a usar Raioz en tu equipo de desarrollo.

## Requisitos previos

- **Docker** (con Docker Compose v2)
- **Git**
- **Go 1.22+** (solo si compilas desde fuente)

Verifica:

```bash
docker --version    # Docker 20.10+
docker compose version  # v2.0+
git --version       # Git 2.0+
```

## Instalación

```bash
# Opción 1: Script de instalación
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash

# Opción 2: Compilar desde fuente
git clone https://github.com/ingeniomaps/raioz.git
cd raioz && make build && sudo make install

# Verificar
raioz version
```

## Tu primer proyecto

### 1. Crear la configuración

Puedes usar el wizard interactivo o crear el archivo manualmente.

**Con wizard:**

```bash
raioz init
```

**Manualmente** — crea `.raioz.json` en la raíz de tu proyecto:

```json
{
  "schemaVersion": "1.0",
  "network": "my-project-network",
  "project": {
    "name": "my-project"
  },
  "services": {
    "api": {
      "source": {
        "kind": "image",
        "image": "hashicorp/http-echo",
        "tag": "latest"
      },
      "docker": {
        "mode": "prod",
        "ports": ["5678:5678"],
        "command": "-text='hello from raioz'"
      }
    }
  },
  "infra": {
    "redis": {
      "image": "redis",
      "tag": "7-alpine",
      "ports": ["6379:6379"]
    }
  }
}
```

### 2. Levantar el proyecto

```bash
raioz up
```

Raioz automáticamente:
- Crea la red Docker
- Descarga las imágenes
- Genera `docker-compose.generated.yml`
- Levanta infra (redis) y espera a que esté saludable
- Levanta servicios (api)

### 3. Verificar que funciona

```bash
# Ver el estado
raioz status

# Ver puertos en uso
raioz ports

# Verificar que responde
curl localhost:5678
```

### 4. Trabajar con los servicios

```bash
# Ver logs
raioz logs api
raioz logs redis

# Ejecutar comando en un contenedor
raioz exec redis redis-cli ping

# Reiniciar un servicio
raioz restart api
```

### 5. Detener el proyecto

```bash
raioz down
```

## Tipos de servicios

Raioz soporta 4 tipos de fuente para servicios:

### `image` — Imagen Docker directa

El más simple. Usa una imagen pública o privada:

```json
{
  "source": {
    "kind": "image",
    "image": "nginx",
    "tag": "alpine"
  },
  "docker": {
    "mode": "prod",
    "ports": ["8080:80"]
  }
}
```

### `git` — Repositorio Git

Clona un repo y lo monta en modo dev:

```json
{
  "source": {
    "kind": "git",
    "repo": "git@github.com:org/api.git",
    "branch": "main",
    "path": "services/api"
  },
  "docker": {
    "mode": "dev",
    "ports": ["3000:3000"],
    "command": "npm run dev",
    "runtime": "node"
  }
}
```

### `local` — Ruta local

Usa código que ya tienes en tu máquina:

```json
{
  "source": {
    "kind": "local",
    "path": "/home/dev/my-service"
  },
  "docker": {
    "mode": "dev",
    "ports": ["3000:3000"]
  }
}
```

### `command` — Ejecución en el host

Ejecuta directamente en tu máquina, sin Docker:

```json
{
  "source": {
    "kind": "local",
    "path": "/home/dev/worker",
    "command": "npm start",
    "runtime": "node"
  }
}
```

## Infraestructura

La sección `infra` define servicios de soporte (bases de datos, caches, etc.):

```json
{
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "15-alpine",
      "ports": ["5432:5432"],
      "volumes": ["pg-data:/var/lib/postgresql/data"],
      "env": {
        "POSTGRES_PASSWORD": "dev123",
        "POSTGRES_DB": "myapp"
      }
    },
    "redis": {
      "image": "redis",
      "tag": "7-alpine",
      "ports": ["6379:6379"]
    }
  }
}
```

La infra siempre se levanta primero y raioz espera a que esté saludable antes de iniciar los servicios.

## Variables de entorno

Raioz gestiona variables de entorno con plantillas:

```
env/
  templates/
    global/
      .env.template          # Variables compartidas
    services/
      api/
        .env.template        # Variables del servicio api
    infra/
      postgres/
        .env.template        # Variables de postgres
```

En `.raioz.json`:

```json
{
  "env": {
    "useGlobal": true,
    "files": ["global"]
  },
  "services": {
    "api": {
      "env": ["services/api"]
    }
  }
}
```

## Operaciones diarias

### Reiniciar servicios sin reiniciar infra

```bash
raioz restart api
raioz restart --all               # Solo servicios, no infra
raioz restart --all --include-infra  # Todo
```

### Ejecutar comandos en contenedores

```bash
raioz exec postgres psql -U postgres
raioz exec redis redis-cli
raioz exec api sh
```

### Sobreescribir un servicio con código local

```bash
raioz override api --path ~/dev/mi-api
raioz up    # Usa tu copia local en vez del repo
raioz override remove api   # Vuelve al repo
```

### Ignorar servicios que no necesitas

```bash
raioz ignore add legacy-worker
raioz up    # No levanta legacy-worker
```

### Gestionar volúmenes

```bash
raioz volumes list                          # Ver volúmenes del proyecto
raioz volumes remove --all --force          # Resetear todos los datos
raioz volumes remove myproject_pg-data      # Resetear solo postgres
```

### Verificar drift de configuración

```bash
raioz check    # Compara config actual vs estado guardado
```

## Cambiar idioma

Raioz soporta inglés y español:

```bash
raioz lang set es    # Cambiar a español
raioz lang set en    # Cambiar a inglés
raioz lang list      # Ver idiomas disponibles
```

## Ejemplos

El directorio `examples/` contiene proyectos de ejemplo listos para usar:

| Ejemplo | Descripción |
|---------|-------------|
| `01-basic-web-app` | Aplicación web con frontend, API y bases de datos |
| `05-image-only` | Servicios usando solo imágenes Docker |
| `07-infra-only` | Solo infraestructura (postgres, redis, mongo) |
| `11-host-service` | Mezcla de servicios Docker y host |
| `13-project-compose` | Proyecto con su propio docker-compose.yml |

```bash
cd examples/05-image-only
raioz up
raioz status
raioz exec redis redis-cli ping
raioz down
```

## Referencia completa

- [Guía de Comandos](./COMMANDS.md) — Documentación detallada de todos los comandos
- [Ejemplos](../examples/) — Proyectos de ejemplo
