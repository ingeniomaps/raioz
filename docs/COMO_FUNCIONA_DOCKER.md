# Cómo Funciona el Campo `docker: {}` en Raioz

## ❌ Concepto Incorrecto

**NO**: El campo `docker: {}` **NO** toma la imagen del `docker-compose.yml` del proyecto.

Raioz **genera** el `docker-compose.generated.yml` completamente desde `.raioz.json`. No lee ningún `docker-compose.yml` existente del proyecto.

---

## ✅ Cómo Funciona Realmente

El campo `docker: {}` define **cómo se ejecuta el contenedor Docker**, pero **NO define de dónde viene la imagen**.

**La imagen/construcción depende del `source.kind`:**

---

## 📦 ¿Qué Dockerfile Necesita el Proyecto?

### Para Servicios con `source.kind: "git"` o `"local"`

Raioz busca un Dockerfile en el siguiente orden:

1. **Si especificas `docker.dockerfile`**: Usa ese nombre exacto
   ```json
   {
     "docker": {
       "dockerfile": "Dockerfile.prod"
     }
   }
   ```
   → Busca `Dockerfile.prod` en el repositorio/directorio

2. **Si NO especificas `docker.dockerfile`**: Por defecto busca `Dockerfile.dev`
   ```json
   {
     "docker": {
       "mode": "dev"
       // dockerfile no especificado → busca "Dockerfile.dev"
     }
   }
   ```
   → Busca `Dockerfile.dev` en el repositorio/directorio

3. **Si `Dockerfile.dev` NO existe**:
   - ✅ **Tiene `docker.command`**: Raioz genera un Dockerfile wrapper automático
   - ❌ **NO tiene `docker.command`**: Error (requiere Dockerfile o command)

### Resumen

| ¿Qué tienes? | ¿Qué busca Raioz? | ¿Resultado? |
|--------------|-------------------|-------------|
| `docker.dockerfile: "Dockerfile.prod"` | `Dockerfile.prod` | ✅ Usa ese archivo |
| No especificas `docker.dockerfile` | `Dockerfile.dev` | ✅ Usa ese archivo si existe |
| No existe `Dockerfile.dev` pero tienes `docker.command` | - | ✅ Genera wrapper automático |
| No existe `Dockerfile.dev` y NO tienes `docker.command` | - | ❌ Error: necesita Dockerfile o command |

**Nota importante**: Raioz NO busca `Dockerfile` (sin `.dev`) por defecto. Solo busca `Dockerfile.dev` si no especificas `docker.dockerfile`.

Si tu proyecto tiene solo `Dockerfile` (sin `.dev`), debes especificarlo:

```json
{
  "docker": {
    "dockerfile": "Dockerfile"  // Especifica explícitamente
  }
}
```

---

### La imagen/construcción depende del `source.kind`:**

### 1. Si `source.kind: "git"` → **BUILD desde Dockerfile**

Raioz:
1. Clona el repositorio Git
2. Busca un `Dockerfile` en el repositorio clonado (por defecto `Dockerfile.dev`)
3. **Construye la imagen** usando `docker build`
4. Ejecuta el contenedor con la imagen construida

#### Ejemplo

```json
{
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/empresa/api.git",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"]
      }
    }
  }
}
```

**Qué genera Raioz en `docker-compose.generated.yml`:**

```yaml
services:
  api:
    build:
      context: /opt/raioz-proyecto/workspaces/mi-proyecto/local/services/api
      dockerfile: Dockerfile.dev
    container_name: raioz-mi-proyecto-api
    ports:
      - "3000:3000"
    # ... más configuración
```

**Nota**: La imagen se construye desde el Dockerfile del repositorio clonado, NO desde ningún docker-compose.yml del proyecto.

---

### 2. Si `source.kind: "image"` → **Usa Imagen Directa**

Raioz:
1. Toma la imagen y tag de `source.image` y `source.tag`
2. **Hace pull** de la imagen desde Docker Hub/Registry (si no existe localmente)
3. Ejecuta el contenedor directamente con esa imagen

#### Ejemplo

```json
{
  "services": {
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
    }
  }
}
```

**Qué genera Raioz en `docker-compose.generated.yml`:**

```yaml
services:
  nginx:
    image: nginx:alpine
    container_name: raioz-mi-proyecto-nginx
    ports:
      - "80:80"
    # ... más configuración
```

**Nota**: Usa la imagen directamente, NO busca ningún docker-compose.yml.

---

### 3. Si `source.kind: "local"` → **BUILD desde Dockerfile Local**

Raioz:
1. Busca un `Dockerfile` en el directorio local (especificado en `source.path`)
2. **Construye la imagen** usando `docker build`
3. Ejecuta el contenedor con la imagen construida

#### Ejemplo

```json
{
  "services": {
    "mi-app": {
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

**Qué genera Raioz en `docker-compose.generated.yml`:**

```yaml
services:
  mi-app:
    build:
      context: /home/usuario/proyectos/mi-app
      dockerfile: Dockerfile.dev
    container_name: raioz-mi-proyecto-mi-app
    ports:
      - "3000:3000"
    # ... más configuración
```

**Nota**: Construye desde el Dockerfile en el directorio local, NO desde ningún docker-compose.yml del proyecto.

---

## ¿Qué Hace el Campo `docker: {}`?

El campo `docker: {}` configura **cómo se ejecuta el contenedor**, pero **NO de dónde viene la imagen**.

### Configuraciones Disponibles

```json
{
  "docker": {
    "mode": "dev" | "prod",          // Modo de ejecución (requerido)
    "ports": ["3000:3000"],           // Puertos mapeados (opcional)
    "volumes": ["./app:/app:rw"],     // Volúmenes (opcional)
    "dependsOn": ["postgres"],        // Dependencias (opcional)
    "dockerfile": "Dockerfile.dev",   // Dockerfile personalizado (solo para git/local)
    "command": "npm start",           // Comando dentro del contenedor (opcional)
    "ip": "192.160.1.10",            // IP estática (opcional)
    "runtime": "node"                 // Documentación (opcional)
  }
}
```

### Lo que Genera en Docker Compose

Estas configuraciones se traducen directamente a campos del `docker-compose.generated.yml`:

| Campo en `docker: {}` | Campo en `docker-compose.yml` |
|----------------------|------------------------------|
| `mode: "dev"` | `restart: "no"` + bind mounts para hot-reload |
| `mode: "prod"` | `restart: "unless-stopped"` + healthchecks |
| `ports` | `ports` |
| `volumes` | `volumes` |
| `dependsOn` | `depends_on` |
| `dockerfile` | `build.dockerfile` |
| `command` | `command` (dentro del contenedor) |
| `ip` | `networks.{network}.ipv4_address` |

---

## Caso Especial: Proyectos con `docker-compose.yml` Existente

Si tu proyecto **ya tiene un `docker-compose.yml`** (como tu caso del gateway), **NO uses `docker: {}` en servicios**. En su lugar, usa `project.commands`:

### ❌ Incorrecto (No uses esto)

```json
{
  "services": {
    "nginx": {
      "docker": {
        "mode": "dev",
        "image": "openresty/openresty:alpine",
        "ports": ["80:80", "443:443"],
        // ... toda la configuración duplicada
      }
    }
  }
}
```

### ✅ Correcto (Usa esto)

```json
{
  "project": {
    "name": "nginx",
    "commands": {
      "up": "docker compose -f docker/docker-compose.yml up -d",
      "down": "docker compose -f docker/docker-compose.yml down"
    }
  }
}
```

**Comportamiento**:
- Raioz ejecuta tu `docker-compose.yml` existente
- No genera ningún `docker-compose.generated.yml` para ese servicio
- Detecta y gestiona automáticamente el `docker-compose.yml` del proyecto

---

## Flujo Completo: Cómo Raioz Decide la Imagen

```
┌─────────────────────────────────────────────────────────────┐
│  1. Lee .raioz.json                                         │
└─────────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  2. ¿Tiene docker: {} config?                               │
│     SÍ → Continúa                                           │
│     NO → Salta servicio (o usa source.command)              │
└─────────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  3. ¿Qué source.kind tiene?                                 │
└─────────────────────────────────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        │                               │
        ▼                               ▼
┌───────────────┐              ┌───────────────┐
│ source.kind   │              │ source.kind   │
│ == "git"      │              │ == "image"    │
└───────────────┘              └───────────────┘
        │                               │
        ▼                               ▼
┌───────────────────┐          ┌───────────────────┐
│ Busca Dockerfile  │          │ Usa imagen        │
│ en repo clonado   │          │ source.image:tag  │
│                   │          │                   │
│ Construye imagen  │          │ Pull imagen       │
│ (docker build)    │          │ (docker pull)     │
└───────────────────┘          └───────────────────┘
        │                               │
        └───────────────┬───────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  4. Aplica configuración de docker: {}                      │
│     - Puertos                                                │
│     - Volúmenes                                              │
│     - Dependencias                                           │
│     - Modo (dev/prod)                                        │
└─────────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  5. Genera docker-compose.generated.yml                     │
│     - build: { context, dockerfile }  (si git/local)        │
│     - image: "..."                     (si image)            │
│     - ports, volumes, networks, etc.                         │
└─────────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  6. Ejecuta docker compose up                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Resumen

### ¿De dónde viene la imagen cuando uso `docker: {}`?

| `source.kind` | Origen de la Imagen | Proceso |
|--------------|---------------------|---------|
| **`git`** | Dockerfile en repositorio clonado | `docker build` desde código clonado |
| **`image`** | Docker Hub/Registry | `docker pull` de `source.image:tag` |
| **`local`** | Dockerfile en directorio local | `docker build` desde directorio local |

### ¿Qué hace `docker: {}`?

- ✅ Configura cómo se ejecuta el contenedor (puertos, volúmenes, modo, etc.)
- ✅ NO define de dónde viene la imagen (eso lo define `source.kind`)
- ✅ NO lee de `docker-compose.yml` existente del proyecto

### ¿Cómo usar un `docker-compose.yml` existente?

Usa `project.commands` en lugar de `docker: {}`:

```json
{
  "project": {
    "commands": {
      "up": "docker compose -f docker/docker-compose.yml up -d"
    }
  }
}
```

---

## Ejemplos Reales

### Ejemplo 1: Servicio Git con Docker

```json
{
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/empresa/api.git",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "dependsOn": ["postgres"]
      }
    }
  }
}
```

**Proceso**:
1. Clona `api` desde Git
2. Busca `Dockerfile.dev` en el repositorio clonado
3. Construye imagen: `docker build -f Dockerfile.dev`
4. Ejecuta contenedor con puertos y dependencias configuradas

---

### Ejemplo 2: Servicio Image con Docker

```json
{
  "services": {
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
    }
  }
}
```

**Proceso**:
1. Hace pull: `docker pull nginx:alpine`
2. Ejecuta contenedor directamente con esa imagen
3. Mapea puerto 80

---

### Ejemplo 3: Proyecto Local con docker-compose.yml Existente

```json
{
  "project": {
    "name": "gateway",
    "commands": {
      "up": "docker compose -f docker/docker-compose.yml up -d"
    }
  }
}
```

**Proceso**:
1. Raioz ejecuta tu `docker compose up -d` directamente
2. Tu `docker-compose.yml` existente gestiona todo
3. Raioz detecta y muestra los servicios en `raioz status` y `raioz logs`

---

## Referencias

- [Tipos de Servicios](./TIPOS_DE_SERVICIOS.md)
- [Casos de Uso](./casos-de-uso/)
- [Documentación de Docker](./COMMANDS.md)
