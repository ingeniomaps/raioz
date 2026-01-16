# Resumen del Ejemplo `.raioz.json`

## 📋 Estructura del Proyecto

**Proyecto:** `mi-proyecto`

**Arquitectura:**
```
mi-proyecto
├── microservicio-1 (dependencia directa - EDITABLE)
│   └── sub-microservicio-1a (sub-dependencia - READONLY)
├── microservicio-2 (dependencia directa - EDITABLE)
│   └── sub-microservicio-2a (sub-dependencia - READONLY)
└── microservicio-3 (dependencia directa - EDITABLE)
    └── sub-microservicio-3a (sub-dependencia - READONLY)
```

**Total: 6 microservicios**
- 3 microservicios principales (listados directamente)
- 3 sub-microservicios (dependencias de los principales)

## 🔍 Desglose de Servicios

### Microservicios Principales (3)

1. **microservicio-1**
   - Tipo: Git (editable)
   - Puerto: 3001:3000
   - Modo: dev (hot-reload)
   - Depende de: `sub-microservicio-1a`, `database`

2. **microservicio-2**
   - Tipo: Git (editable)
   - Puerto: 3002:3000
   - Modo: dev (hot-reload)
   - Depende de: `sub-microservicio-2a`, `redis`

3. **microservicio-3**
   - Tipo: Git (editable)
   - Puerto: 3003:3000
   - Modo: dev (hot-reload)
   - Depende de: `sub-microservicio-3a`, `database`

### Sub-Microservicios (3)

4. **sub-microservicio-1a**
   - Tipo: Git (readonly)
   - Puerto: 3004:3000
   - Modo: prod (estable)
   - Depende de: `database`
   - Usado por: `microservicio-1`

5. **sub-microservicio-2a**
   - Tipo: Git (readonly)
   - Puerto: 3005:3000
   - Modo: prod (estable)
   - Depende de: `redis`
   - Usado por: `microservicio-2`

6. **sub-microservicio-3a**
   - Tipo: Git (readonly)
   - Puerto: 3006:3000
   - Modo: prod (estable)
   - Depende de: `database`
   - Usado por: `microservicio-3`

## 📊 Diagrama de Dependencias

```
┌─────────────┐      ┌─────────────┐
│  database   │      │    redis    │
│  (infra)    │      │   (infra)   │
└──────┬──────┘      └──────┬──────┘
       │                    │
       │                    │
┌──────▼────────────────────▼──────┐
│    sub-microservicio-1a          │ (readonly)
│    └─► depende de: database     │
└──────┬───────────────────────────┘
       │
┌──────▼───────────────────────────┐
│    microservicio-1               │ (editable)
│    └─► depende de:              │
│        - sub-microservicio-1a   │
│        - database               │
└──────────────────────────────────┘

┌──────▼──────┐
│ sub-microservicio-2a │ (readonly)
│ └─► depende de: redis │
└──────┬──────┘
       │
┌──────▼───────────────────────────┐
│    microservicio-2               │ (editable)
│    └─► depende de:              │
│        - sub-microservicio-2a   │
│        - redis                  │
└──────────────────────────────────┘

┌──────▼──────┐
│ sub-microservicio-3a │ (readonly)
│ └─► depende de: database │
└──────┬──────┘
       │
┌──────▼───────────────────────────┐
│    microservicio-3               │ (editable)
│    └─► depende de:              │
│        - sub-microservicio-3a   │
│        - database               │
└──────────────────────────────────┘
```

## 🔄 Qué Hace Raioz

### Al ejecutar `raioz up`:

1. **Clona microservicios principales (editables):**
   - `microservicio-1` → `{base}/workspaces/mi-proyecto/local/services/microservicio-1`
   - `microservicio-2` → `{base}/workspaces/mi-proyecto/local/services/microservicio-2`
   - `microservicio-3` → `{base}/workspaces/mi-proyecto/local/services/microservicio-3`

2. **Clona sub-microservicios (readonly):**
   - `sub-microservicio-1a` → `{base}/workspaces/mi-proyecto/readonly/services/sub-microservicio-1a`
   - `sub-microservicio-2a` → `{base}/workspaces/mi-proyecto/readonly/services/sub-microservicio-2a`
   - `sub-microservicio-3a` → `{base}/workspaces/mi-proyecto/readonly/services/sub-microservicio-3a`

3. **Levanta infraestructura:**
   - `database` (PostgreSQL 15)
   - `redis` (Redis 7)

4. **Resuelve dependencias y levanta servicios:**
   - Infraestructura primero (database, redis)
   - Sub-microservicios después (sub-microservicio-1a, sub-microservicio-2a, sub-microservicio-3a)
   - Microservicios principales al final (microservicio-1, microservicio-2, microservicio-3)

## 📁 Estructura de Directorios

```
/opt/raioz-proyecto/workspaces/mi-proyecto/
├── local/                    # Servicios editables
│   └── services/
│       ├── microservicio-1/
│       ├── microservicio-2/
│       └── microservicio-3/
├── readonly/                 # Servicios readonly
│   └── services/
│       ├── sub-microservicio-1a/
│       ├── sub-microservicio-2a/
│       └── sub-microservicio-3a/
├── .state.json
└── docker-compose.generated.yml
```

## 🎯 Características Clave

### Servicios Editables (microservicio-1, microservicio-2, microservicio-3)
- ✅ Hot-reload activo (modo dev)
- ✅ Puedes modificar código libremente
- ✅ Volúmenes montados como read-write
- ✅ Se actualizan automáticamente (checkout, pull)

### Servicios Readonly (sub-microservicio-1a, sub-microservicio-2a, sub-microservicio-3a)
- ✅ Protegidos de modificaciones (volúmenes `:ro`)
- ✅ No se actualizan automáticamente
- ✅ `restart: unless-stopped` (se recrean si fallan)
- ✅ Versión estable fija

### Dependencias
- Cada microservicio principal depende de su sub-microservicio
- Sub-microservicios dependen de infraestructura (database o redis)
- Docker Compose resuelve el orden de inicio automáticamente

## 🚀 Comandos Útiles

```bash
# Levantar todo el proyecto
raioz up

# Ver estado de todos los servicios
raioz status

# Ver logs de un microservicio principal
raioz logs microservicio-1 --follow

# Ver logs de un sub-microservicio
raioz logs sub-microservicio-1a --follow

# Ver logs de todos los servicios
raioz logs --all

# Verificar configuración
raioz check

# Detener todo
raioz down
```
