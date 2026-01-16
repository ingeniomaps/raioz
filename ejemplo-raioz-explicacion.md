# Ejemplo de `.raioz.json` - Proyecto con Dependencias Anidadas

## 📋 Estructura del Proyecto

**Proyecto:** `mi-proyecto`

**Arquitectura:**
```
mi-proyecto
├── microservicio-1 (dependencia directa)
│   └── sub-microservicio-1a (sub-dependencia)
├── microservicio-2 (dependencia directa)
│   └── sub-microservicio-2a (sub-dependencia)
└── microservicio-3 (dependencia directa, sin sub-dependencias)
```

**Total: 6 microservicios** (3 principales + 2 sub-dependencias + 1 adicional si necesitas 6)

## 🔍 Explicación de la Configuración

### Microservicios Principales (3)

#### 1. microservicio-1
- **Tipo:** Git (editable)
- **Puerto:** 3001:3000
- **Modo:** dev (hot-reload activo)
- **Dependencias:**
  - `sub-microservicio-1a` (su sub-dependencia)
  - `database` (infraestructura)
- **Acceso:** editable (puedes modificarlo)

#### 2. microservicio-2
- **Tipo:** Git (editable)
- **Puerto:** 3002:3000
- **Modo:** dev (hot-reload activo)
- **Dependencias:**
  - `sub-microservicio-2a` (su sub-dependencia)
  - `redis` (infraestructura)
- **Acceso:** editable (puedes modificarlo)

#### 3. microservicio-3
- **Tipo:** Imagen Docker (estable)
- **Puerto:** 3003:3000
- **Modo:** prod (sin hot-reload)
- **Dependencias:**
  - `database` (infraestructura)
- **Sin sub-dependencias**

### Sub-Microservicios (2)

#### 4. sub-microservicio-1a
- **Tipo:** Git (readonly)
- **Puerto:** 3004:3000
- **Modo:** prod (estable)
- **Dependencias:**
  - `database` (infraestructura)
- **Acceso:** readonly (protegido de modificaciones)
- **Uso:** Dependencia de `microservicio-1`

#### 5. sub-microservicio-2a
- **Tipo:** Git (readonly)
- **Puerto:** 3005:3000
- **Modo:** prod (estable)
- **Dependencias:**
  - `redis` (infraestructura)
- **Acceso:** readonly (protegido de modificaciones)
- **Uso:** Dependencia de `microservicio-2`

## 📊 Diagrama de Dependencias

```
┌─────────────────┐
│   database      │ (infra)
└────────┬────────┘
         │
    ┌────┴─────────────────────┐
    │                          │
┌───▼──────────┐      ┌────────▼────────┐
│microservicio-1│      │microservicio-3 │
└───┬──────────┘      └─────────────────┘
    │
    ├──► sub-microservicio-1a ──► database
    │

┌───▼──────────┐
│microservicio-2│
└───┬──────────┘
    │
    ├──► sub-microservicio-2a ──► redis
    │
┌───▼────┐
│ redis  │ (infra)
└────────┘
```

## 🔄 Qué Hace Raioz con Esta Configuración

### Al ejecutar `raioz up`:

1. **Clona microservicios principales (editables):**
   - `microservicio-1` → `{base}/workspaces/mi-proyecto/local/services/microservicio-1`
   - `microservicio-2` → `{base}/workspaces/mi-proyecto/local/services/microservicio-2`
   - `microservicio-3` → No se clona (es imagen Docker)

2. **Clona sub-microservicios (readonly):**
   - `sub-microservicio-1a` → `{base}/workspaces/mi-proyecto/readonly/services/sub-microservicio-1a`
   - `sub-microservicio-2a` → `{base}/workspaces/mi-proyecto/readonly/services/sub-microservicio-2a`

3. **Levanta infraestructura:**
   - `database` (PostgreSQL)
   - `redis`

4. **Resuelve dependencias:**
   - `microservicio-1` espera a que `sub-microservicio-1a` y `database` estén listos
   - `microservicio-2` espera a que `sub-microservicio-2a` y `redis` estén listos
   - `microservicio-3` espera a que `database` esté listo
   - `sub-microservicio-1a` espera a que `database` esté listo
   - `sub-microservicio-2a` espera a que `redis` esté listo

5. **Levanta todos los servicios en orden:**
   - Primero infraestructura (database, redis)
   - Luego sub-microservicios (sub-microservicio-1a, sub-microservicio-2a)
   - Finalmente microservicios principales (microservicio-1, microservicio-2, microservicio-3)

## 🎯 Casos de Uso

### Desarrollo Activo en microservicio-1

```bash
cd {base}/workspaces/mi-proyecto/local/services/microservicio-1
# Editar código...
# Cambios se reflejan automáticamente (hot-reload)
```

**Sub-microservicio-1a:**
- Está corriendo como dependencia
- No puedes modificarlo (readonly)
- Se recrea automáticamente si falla

### Desarrollo Activo en microservicio-2

```bash
cd {base}/workspaces/mi-proyecto/local/services/microservicio-2
# Editar código...
# Cambios se reflejan automáticamente (hot-reload)
```

**Sub-microservicio-2a:**
- Está corriendo como dependencia
- No puedes modificarlo (readonly)
- Se recrea automáticamente si falla

### microservicio-3 (Estable)

- Corre como imagen Docker
- No se clona
- Versión fija: `2.1.0`
- No se modifica

## 📝 Notas Importantes

### Dependencias Explicadas

**`dependsOn` en docker-compose:**
- Asegura orden de inicio correcto
- Docker Compose espera a que las dependencias estén "healthy" antes de iniciar
- No es necesariamente una dependencia funcional, sino de orquestación

### Sub-Microservicios como Servicios Independientes

Los sub-microservicios (`sub-microservicio-1a`, `sub-microservicio-2a`) son servicios **completos** en `.raioz.json`, no están "anidados" conceptualmente. Son servicios que:

- Tienen su propia configuración
- Tienen sus propios puertos
- Tienen sus propias dependencias
- Se levantan independientemente
- Son referenciados por otros servicios mediante `dependsOn`

### Readonly vs Editable

**Sub-microservicios en modo readonly:**
- Protegidos de modificaciones accidentales
- No se actualizan automáticamente
- Volúmenes montados como `:ro` (read-only)
- `restart: unless-stopped` (se recrean si fallan)

**Microservicios principales en modo editable:**
- Puedes modificar libremente
- Se actualizan automáticamente (checkout, pull)
- Volúmenes montados normalmente (read-write)
- Hot-reload activo en modo dev

## 🔍 Verificación

```bash
# Ver estado de todos los servicios
raioz status

# Ver logs de un microservicio principal
raioz logs microservicio-1

# Ver logs de un sub-microservicio
raioz logs sub-microservicio-1a

# Verificar alineación
raioz check
```

## 🚀 Comandos de Ejemplo

```bash
# Levantar todo el proyecto
raioz up

# Ver estado
raioz status

# Ver logs de un servicio específico
raioz logs microservicio-1 --follow

# Ver logs de todos los servicios
raioz logs --all

# Detener todo
raioz down

# Verificar configuración
raioz check
```
