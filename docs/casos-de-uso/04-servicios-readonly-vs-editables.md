# Caso de Uso 4: Servicios Readonly vs Editables

## 📋 Descripción

Un desarrollador necesita consumir servicios de otros equipos sin riesgo de modificarlos accidentalmente. Raioz distingue entre servicios editables (que desarrollas) y servicios readonly (que solo consumes).

## 🎯 Objetivo

Proteger servicios de otros equipos de modificaciones accidentales mientras se mantiene la capacidad de desarrollarlos localmente cuando es necesario.

## 🔄 Concepto Clave

**Principio:** "No todo servicio que levanto es un servicio que controlo"

**Separación explícita:**
- **Servicios editables**: Bajo tu control, puedes modificar
- **Servicios readonly**: Dependencias, solo lectura
- **Servicios externos**: Imágenes Docker, completamente externos

## 🔍 Diferencia Entre Modos

### Servicios Editables (`access: "editable"` o sin `access`)

**Características:**
- ✅ Se clonan en `{base}/workspaces/{project}/local/`
- ✅ Se actualizan automáticamente (checkout, pull)
- ✅ Volúmenes montados normalmente (read-write)
- ✅ Puedes modificar código
- ✅ Puedes hacer commits
- ✅ Hot-reload activo en modo dev

**Cuándo usar:**
- Servicios que desarrollas activamente
- Servicios de tu equipo
- Servicios que necesitas modificar

### Servicios Readonly (`access: "readonly"`)

**Características:**
- ✅ Se clonan en `{base}/workspaces/{project}/readonly/`
- ❌ NO se actualizan automáticamente (no checkout, no pull)
- ✅ Volúmenes montados como `:ro` (read-only)
- ❌ Docker impide escribir en el código
- ✅ `restart: unless-stopped` (se recrean si fallan)
- ✅ Protegidos de modificaciones accidentales

**Cuándo usar:**
- Servicios de otros equipos
- Dependencias externas
- Servicios que quieres proteger
- Servicios que solo consumes

## 🔄 Flujo Completo

### Configuración en .raioz.json

```json
{
  "services": {
    "users-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/users-service.git",
        "branch": "develop",
        "path": "services/users"
        // Sin "access" = editable por defecto
      },
      "docker": {
        "mode": "dev"
      }
    },
    "auth-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/auth-service.git",
        "branch": "main",
        "path": "services/auth",
        "access": "readonly"  // ← Servicio readonly
      },
      "docker": {
        "mode": "prod"
      }
    }
  }
}
```

### Qué Hace Raioz

#### 1. Clonado de Repositorios

**Servicio editable (`users-service`):**
```bash
# Se clona en:
/opt/raioz-proyecto/workspaces/billing-platform/local/services/users

# Comportamiento:
- Clona si no existe
- Hace checkout de la rama especificada
- Detecta drift de rama
- Hace pull si es necesario
```

**Servicio readonly (`auth-service`):**
```bash
# Se clona en:
/opt/raioz-proyecto/workspaces/billing-platform/readonly/services/auth

# Comportamiento:
- Clona SOLO si no existe
- NO hace checkout automático
- NO hace pull automático
- NO actualiza si ya existe
```

**Mensajes:**
```
✔ users-service clonado (develop)
ℹ️  auth-service ya existe (readonly), saltando actualización
```

#### 2. Generación de Docker Compose

**Servicio editable:**
```yaml
users-service:
  build:
    context: /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
  volumes:
    - ./services/users:/app  # Read-write
  restart: "no"  # Dev mode
```

**Servicio readonly:**
```yaml
auth-service:
  build:
    context: /opt/raioz-proyecto/workspaces/billing-platform/readonly/services/auth
  volumes:
    - ./services/auth:/app:ro  # Read-only (Docker impide escribir)
  restart: "unless-stopped"  # Se recrea si falla
```

#### 3. Protecciones Implementadas

**Protección 1: Volumen Read-Only**
- Docker monta el volumen con `:ro`
- Docker literalmente impide escribir
- Si el contenedor intenta escribir, falla

**Protección 2: Workspace Separado**
- Editables en `local/`
- Readonly en `readonly/`
- Separación física clara

**Protección 3: Sin Actualizaciones Automáticas**
- No hace `git checkout`
- No hace `git pull`
- No sobreescribe cambios locales (si existen)

**Protección 4: Recreación Automática**
- `restart: unless-stopped`
- Si el servicio falla, se recrea
- Inmutable, descartable

## 🎯 Ejemplo Real

### Escenario

**Desarrollador:** María
**Equipo:** Backend (users, payments)
**Dependencias:** auth (otro equipo), orders (otro equipo)

**.raioz.json:**
```json
{
  "services": {
    "users-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/users-service.git",
        "branch": "develop",
        "path": "services/users"
        // Editable (sin access)
      }
    },
    "payments-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/payments-service.git",
        "branch": "feature/new-flow",
        "path": "services/payments"
        // Editable (sin access)
      }
    },
    "auth-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/auth-service.git",
        "branch": "main",
        "path": "services/auth",
        "access": "readonly"  // ← De otro equipo
      }
    },
    "orders-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/orders-service.git",
        "branch": "main",
        "path": "services/orders",
        "access": "readonly"  // ← De otro equipo
      }
    }
  }
}
```

### Ejecución

```bash
$ raioz up

✔ users-service clonado (develop)
✔ payments-service clonado (feature/new-flow)
ℹ️  auth-service ya existe (readonly), saltando actualización
ℹ️  orders-service ya existe (readonly), saltando actualización
✔ Servicios iniciados
```

### Estructura Resultante

```
/opt/raioz-proyecto/workspaces/billing-platform/
├── local/                    # Servicios editables
│   ├── services/
│   │   ├── users/            # ← María puede modificar
│   │   └── payments/          # ← María puede modificar
├── readonly/                  # Servicios readonly
│   ├── services/
│   │   ├── auth/              # ← Protegido, no modificar
│   │   └── orders/            # ← Protegido, no modificar
└── docker-compose.generated.yml
```

### Comportamiento en Práctica

**María trabaja en users-service:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
# Editar código...
# Hacer commits...
# Push cambios...
```

**María NO puede modificar auth-service:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/readonly/services/auth
# Intentar editar código...
# Docker monta como :ro, cambios no se reflejan
# Si intenta escribir, Docker lo impide
```

**Si auth-service falla:**
- Se recrea automáticamente (`restart: unless-stopped`)
- Vuelve a su estado original
- No se contamina el stack

## 🔍 Detalles Técnicos

### Volúmenes Read-Only

**Configuración en Docker Compose:**
```yaml
volumes:
  - ./services/auth:/app:ro  # :ro = read-only
```

**Qué significa:**
- El contenedor puede LEER archivos
- El contenedor NO puede ESCRIBIR archivos
- Docker impide cualquier escritura
- Si el código intenta escribir, falla

**Ejemplo de error si intenta escribir:**
```
Error: EROFS: read-only file system, open '/app/config.json'
```

### Workspace Separado

**Ventajas:**
- Separación física clara
- Fácil identificar qué es editable
- No hay confusión sobre qué modificar
- Migración automática desde estructura antigua

### Sin Actualizaciones Automáticas

**Por qué:**
- Evita romper el stack
- Respeta ownership de otros equipos
- No hace cambios no deseados
- Control humano sobre actualizaciones

**Si necesitas actualizar un servicio readonly:**
```bash
# Opción 1: Eliminar y re-clonar
rm -rf /opt/raioz-proyecto/workspaces/billing-platform/readonly/services/auth
raioz up  # Se clonará de nuevo

# Opción 2: Cambiar a editable temporalmente
# Cambiar access: "readonly" a access: "editable" en .raioz.json
# Actualizar manualmente
# Volver a readonly
```

### Recreación Automática

**Comportamiento:**
- Si el contenedor falla, Docker lo recrea
- Vuelve a su estado original
- No acumula errores
- Inmutable, descartable

**Ventajas:**
- No se rompe permanentemente
- Fácil recuperarse de errores
- Consistente con el concepto de "dependencia"

## 📊 Comparación: Readonly vs Editable vs Image

| Característica | Editable | Readonly | Image |
|----------------|----------|----------|-------|
| Clonado | Sí | Sí (solo si no existe) | No |
| Actualización automática | Sí | No | No |
| Volúmenes | Read-write | Read-only | N/A |
| Modificar código | Sí | No | No |
| Hot-reload | Sí (dev) | No | No |
| Restart policy | `no` (dev) | `unless-stopped` | `unless-stopped` |
| Ubicación | `local/` | `readonly/` | N/A |
| Uso típico | Desarrollo activo | Dependencias | Servicios estables |

## 🎯 Casos de Uso Específicos

### Caso 1: Servicio de Otro Equipo

**Escenario:** Necesitas `auth-service` del equipo de seguridad.

**Configuración:**
```json
"auth-service": {
  "source": {
    "kind": "git",
    "repo": "git@github.com:org/auth-service.git",
    "branch": "main",
    "access": "readonly"
  }
}
```

**Resultado:**
- Se clona una vez
- No se actualiza automáticamente
- Protegido de modificaciones
- Se recrea si falla

### Caso 2: Servicio que Quieres Proteger

**Escenario:** Tienes un servicio crítico que no quieres modificar accidentalmente.

**Configuración:**
```json
"payments-service": {
  "source": {
    "kind": "git",
    "repo": "git@github.com:org/payments-service.git",
    "branch": "stable",
    "access": "readonly"
  }
}
```

**Resultado:**
- Protegido de cambios accidentales
- Versión estable fija
- No se actualiza sin querer

### Caso 3: Alternar Entre Readonly y Editable

**Escenario:** Normalmente consumes `auth-service`, pero hoy necesitas modificarlo.

**Solución:**
1. Cambiar `access: "readonly"` a `access: "editable"` en `.raioz.json`
2. Ejecutar `raioz up`
3. Raioz migra el servicio de `readonly/` a `local/`
4. Ahora puedes modificarlo
5. Cuando termines, volver a `readonly`

## ⚠️ Advertencias y Limitaciones

### No Puedes Forzar Actualización

**Si un servicio readonly está desactualizado:**
- No puedes hacer `git pull` automático
- Debes eliminarlo manualmente y re-clonar
- O cambiar temporalmente a editable

### No Puedes Modificar Código

**Si intentas modificar un servicio readonly:**
- Docker impide escribir
- Cambios no se reflejan
- Debes cambiar a editable primero

### No Hay Hot-Reload

**Servicios readonly:**
- No tienen hot-reload
- Cambios requieren rebuild
- Mejor usar modo `prod`

## 📝 Mejores Prácticas

1. **Usa readonly para servicios de otros equipos**
   - Protege de modificaciones accidentales
   - Respeta ownership

2. **Usa editable para servicios que desarrollas**
   - Hot-reload activo
   - Fácil modificar

3. **Usa imágenes Docker para servicios muy estables**
   - No necesitas clonar
   - Versión fija

4. **Documenta en el equipo qué servicios son readonly**
   - Evita confusión
   - Clarifica ownership

5. **Revisa `.raioz.json` en PRs**
   - Cambios de `access` son visibles
   - Permite discusión sobre ownership

## 🔗 Comandos Relacionados

- `raioz up`: Clona y levanta servicios (respeta readonly)
- `raioz status`: Muestra qué servicios son readonly
- `raioz check`: Verifica alineación (incluye readonly)
- `raioz down`: Detiene servicios (readonly y editables)
