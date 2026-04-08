# Ejemplo: Conflicto entre Servicio Clonado y Proyecto Principal

## Escenario

### Proyecto A: Plataforma Principal (clona servicios desde Git)

**Ubicación**: `/home/usuario/proyectos/plataforma/.raioz.json`

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "plataforma",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    }
  },
  "infra": {
    "postgres": {
      "image": "postgres",
      "tag": "18.1-alpine",
      "ip": "192.160.1.2"
    }
  },
  "services": {
    "nginx": {
      "source": {
        "kind": "git",
        "repo": "https://github.com/empresa/gateway.git",
        "branch": "develop",
        "path": "services/gateway",
        "access": "readonly"
      }
    }
  }
}
```

**Comportamiento**:
- Al hacer `raioz up` desde este proyecto, clona `nginx` desde Git
- Lo despliega en el workspace como servicio clonado
- El servicio corre desde: `/opt/raioz-proyecto/workspaces/roax/readonly/services/gateway`

---

### Proyecto B: Gateway Local (proyecto principal con docker-compose)

**Ubicación**: `/home/usuario/proyectos/gateway/.raioz.json`

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "nginx",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "docker compose -f docker/docker-compose.yml up -d",
      "down": "docker compose -f docker/docker-compose.yml down"
    },
    "env": ["."]
  },
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

**Comportamiento**:
- Este es un proyecto LOCAL (no está en el workspace de Raioz)
- Tiene su propio `docker-compose.yml` en `docker/docker-compose.yml`
- Al hacer `raioz up`, ejecuta `docker compose up -d` directamente

---

## Flujo Esperado (Comportamiento Ideal)

### Caso 1: Proyecto A se ejecuta primero

```bash
cd /home/usuario/proyectos/plataforma
raioz up
```

**Resultado**:
- ✅ Clona `nginx` desde Git
- ✅ Lo despliega en el workspace
- ✅ Crea contenedor: `raioz-roax-nginx` (desde compose generado)

---

### Caso 2: Proyecto B intenta ejecutarse (conflicto)

```bash
cd /home/usuario/proyectos/gateway
raioz up
```

**Comportamiento Esperado** (actualmente NO implementado):

```
⚠️  Conflict detected: service 'nginx' is already running

Service 'nginx' is currently running from:
  Project: plataforma
  Source: git (cloned service)
  Location: /opt/raioz-proyecto/workspaces/roax/readonly/services/gateway
  Container: raioz-roax-nginx

Your project 'nginx' wants to start from:
  Location: /home/usuario/proyectos/gateway
  Using: docker compose -f docker/docker-compose.yml up -d
  Container: nginx (from your compose)

Choose an action:
  [1] Stop cloned service and use local project (recommended for development)
  [2] Keep cloned service, skip local project
  [3] Cancel operation

Your choice [1-3]: 1

Stopping cloned service 'nginx' from workspace...
✅ Service stopped successfully

Starting local project 'nginx'...
✅ Local project started successfully

💾 Decision saved: Use local project for 'nginx' when conflict detected
```

**Después de guardar la decisión**:
- Se guarda en el estado de Raioz: preferencia para `nginx` = "local project"
- En futuras ejecuciones, automáticamente usará el proyecto local
- No pregunta de nuevo (a menos que el usuario borre la preferencia)

---

### Caso 3: Proyecto A se ejecuta después (conflicto inverso)

```bash
cd /home/usuario/proyectos/plataforma
raioz up
```

**Comportamiento Esperado** (actualmente NO implementado):

```
⚠️  Conflict detected: service 'nginx' has preference

Service 'nginx' has a saved preference to use:
  Local project at: /home/usuario/proyectos/gateway
  Currently running: Yes (container: nginx)

Your project 'plataforma' wants to use:
  Cloned service from Git
  Would create container: raioz-roax-nginx

Choose an action:
  [1] Stop local project and use cloned service (from workspace)
  [2] Keep local project, skip cloned service in this run
  [3] Update preference to always use cloned service
  [4] Cancel operation

Your choice [1-4]: 1

Stopping local project 'nginx'...
✅ Local project stopped

Starting cloned service 'nginx' from workspace...
✅ Cloned service started

💾 Decision saved: Use cloned service for 'nginx' when conflict detected
```

---

## Estado Actual vs Ideal

### ✅ Lo que SÍ funciona actualmente

1. **Detección de proyecto duplicado** (mismo nombre de proyecto completo):
   - Si proyecto "nginx" está corriendo desde workspace
   - Y ejecutas proyecto local "nginx"
   - Pregunta si quieres detener el del workspace

2. **Solo funciona para proyectos con `project.commands`** (proyectos locales)

### ❌ Lo que NO funciona actualmente

1. **Detección de servicio duplicado**:
   - No detecta si un SERVICIO específico está corriendo desde otro proyecto
   - Solo detecta si el proyecto completo con el mismo nombre está corriendo

2. **Guardar preferencias**:
   - No guarda decisiones sobre qué usar (local vs clonado)
   - Siempre pregunta cada vez

3. **Detección inversa**:
   - No detecta si un servicio local está corriendo cuando otro proyecto intenta clonarlo

---

## Implementación Necesaria

Para que funcione como se espera, necesitarías:

### 1. Detectar servicios por nombre de contenedor

En lugar de solo detectar proyectos completos, detectar servicios individuales:

```go
// Pseudo-código
func detectServiceConflict(serviceName string) (*ServiceConflict, error) {
    // Buscar contenedores con nombre que coincida:
    // - raioz-{workspace}-{service} (desde workspace)
    // - {service} o {prefix}-{service} (desde proyecto local)

    // Verificar si está corriendo desde workspace o proyecto local
    // Retornar información del conflicto
}
```

### 2. Guardar preferencias de servicio

```go
type ServicePreference struct {
    ServiceName string
    Preference  string // "local" | "cloned" | "ask"
    ProjectPath string // Para proyecto local
    Timestamp   time.Time
}
```

### 3. Preguntar y aplicar decisión

- Mostrar opciones claras
- Aplicar la decisión (detener uno, iniciar otro)
- Guardar preferencia para futuras ejecuciones

---

## Solución Temporal Actual

Mientras no está implementado, puedes:

1. **Siempre usar el proyecto local**:
   - Detener manualmente el servicio clonado antes de ejecutar el local
   - `raioz down` desde el proyecto que clonó el servicio

2. **Usar diferentes nombres de proyecto**:
   - Cambiar `project.name` en uno de los proyectos para evitar conflictos

3. **Usar diferentes nombres de contenedor**:
   - En tu `docker-compose.yml`, usa `container_name` diferente
   - Ejemplo: `container_name: gateway-nginx` en lugar de `nginx`

---

## Recomendación

El comportamiento que describes es el **ideal** y debería implementarse. Actualmente Raioz tiene la base (detección de proyecto duplicado) pero falta extenderlo para:

1. ✅ Detectar servicios individuales (no solo proyectos completos)
2. ✅ Guardar preferencias por servicio
3. ✅ Detectar conflictos en ambas direcciones (local → clonado y clonado → local)
