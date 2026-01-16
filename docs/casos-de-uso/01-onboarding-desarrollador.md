# Caso de Uso 1: Onboarding de un Desarrollador Nuevo

## 📋 Descripción

Un desarrollador nuevo se une al equipo y necesita configurar su entorno local para trabajar en el proyecto. Sin Raioz, esto puede tomar 1-2 días. Con Raioz, toma 5-10 minutos.

## 🎯 Objetivo

Permitir que un desarrollador nuevo pueda empezar a trabajar en el proyecto con un solo comando, sin necesidad de:

- Clonar múltiples repositorios manualmente
- Configurar variables de entorno en cada servicio
- Resolver conflictos de puertos
- Entender la arquitectura completa antes de empezar

## 🔄 Flujo Completo

### Paso 1: Clonar el repositorio del proyecto

```bash
git clone git@github.com:org/billing-dashboard.git
cd billing-dashboard
```

**Qué hace el desarrollador:**

- Clona solo el repositorio principal del proyecto
- No necesita clonar repositorios de microservicios individuales

**Qué contiene el repositorio:**

- `.raioz.json`: Archivo de configuración declarativa
- Código del proyecto (si es monolito) o configuración
- Documentación

### Paso 2: Ejecutar `raioz up`

```bash
raioz up
```

**Qué hace Raioz internamente:**

1. **Lee `.raioz.json`**

   - Valida el esquema JSON
   - Verifica que todos los campos requeridos estén presentes
   - Valida reglas de negocio (dependencias, puertos, etc.)

2. **Crea workspace**

   - Crea directorio base: `/opt/raioz-proyecto/` o `~/.raioz/`
   - Crea estructura de directorios:
     ```
     {base}/workspaces/{project}/
     ├── local/          # Servicios editables
     ├── readonly/       # Servicios readonly
     └── .state.json     # Estado del proyecto
     ```

3. **Clona repositorios necesarios**

   - Solo clona servicios con `source.kind: "git"` y `enabled: true`
   - Usa las ramas especificadas en `.raioz.json`
   - Clona en directorios separados según `access` (editable/readonly)

4. **Resuelve variables de entorno**

   - Lee archivos `.env` desde `/opt/raioz-proyecto/env/`
   - Aplica precedencia: global → project → service
   - Genera archivos `.env` consolidados para cada servicio

5. **Valida recursos Docker**

   - Verifica que las imágenes Docker existen (para `source.kind: "image"`)
   - Crea red Docker del proyecto si no existe
   - Crea volúmenes nombrados si es necesario

6. **Genera `docker-compose.generated.yml`**

   - Combina toda la configuración
   - Aplica modos dev/prod
   - Agrega nombres de contenedores normalizados
   - Aplica volúmenes readonly si corresponde

7. **Levanta servicios**

   - Ejecuta `docker compose up -d`
   - Espera a que los servicios estén corriendo
   - Verifica healthchecks (en modo prod)

8. **Guarda estado**
   - Guarda configuración en `.state.json`
   - Actualiza estado global en `{base}/state.json`

### Paso 3: Verificar que todo funciona

```bash
raioz status
```

**Qué muestra:**

- Estado de cada servicio (running/stopped)
- Health status (healthy/unhealthy/starting)
- Uptime
- Versión/commit de cada servicio
- Uso de recursos (CPU, memoria)

## 📊 Comparación: Sin vs Con Raioz

### ❌ Sin Raioz (Proceso Manual)

**Tiempo estimado: 1-2 días**

1. Clonar 6 repositorios manualmente
2. Resolver versiones de Node/Go/Python en cada servicio
3. Copiar `.env.example` → `.env` en cada servicio
4. Ajustar puertos manualmente para evitar conflictos
5. Resolver conflictos de Docker Compose
6. Preguntar al equipo "¿qué servicios necesito?"
7. Esperar ayuda de otros desarrolladores
8. Debuggear errores de configuración

**Errores comunes:**

- Servicios que no levantan
- Versiones incorrectas de dependencias
- "Funciona en mi máquina"
- Conflictos de puertos
- Variables de entorno incorrectas

### ✅ Con Raioz

**Tiempo estimado: 5-10 minutos**

1. Clonar repositorio del proyecto
2. Ejecutar `raioz up`
3. Esperar a que termine
4. Verificar con `raioz status`
5. Empezar a trabajar

**Ventajas:**

- ✅ Solo clona servicios necesarios
- ✅ Configuración centralizada
- ✅ Sin conflictos de puertos
- ✅ Variables de entorno resueltas automáticamente
- ✅ Reproducible entre desarrolladores

## 🎯 Ejemplo Real

### Escenario: Proyecto "Billing Dashboard"

**Desarrollador nuevo:** María
**Responsabilidad:** Trabajar en `users-service` y `billing-service`

**.raioz.json:**

```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-dashboard",
    "network": "billing-network"
  },
  "services": {
    "users-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/users-service.git",
        "branch": "develop",
        "path": "services/users",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3001:3000"],
        "dependsOn": ["postgres"]
      }
    },
    "billing-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/billing-service.git",
        "branch": "feature/taxes",
        "path": "services/billing",
        "access": "editable"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3002:3000"],
        "dependsOn": ["postgres", "rabbit"]
      }
    },
    "auth-service": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/auth-service.git",
        "branch": "main",
        "path": "services/auth",
        "access": "readonly"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3003:3000"]
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
        "ports": ["3004:3000"]
      }
    }
  },
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "15"
    },
    "redis": {
      "image": "redis",
      "tag": "7"
    },
    "rabbit": {
      "image": "rabbitmq",
      "tag": "3-management"
    }
  }
}
```

**Comandos ejecutados por María:**

```bash
# 1. Clonar proyecto
git clone git@github.com:org/billing-dashboard.git
cd billing-dashboard

# 2. Levantar entorno
raioz up

# Salida esperada:
# ✔ Workspace creado
# ✔ users-service clonado (develop)
# ✔ billing-service clonado (feature/taxes)
# ✔ auth-service clonado (readonly, main)
# ✔ payments-service usando imagen org/payments-service:2.4.1
# ✔ postgres levantado
# ✔ redis levantado
# ✔ rabbit levantado
# ✔ Entorno listo

# 3. Verificar estado
raioz status

# Salida esperada:
# Project: billing-dashboard
# Network: billing-network
#
# Services:
# users-service    running  healthy  5m  0.5% CPU  50MB  abc123def456
# billing-service  running  healthy  5m  0.3% CPU  45MB  def456ghi789
# auth-service      running  healthy  5m  0.2% CPU  40MB  ghi789jkl012
# payments-service  running  healthy  5m  0.4% CPU  55MB  2.4.1
```

**Resultado:**

- ✅ María puede empezar a trabajar inmediatamente
- ✅ Solo tiene clonados los servicios que necesita
- ✅ Todo está configurado y corriendo
- ✅ No necesita entender la arquitectura completa

## 🔍 Detalles Técnicos

### Qué se clona y qué no

**Se clona:**

- Servicios con `source.kind: "git"` y `enabled: true` (o sin `enabled`)
- En las ramas especificadas en `.raioz.json`

**No se clona:**

- Servicios con `source.kind: "image"` (solo se usa la imagen)
- Servicios con `enabled: false`
- Servicios filtrados por feature flags

### Dónde se clonan

**Servicios editables (`access: "editable"` o sin `access`):**

```
{base}/workspaces/{project}/local/{path}
```

**Servicios readonly (`access: "readonly"`):**

```
{base}/workspaces/{project}/readonly/{path}
```

### Variables de entorno

**Estructura:**

```
{base}/env/
├── global.env              # Variables globales
├── projects/
│   └── {project}.env      # Variables del proyecto
└── services/
    └── {service}.env      # Variables del servicio
```

**Precedencia:**

1. `services/{service}.env` (mayor precedencia)
2. `projects/{project}.env`
3. `global.env` (menor precedencia)

### Estado guardado

**Archivo:** `{base}/workspaces/{project}/.state.json`

**Contiene:**

- Configuración completa del proyecto
- Servicios activos
- Infra activa
- Usado para detectar cambios en ejecuciones futuras

## ⚠️ Posibles Problemas y Soluciones

### Problema: "port is already in use"

**Causa:** Otro proyecto está usando el mismo puerto.

**Solución:**

```bash
# Ver qué puertos están en uso
raioz ports

# Cambiar puerto en .raioz.json
"ports": ["3005:3000"]  # Usar puerto alternativo
```

### Problema: "branch does not exist"

**Causa:** La rama especificada no existe en el remoto.

**Solución:**

- Verificar el nombre de la rama en el repositorio
- Actualizar `.raioz.json` con la rama correcta
- O crear la rama en el repositorio

### Problema: "failed to clone repository"

**Causa:** Problemas de autenticación Git o repositorio no accesible.

**Solución:**

- Verificar que las SSH keys están configuradas
- Verificar acceso al repositorio
- Verificar que la URL del repositorio es correcta

## 📝 Checklist para el Desarrollador

- [ ] Tener Docker instalado y corriendo
- [ ] Tener Git configurado con SSH keys
- [ ] Tener `raioz` instalado (`curl -fsSL https://raioz.dev/install | sh`)
- [ ] Clonar repositorio del proyecto
- [ ] Ejecutar `raioz up`
- [ ] Verificar con `raioz status`
- [ ] Revisar logs si hay problemas: `raioz logs --all`

## 🎓 Aprendizajes Clave

1. **Un solo archivo de configuración**: `.raioz.json` contiene toda la información necesaria
2. **Declarativo**: No necesitas saber cómo funciona internamente, solo declarar qué necesitas
3. **Idempotente**: Puedes ejecutar `raioz up` múltiples veces sin problemas
4. **Reproducible**: Todos los desarrolladores obtienen el mismo entorno
5. **Incremental**: Solo clona y levanta lo necesario

## 🔗 Comandos Relacionados

- `raioz up`: Levantar proyecto
- `raioz status`: Ver estado de servicios
- `raioz logs`: Ver logs de servicios
- `raioz check`: Verificar alineación
- `raioz list`: Listar proyectos activos
