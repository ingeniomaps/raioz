# Ejemplo: Configurar Gateway como Servicio

Este documento muestra cómo configurar `gateway` como un servicio Git en otro proyecto de Raioz.

---

## 📋 Escenario

Tienes dos proyectos:

1. **`dashboard`** (proyecto principal): Quiere usar `gateway` como servicio
2. **`gateway`** (servicio): Está en un repositorio Git

---

## 🎯 Configuración del Proyecto que Usa Gateway

### Proyecto: `dashboard`

**Ubicación**: `~/dev/dashboard/`

**`.raioz.json`**:

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "dashboard",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    }
  },
  "services": {
    "gateway": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/gateway.git",
        "branch": "main",
        "path": "services/gateway",
        "access": "readonly"
      },
      "docker": {
        "mode": "prod",
        "ports": ["80:80", "443:443"],
        "dependsOn": []
      },
      "env": ["services/gateway"]
    },
    "api": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/api.git",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "dependsOn": ["gateway"]
      },
      "env": ["services/api"]
    }
  },
  "infra": {},
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

**¿Qué hace esto?**:
- ✅ Clona `gateway` desde Git al workspace de Raioz
- ✅ Lo ejecuta como servicio dentro del proyecto `dashboard`
- ✅ El gateway se despliega automáticamente cuando ejecutas `raioz up` en `dashboard`

---

## 🔄 Flujo de Trabajo

### 1. Primera vez: Clonar y Desplegar

```bash
cd ~/dev/dashboard
raioz up
```

**Raioz hace**:
1. Clona `gateway` desde Git a `/opt/raioz-proyecto/workspaces/roax/local/services/gateway`
2. Clona `api` desde Git a `/opt/raioz-proyecto/workspaces/roax/local/services/api`
3. Construye las imágenes Docker (si tienen `Dockerfile.dev`)
4. Genera `docker-compose.generated.yml`
5. Ejecuta `docker compose up -d`
6. Gateway y API quedan corriendo

**Resultado**:
```
✅ dashboard (proyecto)
   ├─ gateway (servicio Git, corriendo) 🟢
   └─ api (servicio Git, corriendo) 🟢
```

### 2. Trabajar en Gateway Directamente

Si quieres trabajar directamente en `gateway`:

```bash
# Clonar gateway manualmente
cd ~/dev
git clone https://github.com/tu-org/gateway.git
cd gateway

# Crear .raioz.json para trabajo directo
cat > .raioz.json << 'EOF'
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
      "down": "cd docker && docker compose -f docker-compose.yml down"
    },
    "env": ["."]
  },
  "services": {},
  "infra": {},
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
EOF

# Ejecutar
raioz up
```

**Raioz detecta conflicto**:
```
⚠️  Conflict detected: service 'gateway' is already running

Current status:
  Container: raioz-roax-gateway
  Source: git (workspace)
  Project: dashboard

Your project wants to run from:
  Location: ~/dev/gateway
  Container: nginx

Choose an action:
  [1] Stop cloned service and use local project (recommended for development)
  [2] Keep cloned service, skip local project
  [3] Update preference to always use local project

Choose an action: 3
```

**Resultado**:
- ✅ Guarda preferencia: "siempre usar local para gateway"
- ✅ Detiene el gateway clonado
- ✅ Ejecuta tu gateway local
- ✅ La próxima vez que `dashboard` ejecute `raioz up`, usará tu gateway local

---

## 📝 Ejemplo Completo: Proyecto con Múltiples Servicios

### Proyecto: `platform`

**`.raioz.json`**:

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "platform",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    }
  },
  "services": {
    "gateway": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/gateway.git",
        "branch": "main",
        "path": "services/gateway",
        "access": "readonly"
      },
      "docker": {
        "mode": "prod",
        "ports": ["80:80", "443:443"],
        "dependsOn": []
      },
      "env": ["services/gateway"]
    },
    "auth-service": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/auth-service.git",
        "branch": "main",
        "path": "services/auth",
        "access": "readonly"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3001:3000"],
        "dependsOn": ["gateway"]
      },
      "env": ["services/auth"]
    },
    "api-service": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/api-service.git",
        "branch": "develop",
        "path": "services/api",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "dependsOn": ["gateway", "auth-service"]
      },
      "env": ["services/api"]
    },
    "frontend": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/frontend.git",
        "branch": "main",
        "path": "services/frontend",
        "access": "readonly"
      },
      "docker": {
        "mode": "prod",
        "ports": ["8080:80"],
        "dependsOn": ["gateway"]
      },
      "env": ["services/frontend"]
    }
  },
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "15-alpine",
      "ports": ["5432:5432"],
      "volumes": ["postgres_data:/var/lib/postgresql/data"],
      "env": ["infra/postgres"]
    },
    "redis": {
      "image": "redis",
      "tag": "7-alpine",
      "ports": ["6379:6379"],
      "volumes": ["redis_data:/data"],
      "env": ["infra/redis"]
    }
  },
  "env": {
    "useGlobal": true,
    "files": [".env", "projects/platform"]
  }
}
```

**Estructura generada**:

```
/opt/raioz-proyecto/workspaces/roax/
├── local/
│   └── services/
│       ├── gateway/          ← Clonado desde Git (readonly)
│       ├── auth/             ← Clonado desde Git (readonly)
│       ├── api/              ← Clonado desde Git (editable)
│       └── frontend/         ← Clonado desde Git (readonly)
├── docker-compose.generated.yml
└── .raioz.state.json
```

---

## 🔍 Diferencias: Gateway como Servicio vs Gateway como Proyecto

### Gateway como Servicio (Clonado desde Git)

**Configuración**:
```json
{
  "services": {
    "gateway": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/gateway.git",
        "branch": "main",
        "path": "services/gateway"
      },
      "docker": {
        "mode": "prod",
        "ports": ["80:80", "443:443"]
      }
    }
  }
}
```

**Características**:
- ✅ Se clona automáticamente desde Git
- ✅ Se despliega como parte del proyecto
- ✅ Puede tener `Dockerfile.dev` o usar wrapper automático
- ✅ Se actualiza automáticamente al cambiar de branch
- ✅ Puede ser `readonly` o `editable`

**Ubicación**:
- Clonado a: `/opt/raioz-proyecto/workspaces/roax/local/services/gateway`

---

### Gateway como Proyecto (Trabajo Directo)

**Configuración**:
```json
{
  "project": {
    "name": "gateway-dev",
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker-compose.yml down"
    }
  }
}
```

**Características**:
- ✅ Trabajas directamente en el código
- ✅ Usa tu `docker-compose.yml` existente
- ✅ Ejecuta `installer.sh` automáticamente
- ✅ Más control sobre el despliegue

**Ubicación**:
- Tu directorio: `~/dev/gateway/`

---

## 🎯 Comparación: Cuándo Usar Cada Enfoque

| Situación | Usar Gateway como... |
|-----------|---------------------|
| **Desarrollo en gateway** | Proyecto (con `project.commands`) |
| **Gateway como dependencia** | Servicio (clonado desde Git) |
| **Proyecto que usa gateway** | Servicio en su `.raioz.json` |
| **Gateway para producción** | Servicio (clonado, readonly) |
| **Gateway para modificar** | Proyecto (editable localmente) |

---

## 🔄 Ejemplo de Flujo Completo

### Escenario: Desarrollar en Gateway y Usar en Dashboard

```bash
# 1. Trabajar en gateway directamente
cd ~/dev/gateway
# Editar código, hacer cambios
vim conf/services/app.conf

# 2. Ejecutar gateway localmente
raioz up
# → Raioz pregunta: "Gateway ya está corriendo desde workspace"
# → Tú eliges: "Detener clonado, usar local, y guardar preferencia"

# 3. Probar cambios
curl http://localhost/health

# 4. Hacer commit y push
git add .
git commit -m "Update gateway config"
git push origin main

# 5. Dashboard usa gateway (después de push)
cd ~/dev/dashboard
raioz up
# → Detecta preferencia: "usar local si está corriendo"
# → Si tu gateway local está corriendo, lo usa
# → Si no, clona la nueva versión desde Git

# 6. Ver estado completo
raioz status
# → gateway-dev (local, corriendo) ✅
# → dashboard (proyecto, corriendo) ✅
# →   └─ gateway: usando local (preferencia) ✅
```

---

## 📝 Resumen

### Para usar Gateway como Servicio

**En el proyecto que usa gateway** (ej: `dashboard`):

```json
{
  "services": {
    "gateway": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/tu-org/gateway.git",
        "branch": "main",
        "path": "services/gateway"
      },
      "docker": {
        "mode": "prod",
        "ports": ["80:80", "443:443"]
      }
    }
  }
}
```

### Para trabajar directamente en Gateway

**En el proyecto gateway** (ej: `~/dev/gateway/`):

```json
{
  "project": {
    "name": "gateway-dev",
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker-compose.yml down"
    }
  },
  "services": {},
  "infra": {}
}
```

---

## Referencias

- [Escenario Gateway Manual](./ESCENARIO_GATEWAY_MANUAL.md)
- [Recomendación Gateway](./RECOMENDACION_GATEWAY.md)
- [Tipos de Servicios](./TIPOS_DE_SERVICIOS.md)
