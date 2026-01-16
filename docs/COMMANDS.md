# Guía Completa de Comandos

Esta guía documenta todos los comandos disponibles en Raioz con ejemplos de uso y casos de uso comunes.

## Tabla de Contenidos

- [Comandos Básicos](#comandos-básicos)
- [Comandos de Gestión](#comandos-de-gestión)
- [Comandos Avanzados](#comandos-avanzados)
- [Comandos de CI/CD](#comandos-de-cicd)
- [Comandos de Utilidad](#comandos-de-utilidad)

---

## Comandos Básicos

### `raioz up`

Levanta todos los servicios del proyecto definidos en `.raioz.json`.

**Sintaxis:**

```bash
raioz up [flags]
```

**Flags:**

- `--config, -c`: Ruta al archivo de configuración (default: `.raioz.json`)
- `--project, -p`: Nombre del proyecto (alternativa a `--config`)
- `--profile`: Perfil a usar (filtra servicios por perfil)
- `--force-reclone`: Fuerza re-clonado de todos los repositorios Git
- `--dry-run`: Muestra qué se haría sin ejecutar cambios

**Ejemplos:**

```bash
# Levantar proyecto con configuración por defecto
raioz up

# Usar configuración personalizada
raioz up --config custom-raioz.json

# Levantar solo servicios del perfil frontend
raioz up --profile frontend

# Levantar solo servicios del perfil backend
raioz up --profile backend

# Forzar re-clonado de repositorios (útil después de cambios en ramas)
raioz up --force-reclone

# Ver qué se haría sin ejecutar (dry-run)
raioz up --dry-run
```

**Qué hace:**

1. Valida la configuración (JSON Schema + validaciones de negocio)
2. Ejecuta preflight checks (Docker, Git, espacio en disco)
3. Resuelve workspace y adquiere lock
4. Clona/actualiza repositorios Git necesarios
5. Resuelve variables de entorno
6. **Inicia servicios host** (si hay servicios con `execution: "host"`)
7. Valida y descarga imágenes Docker
8. Crea redes y volúmenes Docker
9. Genera `docker-compose.generated.yml`
10. Levanta servicios con Docker Compose
11. Guarda el estado del proyecto

**Casos de uso:**

- Onboarding inicial: `raioz up` levanta todo el entorno
- Después de cambios en `.raioz.json`: `raioz up` detecta cambios y actualiza
- Cambio de rama: `raioz up --force-reclone` actualiza repositorios
- Desarrollo con perfiles: `raioz up --profile frontend` para trabajar solo frontend

**Ejecución Host (sin Docker):**

Los servicios pueden ejecutarse directamente en el host (sin Docker) especificando `source.command` en la configuración. Si `source.command` existe, el servicio se ejecuta directamente en el host y la sección `docker` es opcional. Esto es útil para servicios que no necesitan contenedores Docker o para desarrollo local.

**Ejemplo de configuración host:**

```json
{
  "services": {
    "mi-servicio-host": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/mi-servicio.git",
        "branch": "main",
        "path": "services/mi-servicio",
        "command": "npm run dev",
        "runtime": "node"
      },
      "env": ["services/mi-servicio"]
    }
  }
}
```

**Ejemplo de configuración Docker (comportamiento original):**

```json
{
  "services": {
    "mi-servicio-docker": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/mi-servicio.git",
        "branch": "main",
        "path": "services/mi-servicio"
      },
      "docker": {
        "mode": "dev",
        "command": "npm run dev",
        "runtime": "node",
        "ports": ["3000:3000"]
      },
      "env": ["services/mi-servicio"]
    }
  }
}
```

**Características de ejecución host:**

- El servicio se ejecuta directamente en el host (no en Docker)
- Requiere el campo `source.command` para especificar el comando a ejecutar
- Opcionalmente puede especificar `source.runtime` para documentación
- La sección `docker` es opcional cuando hay `source.command`
- Las variables de entorno se resuelven igual que para servicios Docker
- Los logs se guardan en `workspace/logs/host/<servicio>.stdout.log` y `stderr.log`
- Los procesos se detienen automáticamente con `raioz down`
- **Nota:** Solo servicios con `source.kind: "git"` pueden ejecutarse en host. Servicios con `source.kind: "image"` requieren Docker.

**Diferencias entre `source.command` y `docker.command`:**

- `source.command`: Se ejecuta directamente en el host (sin Docker). Activa modo host.
- `docker.command`: Se ejecuta dentro del contenedor Docker. Solo se usa si no hay `source.command`.

---

### `raioz down`

Detiene todos los servicios del proyecto.

**Sintaxis:**

```bash
raioz down [flags]
```

**Flags:**

- `--config, -c`: Ruta al archivo de configuración
- `--project, -p`: Nombre del proyecto

**Ejemplos:**

```bash
# Detener proyecto actual
raioz down

# Detener proyecto específico
raioz down --project my-project
```

**Qué hace:**

1. Adquiere lock para evitar ejecuciones concurrentes
2. **Detiene servicios host** (si hay servicios con `source.command`)
3. Detiene servicios con Docker Compose
4. Elimina archivo de estado (`.state.json`)
5. Mantiene redes y volúmenes para reutilización

**Nota:** Las redes y volúmenes se mantienen para acelerar el próximo `raioz up`. Usa `raioz clean` si necesitas limpiarlos.

---

### `raioz status`

Muestra el estado detallado de todos los servicios del proyecto.

**Sintaxis:**

```bash
raioz status [flags]
```

**Flags:**

- `--config, -c`: Ruta al archivo de configuración
- `--project, -p`: Nombre del proyecto
- `--json`: Output en formato JSON (útil para scripts)

**Ejemplos:**

```bash
# Ver estado en formato tabla
raioz status

# Output en JSON para procesamiento
raioz status --json

# Estado de proyecto específico
raioz status --project my-project
```

**Información mostrada:**

- **NAME**: Nombre del servicio
- **STATUS**: Estado (running/stopped)
- **HEALTH**: Estado de salud (healthy/unhealthy/starting/none)
- **UPTIME**: Tiempo desde que se inició
- **CPU**: Uso de CPU
- **MEMORY**: Uso de memoria
- **VERSION**: Versión/commit del servicio
- **UPDATED**: Última actualización

**Ejemplo de salida:**

```
━━━ PROJECT STATUS ━━━

  Project: my-project
  Network: my-network

▸ Services
NAME      STATUS    HEALTH    UPTIME    CPU    MEMORY    VERSION    UPDATED
────      ──────    ──────    ──────    ───    ──────    ───────    ───────
api       running   healthy   2h 15m    5%     120MB     abc123     2024-01-15 10:30
frontend  running   healthy   2h 15m    3%     80MB      def456     2024-01-15 10:30
database  running   healthy   2h 15m    1%     200MB     15.0       2024-01-15 10:30
```

---

### `raioz logs`

Muestra logs de uno o más servicios.

**Sintaxis:**

```bash
raioz logs [service...] [flags]
```

**Flags:**

- `--all, -a`: Ver logs de todos los servicios
- `--follow, -f`: Seguir logs en tiempo real (similar a `tail -f`)
- `--tail, -n`: Número de líneas a mostrar (default: todas)
- `--config, -c`: Ruta al archivo de configuración
- `--project, -p`: Nombre del proyecto

**Ejemplos:**

```bash
# Ver logs de un servicio
raioz logs api

# Ver logs de múltiples servicios
raioz logs api frontend

# Ver logs de todos los servicios
raioz logs --all

# Seguir logs en tiempo real
raioz logs --follow api

# Ver últimas 100 líneas
raioz logs --tail 100 api

# Seguir logs de múltiples servicios
raioz logs --follow api frontend database
```

**Casos de uso:**

- Debugging: `raioz logs api` para ver errores
- Monitoreo: `raioz logs --follow --all` para monitorear todo
- Troubleshooting: `raioz logs --tail 200 service` para ver contexto

---

## Comandos de Gestión

### `raioz list`

Lista todos los proyectos activos desde el estado global.

**Sintaxis:**

```bash
raioz list [flags]
```

**Flags:**

- `--json`: Output en formato JSON
- `--filter <pattern>`: Filtrar proyectos por nombre (búsqueda parcial, case-insensitive)
- `--status <status>`: Filtrar proyectos por estado de servicios (running, stopped)

**Ejemplos:**

```bash
# Listar proyectos activos
raioz list

# Output en JSON
raioz list --json

# Filtrar por nombre
raioz list --filter billing

# Filtrar por estado
raioz list --status running

# Combinar filtros
raioz list --filter api --status running
```

**Información mostrada:**

- Nombre del proyecto
- Ruta del workspace
- Última ejecución (formato relativo)
- Cantidad de servicios activos
- Cantidad de servicios corriendo

**Ejemplo de salida:**

```
━━━ ACTIVE PROJECTS ━━━

▸ billing-platform
  Workspace: /opt/raioz-proyecto/workspaces/billing-platform
  Last Execution: 2 hours ago
  Active Services: 5
  Running: 5/5

▸ auth-service
  Workspace: /opt/raioz-proyecto/workspaces/auth-service
  Last Execution: 1 day ago
  Active Services: 3
  Running: 2/3
```

---

### `raioz ports`

Lista todos los puertos en uso por proyectos activos.

**Sintaxis:**

```bash
raioz ports [flags]
```

**Flags:**

- `--project, -p`: Filtrar por proyecto específico

**Ejemplos:**

```bash
# Ver todos los puertos activos
raioz ports

# Ver puertos de un proyecto específico
raioz ports --project my-project
```

**Ejemplo de salida:**

```
━━━ ACTIVE PORTS ━━━

PORT    PROJECT          SERVICE
────    ───────          ───────
3000    my-project       api
5432    my-project       database
8080    other-project    frontend
```

**Casos de uso:**

- Detectar conflictos de puertos antes de `raioz up`
- Ver qué puertos están en uso
- Debugging de problemas de conectividad

---

### `raioz clean`

Limpia recursos Docker no usados.

**Sintaxis:**

```bash
raioz clean [flags]
```

**Flags:**

- `--all`: Limpiar todos los proyectos
- `--images`: Eliminar imágenes Docker no usadas
- `--volumes`: Eliminar volúmenes Docker no usados (requiere confirmación o `--force`)
- `--networks`: Eliminar redes Docker no usadas
- `--dry-run`: Mostrar qué se limpiaría sin ejecutar
- `--force`: Saltar confirmaciones
- `--config, -c`: Ruta al archivo de configuración
- `--project, -p`: Nombre del proyecto

**Ejemplos:**

```bash
# Limpiar proyecto actual (solo contenedores detenidos)
raioz clean

# Limpiar todos los proyectos
raioz clean --all

# Limpiar imágenes no usadas
raioz clean --images

# Limpiar volúmenes no usados (preguntará confirmación)
raioz clean --volumes

# Limpiar volúmenes sin confirmación
raioz clean --volumes --force

# Limpiar todo (proyectos, imágenes, volúmenes, redes)
raioz clean --all --images --volumes --networks

# Ver qué se limpiaría sin ejecutar
raioz clean --dry-run --all --images
```

**Advertencias:**

- `--volumes` elimina datos persistentes. Úsalo con cuidado.
- `--all` afecta a todos los proyectos. Verifica antes de ejecutar.
- Usa `--dry-run` primero para ver qué se eliminará.

---

## Comandos Avanzados

### `raioz workspace`

Gestiona workspaces para organizar múltiples proyectos.

#### `raioz workspace use <workspace-name>`

Establece el workspace activo.

**Ejemplos:**

```bash
# Cambiar a workspace específico
raioz workspace use empresa-x

# Crear y usar nuevo workspace
raioz workspace use nuevo-proyecto
```

**Qué hace:**

- Crea el workspace si no existe
- Establece el workspace como activo en `~/.raioz/active-workspace`
- Los comandos futuros usarán este workspace por defecto

#### `raioz workspace list`

Lista todos los workspaces disponibles y muestra cuál está activo.

**Ejemplos:**

```bash
raioz workspace list
```

**Ejemplo de salida:**

```
Available Workspaces:
  - empresa-x (active)
  - billing-platform
  - auth-service
```

---

### `raioz override`

Sobrescribe un servicio para usar una ruta local en lugar del repositorio Git o imagen definida en `.raioz.json`.

**Sintaxis:**

```bash
raioz override <service> --path <local-path>
```

**Flags:**

- `--path`: Ruta local al servicio (requerido)

**Ejemplos:**

```bash
# Sobrescribir servicio con ruta local
raioz override orders --path ~/dev/orders

# Sobrescribir con ruta absoluta
raioz override api --path /opt/custom/api
```

**Qué hace:**

- Registra el override en `~/.raioz/overrides.json`
- El override tiene precedencia sobre `.raioz.json`
- No modifica `.raioz.json`
- El override se revierte automáticamente si la ruta no existe

**Casos de uso:**

- Desarrollo local: trabajar con código local en lugar de clonado
- Testing: probar cambios sin modificar `.raioz.json`
- Hot-reload: usar directorio con hot-reload habilitado

**Ver overrides:**

```bash
raioz override list
```

**Eliminar override:**

```bash
raioz override remove <service>
# o
raioz override rm <service>
```

---

### `raioz ignore`

Gestiona servicios que deben ser ignorados durante la resolución de dependencias.

#### `raioz ignore add <service>`

Agrega un servicio a la lista de ignorados.

**Ejemplos:**

```bash
# Ignorar un servicio
raioz ignore add legacy-service

# Ignorar múltiples servicios
raioz ignore add service1 service2 service3
```

**Qué hace:**

- El servicio no se clonará, construirá o iniciará durante `raioz up`
- Se guarda en `~/.raioz/ignore.json`
- Útil para servicios que no se necesitan en desarrollo local

#### `raioz ignore remove <service>`

Elimina un servicio de la lista de ignorados.

**Ejemplos:**

```bash
raioz ignore remove legacy-service
# o
raioz ignore rm legacy-service
```

#### `raioz ignore list`

Lista todos los servicios ignorados.

**Ejemplos:**

```bash
raioz ignore list
```

---

### `raioz link`

Gestiona symlinks desde el workspace de Raioz a rutas externas para edición.

#### `raioz link add <service> <external-path>`

Crea un symlink desde el workspace a una ruta externa.

**Ejemplos:**

```bash
# Crear symlink para editar servicio externamente
raioz link add api ~/dev/api

# Crear symlink con ruta absoluta
raioz link add frontend /opt/my-editor/frontend
```

**Qué hace:**

- Crea un symlink del servicio en el workspace a la ruta externa
- Permite editar el código en el editor externo
- Los cambios se reflejan en el contenedor Docker

**Casos de uso:**

- Usar IDE externo para editar código
- Compartir código entre proyectos
- Desarrollo con herramientas externas

#### `raioz link remove <service>`

Elimina un symlink de un servicio.

**Ejemplos:**

```bash
raioz link remove api
# o
raioz link rm api
# o
raioz link unlink api
```

#### `raioz link list`

Lista todos los servicios con symlinks.

**Ejemplos:**

```bash
raioz link list
```

---

### `raioz check`

Verifica la alineación entre la configuración y el estado guardado.

**Sintaxis:**

```bash
raioz check [flags]
```

**Flags:**

- `--config, -c`: Ruta al archivo de configuración
- `--project, -p`: Nombre del proyecto

**Ejemplos:**

```bash
# Verificar alineación
raioz check
```

**Qué detecta:**

- Cambios en configuración (servicios agregados/eliminados)
- Drift de ramas Git (cambios manuales en repositorios)
- Cambios de versiones (commits diferentes)
- Desalineaciones (servicios que deberían estar corriendo pero no)

**Ejemplo de salida:**

```
Checking alignment...

✓ Configuration matches state
✓ All services are on correct branches
✓ All services are on correct commits
✓ All services are running
```

O si hay problemas:

```
Checking alignment...

⚠ Configuration changes detected:
  - Service 'new-service' added
  - Service 'old-service' removed

⚠ Branch drift detected:
  - Service 'api' is on branch 'feature/new' but should be on 'main'

⚠ Version drift detected:
  - Service 'frontend' is on commit 'abc123' but should be on 'def456'
```

---

### `raioz compare`

Compara la configuración local (`.raioz.json`) con una configuración de producción (Docker Compose).

**Sintaxis:**

```bash
raioz compare <docker-compose-file> [flags]
```

**Flags:**

- `--output, -o`: Archivo de salida para el reporte (default: stdout)

**Ejemplos:**

```bash
# Comparar con docker-compose.yml de producción
raioz compare docker-compose.prod.yml

# Guardar reporte en archivo
raioz compare docker-compose.prod.yml --output diff-report.txt
```

**Qué compara:**

- Imágenes Docker (nombre y tag)
- Puertos mapeados
- Volúmenes
- Variables de entorno
- Dependencias entre servicios

**Casos de uso:**

- Verificar que desarrollo local coincide con producción
- Identificar diferencias antes de deploy
- Documentar diferencias entre entornos

---

### `raioz migrate`

Convierte un archivo Docker Compose de producción a formato `.raioz.json`.

**Sintaxis:**

```bash
raioz migrate <docker-compose-file> [flags]
```

**Flags:**

- `--output, -o`: Archivo de salida (default: `.raioz.json`)
- `--project-name`: Nombre del proyecto (default: se infiere del archivo)

**Ejemplos:**

```bash
# Migrar docker-compose.yml a .raioz.json
raioz migrate docker-compose.yml

# Migrar con nombre de proyecto específico
raioz migrate docker-compose.prod.yml --project-name my-project

# Guardar en archivo diferente
raioz migrate docker-compose.yml --output custom-raioz.json
```

**Qué hace:**

- Analiza el Docker Compose
- Convierte servicios a formato `.raioz.json`
- Intenta inferir información de Git (si hay build context)
- Genera configuración lista para usar

**Nota:** La migración es una aproximación. Revisa y ajusta el `.raioz.json` generado.

---

## Comandos de CI/CD

### `raioz ci`

Comando optimizado para pipelines de CI/CD con validaciones rápidas y output en JSON.

**Sintaxis:**

```bash
raioz ci [flags]
```

**Flags:**

- `--config, -c`: Ruta al archivo de configuración
- `--keep`: Mantener entorno efímero después de CI (para debugging)
- `--ephemeral`: Usar entorno efímero (auto-cleanup)
- `--job-id`: ID del job CI para naming de entorno efímero
- `--skip-build`: Saltar build y start de servicios (solo validación)
- `--skip-pull`: Saltar pull de imágenes Docker
- `--only-validate`: Solo ejecutar validaciones, saltar setup completo
- `--force-reclone`: Forzar re-clonado de repositorios

**Ejemplos:**

```bash
# Ejecutar CI completo
raioz ci

# Solo validaciones (no levanta servicios)
raioz ci --only-validate

# Validaciones y compose, sin build
raioz ci --skip-build

# Saltar pull de imágenes
raioz ci --skip-pull

# Usar entorno efímero (auto-cleanup)
raioz ci --ephemeral

# Entorno efímero con job ID
raioz ci --ephemeral --job-id $CI_JOB_ID

# Mantener entorno para debugging
raioz ci --ephemeral --keep
```

**Características:**

- Output siempre en formato JSON (parseable)
- Validaciones rápidas (solo checks críticos)
- Entornos efímeros con limpieza automática
- Exit code 0 si éxito, 1 si falla
- Validaciones paso a paso con estado (passed/failed/skipped)

**Ejemplo de output JSON:**

```json
{
  "success": true,
  "startTime": "2024-01-15T10:30:00Z",
  "endTime": "2024-01-15T10:35:00Z",
  "duration": 300.5,
  "validations": [
    {
      "check": "preflight",
      "status": "passed"
    },
    {
      "check": "load_config",
      "status": "passed"
    },
    {
      "check": "validate",
      "status": "passed"
    }
  ],
  "warnings": [],
  "errors": []
}
```

---

## Comandos de Utilidad

### `raioz version`

Muestra información de versión.

**Sintaxis:**

```bash
raioz version
```

**Ejemplos:**

```bash
raioz version
```

**Información mostrada:**

- Versión del binario
- Commit SHA (si está disponible)
- Fecha de build
- Información del sistema (OS, arch)

**Ejemplo de salida:**

```
raioz version 1.0.0
Commit: abc123def456
Build Date: 2024-01-15T10:30:00Z
OS: linux
Arch: amd64
```

---

## Flags Globales

Todos los comandos soportan estos flags globales:

- `--log-level`: Nivel de log (debug, info, warn, error)
- `--log-json`: Output de logs en formato JSON

**Ejemplos:**

```bash
# Ver logs de debug
raioz up --log-level debug

# Logs en JSON para CI
raioz up --log-json
```

---

## Combinación de Comandos

Ejemplos de uso común combinando comandos:

```bash
# Workflow completo de desarrollo
raioz up                    # Levantar entorno
raioz status               # Verificar estado
raioz logs --follow api    # Monitorear logs
raioz down                 # Detener cuando termine

# Debugging
raioz check                # Verificar alineación
raioz logs --tail 200 api  # Ver últimos logs
raioz status --json       # Estado en JSON para análisis

# Limpieza periódica
raioz clean --dry-run      # Ver qué se limpiaría
raioz clean --images       # Limpiar imágenes
raioz clean --all          # Limpiar todo
```
