# Casos Reales de Uso de Raioz

## 1. Billing Dashboard: Onboarding y orquestacion multi-equipo

### Contexto

- 1 frontend (React), 5 microservicios backend, infra compartida (PostgreSQL, Redis, RabbitMQ)
- Equipo de 6 devs, cada uno toca 1-2 servicios
- Onboarding frecuente de nuevos desarrolladores

### Arquitectura

| Servicio              | Estado             | Rol       |
|-----------------------|--------------------|-----------|
| auth-service          | estable            | compartido|
| users-service         | desarrollo activo  | backend   |
| billing-service       | desarrollo activo  | core      |
| payments-service      | estable            | critico   |
| notifications-service | poco usado         | background|

### Problema sin Raioz

Onboarding de un dev nuevo:
1. Clonar 6 repos
2. Resolver versiones de Node/Go
3. Copiar `.env.example` a `.env` y ajustar puertos manualmente
4. Resolver conflictos de Docker
5. Preguntar "que servicios necesito?" y esperar ayuda

Tiempo real: **1-2 dias**. Errores frecuentes: servicios que no levantan, versiones incorrectas, "en mi maquina si funciona".

### Solucion con Raioz

```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-dashboard",
    "network": "raioz-net"
  },
  "services": {
    "users-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/users-service.git",
        "branch": "develop",
        "path": "users-service"
      },
      "docker": {
        "mode": "dev",
        "dockerfile": "Dockerfile.dev",
        "ports": ["3001:3000"],
        "dependsOn": ["postgres"]
      }
    },
    "billing-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/billing-service.git",
        "branch": "feature/taxes",
        "path": "billing-service"
      },
      "docker": {
        "mode": "dev",
        "dockerfile": "Dockerfile.dev",
        "ports": ["3002:3000"],
        "dependsOn": ["postgres", "rabbit"]
      }
    },
    "payments-service": {
      "source": {
        "kind": "image",
        "image": "org/payments-service",
        "tag": "2.4.1"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3003:3000"]
      }
    }
  },
  "infra": {
    "postgres": { "image": "postgres", "tag": "15" },
    "redis": { "image": "redis", "tag": "7" },
    "rabbit": { "image": "rabbitmq", "tag": "3-management" }
  }
}
```

Flujo del dev nuevo:

```bash
git clone git@github.com:org/billing-dashboard.git
cd billing-dashboard
raioz up
```

Raioz clona solo `users-service` y `billing-service`, usa la imagen de `payments-service`, levanta la infra compartida y centraliza `.env`. Tiempo real: **5-10 minutos**.

### Comparacion objetiva

| Metrica               | Sin Raioz    | Con Raioz   |
|------------------------|-------------|-------------|
| Onboarding             | 1-2 dias    | 10 min      |
| Repos clonados         | 6           | 2           |
| Errores de env         | frecuentes  | raros       |
| Conflictos de puertos  | comunes     | casi cero   |
| Cambio de proyecto     | doloroso    | trivial     |

---

## 2. Trabajo simultaneo en multiples microservicios

### Escenario

Proyecto con 6 microservicios (auth, users, payments, orders, notifications, search). Tu eres responsable de `users` y `payments`. Quieres modificar ambos y tenerlos corriendo al mismo tiempo sin romper el resto.

### Tres estados por servicio

Raioz distingue:

- **Local (editable)**: repo clonado, codigo vivo, hot reload
- **Imagen (estable)**: corre como imagen Docker
- **Desactivado**: ni clona ni levanta

### Configuracion

```json
{
  "project": "billing-platform",
  "services": {
    "users": {
      "mode": "local",
      "repo": "git@github.com:org/users.git",
      "branch": "feature/refactor"
    },
    "payments": {
      "mode": "local",
      "repo": "git@github.com:org/payments.git",
      "branch": "feature/new-flow"
    },
    "auth": {
      "mode": "image",
      "image": "org/auth:stable"
    },
    "orders": {
      "mode": "image",
      "image": "org/orders:stable"
    },
    "notifications": {
      "mode": "image",
      "image": "org/notifications:stable"
    },
    "search": {
      "mode": "disabled"
    }
  }
}
```

Al ejecutar `raioz up`: clona `users` y `payments` en modo desarrollo, levanta `auth`, `orders` y `notifications` como imagenes estables, ignora `search`.

### Sin colisiones

Raioz evita conflictos porque:
- Puertos asignados de forma deterministica
- Volumenes namespaced por proyecto (`raioz-billing-users` nunca choca con `raioz-otroproyecto-users`)
- Contenedores con prefijos unicos
- `.env` centralizado y versionado

### Servicios readonly (dependencias de otro equipo)

Cuando no hay registry y todo vive en Git, Raioz permite clonar repos de otros equipos como dependencias sin riesgo:

```json
{
  "auth": {
    "mode": "git",
    "repo": "git@github.com:org/auth.git",
    "branch": "main",
    "access": "readonly",
    "build": { "dockerfile": "Dockerfile" }
  }
}
```

Protecciones:
- El repo se clona en carpeta separada (`readonly/`)
- Raioz no hace `git checkout` ni `git pull` automatico
- Volumenes montados como `:ro` (Docker impide escritura)
- Si el servicio falla, se recrea sin afectar el stack

Cuando exista un registry, el cambio es minimo:

```json
{
  "auth": {
    "mode": "image",
    "image": "org/auth:1.12.3"
  }
}
```

Estructura del workspace:

```
/opt/raioz/workspaces/billing/
├── local/     # editable
└── readonly/  # dependencias
```

---

## 3. Dependencias anidadas: 6 microservicios con sub-dependencias

### Arquitectura

```
mi-proyecto
├── microservicio-1 (editable)
│   └── sub-microservicio-1a (readonly)
├── microservicio-2 (editable)
│   └── sub-microservicio-2a (readonly)
└── microservicio-3 (editable)
    └── sub-microservicio-3a (readonly)
```

Total: 6 microservicios (3 principales editables + 3 sub-dependencias readonly) + infra (PostgreSQL, Redis).

### Desglose de servicios

#### Principales (editables, modo dev con hot-reload)

| Servicio         | Puerto    | Depende de                       |
|------------------|-----------|----------------------------------|
| microservicio-1  | 3001:3000 | sub-microservicio-1a, database   |
| microservicio-2  | 3002:3000 | sub-microservicio-2a, redis      |
| microservicio-3  | 3003:3000 | sub-microservicio-3a, database   |

#### Sub-dependencias (readonly, modo prod estable)

| Servicio              | Puerto    | Depende de | Usado por        |
|-----------------------|-----------|------------|------------------|
| sub-microservicio-1a  | 3004:3000 | database   | microservicio-1  |
| sub-microservicio-2a  | 3005:3000 | redis      | microservicio-2  |
| sub-microservicio-3a  | 3006:3000 | database   | microservicio-3  |

### Diagrama de dependencias

```
┌─────────────┐      ┌─────────────┐
│  database   │      │    redis    │
│  (infra)    │      │   (infra)   │
└──────┬──────┘      └──────┬──────┘
       │                    │
  ┌────┴────┐          ┌────┴────┐
  │ sub-1a  │          │ sub-2a  │
  │(readonly)│         │(readonly)│
  └────┬────┘          └────┬────┘
       │                    │
  ┌────┴────┐          ┌────┴────┐       ┌────────┐
  │ micro-1 │          │ micro-2 │       │ sub-3a │
  │(editable)│         │(editable)│      │(readonly)│
  └─────────┘          └─────────┘       └────┬────┘
                                               │
                                          ┌────┴────┐
                                          │ micro-3 │
                                          │(editable)│
                                          └─────────┘
```

### Orden de inicio con `raioz up`

1. **Infra**: database (PostgreSQL 15), redis (Redis 7)
2. **Sub-microservicios** (readonly): sub-microservicio-1a, 2a, 3a
3. **Microservicios principales** (editables): microservicio-1, 2, 3

### Estructura de directorios

```
/opt/raioz-proyecto/workspaces/mi-proyecto/
├── local/services/
│   ├── microservicio-1/
│   ├── microservicio-2/
│   └── microservicio-3/
├── readonly/services/
│   ├── sub-microservicio-1a/
│   ├── sub-microservicio-2a/
│   └── sub-microservicio-3a/
├── .state.json
└── docker-compose.generated.yml
```

### Caracteristicas clave

- **Editables**: hot-reload, volumenes read-write, se actualizan automaticamente
- **Readonly**: protegidos con volumenes `:ro`, version estable fija, `restart: unless-stopped`
- **Dependencias**: Docker Compose resuelve el orden de inicio automaticamente

### Comandos utiles

```bash
raioz up                              # Levantar todo
raioz status                          # Ver estado de servicios
raioz logs microservicio-1 --follow   # Logs de un servicio
raioz check                           # Verificar configuracion
raioz down                            # Detener todo
```

---

## Conclusiones

Raioz no acelera Docker. Raioz acelera **decisiones**: hace explicito que servicios importan, cuales son estables, cuales estan en desarrollo y que depende de que.

Principios clave:
- **Orquestacion, no dependencia**: los microservicios siguen siendo autonomos
- **Control humano**: Raioz no fuerza cambios, solo avisa si hay drift
- **Infra como contrato**: cambios al config son PR visibles con impacto controlado
- **Sin lock-in**: los repos, docker-compose generado y el config JSON quedan intactos si Raioz desaparece
- **Ownership respetado**: los servicios de otros equipos se consumen como dependencias seguras y aisladas
