# Caso de Uso 3: Trabajar en Múltiples Servicios Simultáneamente

## 📋 Descripción

Un desarrollador necesita trabajar en varios microservicios al mismo tiempo, mientras otros servicios se mantienen estables. Este es el caso de uso principal de Raioz.

## 🎯 Objetivo

Permitir que un desarrollador:
- Trabaje en múltiples servicios en modo desarrollo (hot-reload)
- Mantenga otros servicios corriendo como imágenes estables
- No tenga que cambiar configuración al alternar entre servicios
- No rompa servicios de otros equipos

## 🔄 Flujo Completo

### Escenario Típico

**Desarrollador:** Juan
**Responsabilidad:** `users-service` y `payments-service`
**Otros servicios:** `auth-service`, `orders-service`, `notifications-service` (de otros equipos)

### Configuración en .raioz.json

```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-platform",
    "network": "billing-network"
  },
  "services": {
    "users-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/users-service.git",
        "branch": "feature/user-profiles",
        "path": "services/users",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3001:3000"],
        "dependsOn": ["database"]
      }
    },
    "payments-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/payments-service.git",
        "branch": "feature/new-payment-flow",
        "path": "services/payments",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3002:3000"],
        "dependsOn": ["database", "rabbit"]
      }
    },
    "auth-service": {
      "source": {
        "kind": "image",
        "image": "org/auth-service",
        "tag": "1.5.2"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3003:3000"]
      }
    },
    "orders-service": {
      "source": {
        "kind": "image",
        "image": "org/orders-service",
        "tag": "2.1.0"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3004:3000"]
      }
    },
    "notifications-service": {
      "source": {
        "kind": "image",
        "image": "org/notifications-service",
        "tag": "1.2.3"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3005:3000"]
      }
    }
  },
  "infra": {
    "database": {
      "image": "postgres",
      "tag": "15"
    },
    "rabbit": {
      "image": "rabbitmq",
      "tag": "3-management"
    }
  }
}
```

### Qué Hace Raioz

#### 1. Clona Solo Servicios Editables

**Se clonan:**
- ✅ `users-service` → `{base}/workspaces/billing-platform/local/services/users`
- ✅ `payments-service` → `{base}/workspaces/billing-platform/local/services/payments`

**No se clonan:**
- ❌ `auth-service` (usa imagen Docker)
- ❌ `orders-service` (usa imagen Docker)
- ❌ `notifications-service` (usa imagen Docker)

#### 2. Configura Modos Diferentes

**Servicios en modo `dev`:**
- Bind mounts para hot-reload
- Código local montado en contenedor
- Cambios se reflejan inmediatamente
- `restart: "no"` (sin auto-restart)

**Servicios en modo `prod`:**
- Solo imagen Docker (sin bind mounts)
- Estable, no cambia
- `restart: "unless-stopped"`
- Healthchecks estrictos

#### 3. Genera Docker Compose

**docker-compose.generated.yml:**
```yaml
services:
  users-service:
    container_name: raioz-billing-platform-users-service
    build:
      context: /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
      dockerfile: Dockerfile.dev
    volumes:
      - ./services/users:/app  # Hot-reload
    restart: "no"  # Dev mode

  payments-service:
    container_name: raioz-billing-platform-payments-service
    build:
      context: /opt/raioz-proyecto/workspaces/billing-platform/local/services/payments
      dockerfile: Dockerfile.dev
    volumes:
      - ./services/payments:/app  # Hot-reload
    restart: "no"  # Dev mode

  auth-service:
    container_name: raioz-billing-platform-auth-service
    image: org/auth-service:1.5.2
    restart: "unless-stopped"  # Prod mode
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## 🎯 Flujo de Trabajo del Desarrollador

### Día 1: Trabajar en users-service

**1. Modificar código:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
# Editar código...
```

**2. Ver cambios en tiempo real:**
- Hot-reload detecta cambios
- Servicio se recarga automáticamente
- No necesita reiniciar contenedor

**3. Ver logs:**
```bash
raioz logs users-service --follow
```

**4. Probar cambios:**
- Hacer requests a `http://localhost:3001`
- Ver cambios reflejados inmediatamente

**5. Hacer commit:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
git add .
git commit -m "feat: add user profiles"
git push
```

### Día 2: Cambiar a payments-service

**No necesita hacer nada especial:**
- `users-service` sigue corriendo
- `payments-service` ya está corriendo
- Solo cambia de directorio y trabaja

**1. Modificar código:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/payments
# Editar código...
```

**2. Ver cambios:**
- Hot-reload funciona igual
- Otros servicios no se afectan

### Día 3: Trabajar en ambos simultáneamente

**Puede trabajar en ambos:**
- Abre dos terminales
- Una para `users-service`
- Otra para `payments-service`
- Ambos con hot-reload activo

## 🔍 Detalles Técnicos

### Hot-Reload por Runtime

**Node.js:**
- Bind mount: `./service:/app`
- Comando: `npm run dev` (nodemon, etc.)
- Cambios en archivos se detectan automáticamente

**Go:**
- Bind mount: `./service:/app`
- Comando: `go run main.go` o `air` (hot-reload tool)
- Recompilación automática

**Python:**
- Bind mount: `./service:/app`
- Comando: `python -m flask run --reload`
- Recarga automática

**Java:**
- Bind mount: `./service:/app`
- Comando: `mvn spring-boot:run` con devtools
- Hot-reload con Spring Boot DevTools

### Aislamiento de Servicios

**Cada servicio tiene:**
- Su propio contenedor
- Su propio puerto
- Sus propios volúmenes
- Sus propias variables de entorno

**No hay conflictos porque:**
- Nombres de contenedores únicos: `raioz-{project}-{service}`
- Puertos determinísticos según `.raioz.json`
- Volúmenes namespaced por proyecto
- Red compartida pero aislada por proyecto

### Servicios Estables (Imágenes)

**Ventajas:**
- No consumen espacio de disco (solo imagen)
- No se actualizan accidentalmente
- Versión fija y estable
- No interfieren con desarrollo

**Cuándo usar:**
- Servicios de otros equipos
- Servicios que no necesitas modificar
- Servicios en versión estable

## 📊 Comparación: Sin vs Con Raioz

### ❌ Sin Raioz

**Problemas:**
- Tienes que clonar todos los repos
- Tienes que configurar cada servicio manualmente
- Cambios en un servicio pueden afectar otros
- Difícil alternar entre servicios
- Configuración duplicada

**Tiempo perdido:**
- Configurar entorno: 30-60 min
- Alternar entre servicios: 10-15 min cada vez
- Debuggear conflictos: variable

### ✅ Con Raioz

**Ventajas:**
- Solo clonas lo que necesitas
- Configuración centralizada
- Servicios aislados
- Alternar es instantáneo
- Sin conflictos

**Tiempo ahorrado:**
- Configurar entorno: 5-10 min (una vez)
- Alternar entre servicios: 0 min (ya están corriendo)
- Sin tiempo de debugging de conflictos

## 🎯 Ejemplo Real: Alternar Entre Servicios

### Escenario

**Juan necesita:**
1. Agregar feature en `users-service`
2. Arreglar bug en `payments-service`
3. Ambos deben funcionar juntos

### Flujo

**1. Levantar todo:**
```bash
raioz up
```

**2. Trabajar en users-service:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
# Editar código...
# Ver cambios en http://localhost:3001
```

**3. Cambiar a payments-service (sin detener nada):**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/payments
# Editar código...
# Ver cambios en http://localhost:3002
```

**4. Verificar que ambos funcionan:**
```bash
raioz status

# Salida:
# users-service    running  healthy  10m
# payments-service running  healthy  5m
```

**5. Probar integración:**
- `users-service` puede llamar a `payments-service`
- Ambos usan la misma base de datos
- Ambos usan el mismo RabbitMQ
- Todo funciona junto

## 🔍 Detalles de Aislamiento

### Puertos

**Cada servicio tiene su puerto:**
- `users-service`: `3001:3000`
- `payments-service`: `3002:3000`
- `auth-service`: `3003:3000`

**No hay conflictos:**
- Raioz valida que no hay conflictos antes de levantar
- Muestra error claro si hay conflicto
- Sugiere puerto alternativo

### Volúmenes

**Servicios en dev:**
- Bind mounts para hot-reload
- Código local montado en contenedor

**Servicios en prod:**
- Sin bind mounts
- Solo imagen Docker

**Volúmenes compartidos:**
- Base de datos: volumen compartido
- RabbitMQ: volumen compartido
- Redis: volumen compartido

### Variables de Entorno

**Cada servicio tiene sus propias variables:**
- `env/services/users-service.env`
- `env/services/payments-service.env`

**Variables compartidas:**
- `env/projects/billing-platform.env`
- `env/global.env`

**Precedencia:**
1. Service-specific (mayor)
2. Project-specific
3. Global (menor)

## ⚠️ Consideraciones

### Cambios en Dependencias

**Si cambias una dependencia:**
- Ejemplo: `users-service` ahora depende de `auth-service`
- Actualiza `.raioz.json`:
  ```json
  "users-service": {
    "docker": {
      "dependsOn": ["database", "auth-service"]
    }
  }
  ```
- Ejecuta `raioz up` de nuevo
- Raioz actualiza la configuración

### Cambios de Rama

**Si necesitas cambiar de rama:**
```bash
cd /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
git checkout feature/new-branch
```

**Raioz detecta el cambio:**
- Muestra warning sobre branch drift
- Puedes actualizar `.raioz.json` o mantener el cambio manual

### Servicios que Fallan

**Si un servicio falla:**
- En modo `dev`: no se reinicia automáticamente
- En modo `prod`: se reinicia automáticamente (`restart: unless-stopped`)

**Debugging:**
```bash
# Ver logs del servicio que falla
raioz logs users-service

# Ver estado
raioz status

# Reiniciar manualmente
docker restart raioz-billing-platform-users-service
```

## 📝 Mejores Prácticas

1. **Usa modo `dev` para servicios que desarrollas**
   - Hot-reload activo
   - Fácil debugging

2. **Usa modo `prod` para servicios estables**
   - No necesitas modificarlos
   - Versión fija

3. **Usa imágenes Docker para servicios de otros equipos**
   - No necesitas clonar sus repos
   - Versión estable

4. **Mantén `.raioz.json` actualizado**
   - Versiona en Git
   - Revisa en PRs

5. **Usa `raioz check` regularmente**
   - Detecta desalineaciones
   - Verifica que todo está correcto

## 🔗 Comandos Relacionados

- `raioz up`: Levantar todos los servicios
- `raioz status`: Ver estado de todos los servicios
- `raioz logs [service]`: Ver logs de un servicio específico
- `raioz check`: Verificar alineación
- `raioz down`: Detener todos los servicios
