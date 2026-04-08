# Dockerfiles en Raioz: ¿Dockerfile.dev o Dockerfile?

## Respuesta Rápida

**Por defecto**: Raioz busca `Dockerfile.dev` (no `Dockerfile`)

**Puedes especificar**: Cualquier nombre usando `docker.dockerfile`

---

## Comportamiento por Defecto

Si NO especificas `docker.dockerfile` en la configuración:

```json
{
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "...",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev"
        // dockerfile no especificado
      }
    }
  }
}
```

**Raioz busca**: `Dockerfile.dev` en el repositorio/directorio del servicio.

---

## Especificar un Dockerfile Personalizado

Puedes especificar cualquier nombre de Dockerfile:

```json
{
  "docker": {
    "mode": "dev",
    "dockerfile": "Dockerfile"  // Especifica explícitamente
  }
}
```

**Raioz busca**: `Dockerfile` (el nombre que especificaste).

---

## Opciones Disponibles

### Opción 1: Usar `Dockerfile.dev` (Recomendado)

**Estructura del proyecto**:
```
mi-servicio/
├── Dockerfile.dev    ← Para desarrollo
├── Dockerfile        ← Para producción (opcional)
├── src/
└── package.json
```

**Configuración**:
```json
{
  "docker": {
    "mode": "dev"
    // Por defecto busca Dockerfile.dev
  }
}
```

✅ **Ventajas**:
- Separación clara entre dev y prod
- Por defecto en Raioz
- Convención estándar

---

### Opción 2: Usar `Dockerfile` (Estándar)

**Estructura del proyecto**:
```
mi-servicio/
├── Dockerfile        ← Único Dockerfile
├── src/
└── package.json
```

**Configuración**:
```json
{
  "docker": {
    "mode": "dev",
    "dockerfile": "Dockerfile"  // Especifica explícitamente
  }
}
```

✅ **Ventajas**:
- Nombre estándar de Docker
- Compatible con herramientas estándar
- Único archivo para dev y prod

---

### Opción 3: Dockerfile por Modo

**Estructura del proyecto**:
```
mi-servicio/
├── Dockerfile.dev    ← Para desarrollo
├── Dockerfile.prod   ← Para producción
├── src/
└── package.json
```

**Configuración para dev**:
```json
{
  "docker": {
    "mode": "dev",
    "dockerfile": "Dockerfile.dev"  // Explícito
  }
}
```

**Configuración para prod**:
```json
{
  "docker": {
    "mode": "prod",
    "dockerfile": "Dockerfile.prod"  // Explícito
  }
}
```

✅ **Ventajas**:
- Múltiples Dockerfiles para diferentes modos
- Control total sobre build
- Optimizaciones específicas por modo

---

### Opción 4: Sin Dockerfile (Wrapper Automático)

Si NO tienes Dockerfile pero tienes `docker.command`, Raioz genera un wrapper automático:

**Estructura del proyecto**:
```
mi-servicio/
├── src/              ← Sin Dockerfile
└── package.json
```

**Configuración**:
```json
{
  "docker": {
    "mode": "dev",
    "command": "npm start",
    "runtime": "node"
    // No especificas dockerfile
  }
}
```

**Raioz genera automáticamente**:
```dockerfile
FROM node:22-alpine

WORKDIR /app

# Copy project files
COPY . .

# Install dependencies (if package.json, go.mod, requirements.txt exist)
RUN if [ -f package.json ]; then npm install --legacy-peer-deps || npm install --force --legacy-peer-deps || npm install --force || true; fi

# Run the command
CMD npm start
```

✅ **Ventajas**:
- No necesitas escribir Dockerfile
- Ideal para proyectos simples
- Automático para Node.js, Go, Python, etc.

⚠️ **Limitaciones**:
- Menos control sobre el build
- Wrapper temporal (se regenera cada vez)

---

## Flujo de Decisión: Qué Dockerfile Usa Raioz

```
┌─────────────────────────────────────────────┐
│ 1. ¿Tienes docker.dockerfile especificado? │
└─────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┐
    │                   │
    ▼                   ▼
   SÍ                  NO
    │                   │
    ▼                   ▼
┌─────────────┐   ┌─────────────────────┐
│ Usa ese     │   │ Busca "Dockerfile. │
│ nombre      │   │ dev" (default)      │
└─────────────┘   └─────────────────────┘
    │                   │
    │                   ▼
    │            ┌──────────────┐
    │            │ ¿Existe?     │
    │            └──────────────┘
    │                   │
    │        ┌──────────┴──────────┐
    │        │                     │
    │        ▼                     ▼
    │       SÍ                    NO
    │        │                     │
    │        │              ┌──────────────┐
    │        │              │ ¿Tienes      │
    │        │              │ docker.      │
    │        │              │ command?     │
    │        │              └──────────────┘
    │        │                     │
    │        │          ┌──────────┴──────────┐
    │        │          │                     │
    │        │          ▼                     ▼
    │        │         SÍ                    NO
    │        │          │                     │
    │        │          ▼                     ▼
    │        │    ┌─────────────┐      ┌─────────────┐
    │        │    │ Genera      │      │ ERROR:      │
    │        │    │ wrapper     │      │ Necesita    │
    │        │    │ automático  │      │ Dockerfile  │
    │        │    └─────────────┘      │ o command   │
    │        │                         └─────────────┘
    │        │
    └────────┴─────► Usa el Dockerfile encontrado/generado
```

---

## Ejemplos Reales

### Ejemplo 1: Proyecto con `Dockerfile.dev`

**Estructura**:
```
api-service/
├── Dockerfile.dev
├── src/
└── package.json
```

**`.raioz.json`**:
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
        "mode": "dev"
        // Busca Dockerfile.dev automáticamente
      }
    }
  }
}
```

**Resultado**: ✅ Usa `Dockerfile.dev` del repositorio

---

### Ejemplo 2: Proyecto con solo `Dockerfile`

**Estructura**:
```
api-service/
├── Dockerfile        ← Solo este archivo
├── src/
└── package.json
```

**`.raioz.json`**:
```json
{
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "...",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "dockerfile": "Dockerfile"  // Especifica explícitamente
      }
    }
  }
}
```

**Resultado**: ✅ Usa `Dockerfile` del repositorio

---

### Ejemplo 3: Proyecto sin Dockerfile (Wrapper Automático)

**Estructura**:
```
worker-service/
├── src/
└── package.json     ← Sin Dockerfile
```

**`.raioz.json`**:
```json
{
  "services": {
    "worker": {
      "source": {
        "kind": "git",
        "repo": "...",
        "path": "services/worker"
      },
      "docker": {
        "mode": "dev",
        "command": "node worker.js",
        "runtime": "node"
        // No especificas dockerfile
      }
    }
  }
}
```

**Resultado**: ✅ Raioz genera wrapper automático con `node:22-alpine`

---

## Resumen: ¿Qué Necesitas?

| Tu Situación | Qué Hacer |
|--------------|-----------|
| Tienes `Dockerfile.dev` | No hacer nada (es el default) |
| Tienes solo `Dockerfile` | Especifica `"dockerfile": "Dockerfile"` |
| Tienes `Dockerfile.prod`, `Dockerfile.staging`, etc. | Especifica `"dockerfile": "Dockerfile.prod"` |
| No tienes Dockerfile | Usa `docker.command` + `docker.runtime` (genera wrapper) |
| Quieres control total | Crea `Dockerfile.dev` y configúralo manualmente |

---

## Recomendaciones

### Para Desarrollo Local

✅ **Usa `Dockerfile.dev`**:
- Por defecto en Raioz
- Separación clara dev/prod
- Convención estándar

### Para Producción

✅ **Usa `Dockerfile` o `Dockerfile.prod`**:
- Especifica explícitamente `"dockerfile": "Dockerfile.prod"`
- Optimizaciones específicas para producción
- Multi-stage builds si es necesario

### Para Proyectos Simples

✅ **Sin Dockerfile (wrapper automático)**:
- Usa `docker.command` + `docker.runtime`
- Raioz genera todo automáticamente
- Ideal para proyectos Node.js, Go, Python simples

---

## Referencias

- [Cómo Funciona Docker](./COMO_FUNCIONA_DOCKER.md)
- [Tipos de Servicios](./TIPOS_DE_SERVICIOS.md)
