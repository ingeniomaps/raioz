# Escenario: Gateway como Servicio y Trabajo Manual

## 📋 Escenario del Usuario

Tienes dos situaciones:

1. **Proyecto A**: Tiene `gateway` configurado como servicio Git (clonado por Raioz)
2. **Tu trabajo directo**: Clonas `gateway` manualmente y lo ejecutas sin `.raioz.json` (modo manual)

**Pregunta**: ¿Qué pasa? ¿Debería siempre usar Raioz o Raioz debería manejar esto mejor?

---

## 🔄 Situaciones Posibles

### Situación 1: Gateway Corriendo desde Proyecto A

```
Estado Inicial:
┌─────────────────────────────────────────────────┐
│ Proyecto A (dashboard)                          │
│   - Tiene gateway como servicio Git             │
│   - Gateway está corriendo desde workspace      │
│   - Contenedor: raioz-roax-gateway              │
└─────────────────────────────────────────────────┘

Tú quieres trabajar directamente en gateway:
┌─────────────────────────────────────────────────┐
│ ~/dev/gateway/                                  │
│   - Clonado manualmente                         │
│   - Sin .raioz.json                             │
│   - Quieres ejecutar: docker compose up         │
│   - Contenedor: nginx (según docker-compose.yml)│
└─────────────────────────────────────────────────┘
```

**Pregunta**: ¿Qué pasa cuando ejecutas `docker compose up` en `~/dev/gateway/`?

---

## ✅ Cómo Funciona Actualmente

### Detección de Conflictos en Raioz

Raioz **sí detecta conflictos** cuando:

1. **Proyecto A ejecuta `raioz up`**:
   - Detecta si `gateway` está corriendo localmente (fuera de workspace)
   - Pregunta: "¿Quieres detener el local y usar el clonado?"
   - Guarda preferencia si el usuario elige

2. **Tú ejecutas `docker compose up` manualmente**:
   - Docker Compose no detecta automáticamente conflictos con Raioz
   - Pueden correr **dos contenedores al mismo tiempo** (conflicto de puertos)
   - Pueden tener **nombres de contenedor diferentes** (no conflicto directo)

### Problemas Potenciales

#### ❌ Problema 1: Puertos en Uso

Si ambos contenedores usan los mismos puertos (`80:80`, `443:443`):

```bash
# Proyecto A ejecuta raioz up
raioz up
# → Gateway clonado usa puertos 80:80, 443:443
# → Contenedor: raioz-roax-gateway

# Tú ejecutas manualmente
cd ~/dev/gateway
docker compose -f docker/docker-compose.yml up
# → ERROR: Bind for 0.0.0.0:80 failed: port is already allocated
```

**Resultado**: El manual falla porque los puertos ya están en uso.

#### ❌ Problema 2: Contenedores Simultáneos (si puertos diferentes)

Si cambias los puertos en tu `docker-compose.yml` local:

```yaml
# docker-compose.yml local
services:
  nginx:
    ports:
      - '8080:80'    # Puerto diferente
      - '8443:443'   # Puerto diferente
```

**Resultado**: Ambos corren simultáneamente, pero:
- Gateway clonado: `http://localhost` (puerto 80)
- Gateway local: `http://localhost:8080` (puerto 8080)

**Problema**: Confusión sobre cuál está activo.

#### ❌ Problema 3: Estado Desincronizado

Raioz no sabe que estás ejecutando `gateway` manualmente:
- `raioz status` muestra solo el gateway clonado
- No detecta que hay otro gateway corriendo manualmente
- No sincroniza estados

---

## 🎯 Soluciones Recomendadas

### Opción 1: Usar Raioz Siempre (RECOMENDADO)

**Mejor práctica**: Usa Raioz tanto para el proyecto como para trabajar en gateway.

#### Configuración para Gateway (trabajo directo)

**`.raioz.json` en `~/dev/gateway/`**:

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "gateway-dev",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker/docker-compose.yml down"
    },
    "env": ["."]
  },
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

**Ventajas**:
- ✅ Raioz detecta conflictos automáticamente
- ✅ Puedes trabajar en gateway directamente
- ✅ Si Proyecto A está corriendo, Raioz te pregunta qué hacer
- ✅ Puedes guardar preferencia ("siempre usar local")
- ✅ `raioz status` muestra todo correctamente

**Flujo de trabajo**:

```bash
# Trabajar en gateway
cd ~/dev/gateway
raioz up
# → Raioz detecta: "Gateway ya está corriendo desde workspace (Proyecto A)"
# → Pregunta: "¿Quieres detener el clonado y usar el local?"
# → Tu respuesta: "Sí, y guarda esta preferencia"
# → Raioz detiene el clonado y ejecuta tu local

# Trabajar en proyecto A
cd ~/dev/dashboard
raioz up
# → Raioz detecta: "Gateway ya está corriendo localmente (trabajo directo)"
# → Aplica preferencia: "siempre usar local"
# → Salta clonar gateway (usa el local que ya está corriendo)

# Ver estado completo
raioz status
# → Muestra: gateway-dev (local, corriendo) ✅
# → Muestra: dashboard (proyecto) ✅
```

---

### Opción 2: Trabajo Manual con Detección de Puertos

Si prefieres trabajar manualmente sin Raioz:

#### Verificar si Gateway ya está corriendo

```bash
# Antes de docker compose up, verifica puertos
docker ps | grep -E '80:80|443:443'
# Si hay algo, detén el contenedor de Raioz primero

# O verificar por nombre
docker ps | grep gateway
```

#### Detener Gateway de Raioz antes de trabajar manualmente

```bash
# Detener gateway clonado por Proyecto A
docker stop raioz-roax-gateway

# Trabajar manualmente
cd ~/dev/gateway
docker compose -f docker/docker-compose.yml up -d
```

**Problemas**:
- ❌ Manual y propenso a errores
- ❌ No detecta conflictos automáticamente
- ❌ Debes recordar detener/arrancar manualmente
- ❌ Raioz no sabe del cambio de estado

---

### Opción 3: Usar Puertos Diferentes para Desarrollo

Si quieres trabajar manualmente **sin detener** el gateway de Raioz:

#### Modificar `docker-compose.yml` local temporalmente

```yaml
# docker-compose.yml (para desarrollo local)
services:
  nginx:
    ports:
      - '8080:80'    # Puerto diferente para desarrollo
      - '8443:443'   # Puerto diferente para desarrollo
    # ... resto de configuración
```

**Ventajas**:
- ✅ Ambos pueden correr simultáneamente
- ✅ No hay conflicto de puertos
- ✅ Puedes probar cambios sin afectar el gateway de producción

**Problemas**:
- ⚠️ Debes recordar cambiar puertos de vuelta antes de commit
- ⚠️ URLs diferentes (localhost vs localhost:8080)
- ⚠️ Otros servicios pueden estar configurados para usar puerto 80

---

## 🏆 Recomendación Final

### ✅ Para Desarrollo Local Directo: Usar Raioz

**Mejor práctica**: Crea un `.raioz.json` en `~/dev/gateway/` y usa Raioz:

```json
{
  "project": {
    "name": "gateway-dev",
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker/docker-compose.yml down"
    }
  }
}
```

**Por qué**:
1. ✅ Raioz detecta conflictos automáticamente
2. ✅ Puedes guardar preferencias ("siempre usar local")
3. ✅ El proyecto A respeta tu preferencia automáticamente
4. ✅ `raioz status` muestra todo correctamente
5. ✅ Más fácil de mantener

**Flujo de trabajo**:

```bash
# Trabajar en gateway
cd ~/dev/gateway
raioz up
# → Detiene gateway clonado (si está corriendo)
# → Ejecuta tu local
# → Guarda preferencia: "siempre usar local para gateway"

# Proyecto A ejecuta raioz up
cd ~/dev/dashboard
raioz up
# → Detecta que gateway ya está corriendo localmente
# → Aplica preferencia: "siempre usar local"
# → Salta clonar gateway
# → Usa el gateway local que ya está corriendo

# Ver todo
raioz status
# → gateway-dev (local, corriendo) ✅
# → dashboard (proyecto, corriendo) ✅
# →   └─ gateway: usando local (preferencia) ✅
```

---

## 🔄 Comparación: Usar Raioz vs Manual

| Aspecto | Usar Raioz (`.raioz.json`) | Manual (`docker compose`) |
|---------|---------------------------|--------------------------|
| **Detección de conflictos** | ✅ Automática | ❌ Manual |
| **Sincronización de estado** | ✅ `raioz status` muestra todo | ❌ Raioz no sabe |
| **Guardar preferencias** | ✅ Automático | ❌ No disponible |
| **Compatibilidad con proyectos** | ✅ Proyecto A respeta preferencias | ❌ Puede causar conflictos |
| **Facilidad de uso** | ✅ `raioz up` / `raioz down` | ⚠️ Debes recordar detener/arrancar |
| **Flexibilidad** | ✅ Configurable | ✅ Más control manual |

---

## 📝 Ejemplo Completo: Trabajo en Gateway

### Configuración Inicial

**`~/dev/gateway/.raioz.json`**:

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "gateway-dev",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker/docker-compose.yml down",
      "health": "docker compose -f docker/docker-compose.yml exec nginx wget --quiet --tries=1 --spider http://localhost/health || exit 1"
    },
    "env": ["."]
  },
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

### Flujo de Trabajo

```bash
# 1. Trabajar en gateway
cd ~/dev/gateway
raioz up

# Si Proyecto A tiene gateway corriendo:
# ⚠️  Conflict detected: service 'gateway' is already running
# Current status:
#   Container: raioz-roax-gateway
#   Source: git (workspace)
# Your project wants to run from:
#   Location: ~/dev/gateway
#   Container: nginx
#
# Choose an action:
#   [1] Stop cloned service and use local project (recommended for development)
#   [2] Keep cloned service, skip local project
#   [3] Update preference to always use local project
#
# Tu elección: 1 (o 3 para guardar preferencia)

# 2. Gateway local está corriendo
raioz status
# → gateway-dev (local, corriendo) ✅

# 3. Proyecto A ejecuta raioz up
cd ~/dev/dashboard
raioz up
# → Detecta que gateway ya está corriendo localmente
# → Aplica preferencia: "siempre usar local para gateway"
# → Salta clonar gateway
# → Usa el gateway local

# 4. Ver estado completo
raioz status
# → gateway-dev (local, corriendo) ✅
# → dashboard (proyecto, corriendo) ✅
# →   └─ gateway: usando local (preferencia) ✅

# 5. Detener todo
cd ~/dev/gateway
raioz down
# → Detiene gateway local

cd ~/dev/dashboard
raioz down
# → Detiene dashboard
# → Si gateway era dependencia, puede clonarlo de nuevo (o no, según preferencia)
```

---

## 🎯 Resumen

### ✅ Recomendación Final

**Para trabajar directamente en gateway**:

1. ✅ **Crea un `.raioz.json` en `~/dev/gateway/`** con `project.commands`
2. ✅ **Usa Raioz** (`raioz up` / `raioz down`)
3. ✅ **Guarda preferencias** cuando Raioz pregunte ("siempre usar local")
4. ✅ **El proyecto A respetará tu preferencia** automáticamente

**Ventajas**:
- ✅ Detección automática de conflictos
- ✅ Sincronización de estado
- ✅ Preferencias guardadas
- ✅ Compatible con proyectos que usan gateway

**❌ No recomiendado**: Trabajar manualmente con `docker compose` sin Raioz, porque:
- ❌ No detecta conflictos automáticamente
- ❌ Puede causar conflictos de puertos
- ❌ Raioz no sabe del cambio de estado
- ❌ Debes gestionar manualmente detener/arrancar

---

## Referencias

- [Recomendación Gateway](./RECOMENDACION_GATEWAY.md)
- [Mejores Prácticas Gateway](./BEST_PRACTICES_GATEWAY.md)
- [Ejemplos de Conflictos](./ejemplos/conflicto-servicios.md)
