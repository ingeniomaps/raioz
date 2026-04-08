# Guía de Comandos

Referencia completa de todos los comandos de Raioz.

## Resumen

| Comando | Descripción |
|---------|-------------|
| `raioz up` | Levantar servicios e infraestructura |
| `raioz down` | Detener servicios |
| `raioz status` | Mostrar estado del proyecto |
| `raioz restart` | Reiniciar servicios |
| `raioz exec` | Ejecutar comando en un contenedor |
| `raioz logs` | Ver logs de servicios |
| `raioz ports` | Listar puertos en uso |
| `raioz health` | Verificar salud del proyecto local |
| `raioz check` | Validar configuración y detectar drift |
| `raioz init` | Crear `.raioz.json` interactivamente |
| `raioz list` | Listar proyectos activos |
| `raioz clean` | Limpiar recursos Docker no utilizados |
| `raioz volumes` | Gestionar volúmenes del proyecto |
| `raioz workspace` | Gestionar workspaces |
| `raioz override` | Sobreescribir servicio con ruta local |
| `raioz ignore` | Ignorar servicios durante `up` |
| `raioz link` | Crear symlinks a rutas externas |
| `raioz migrate` | Convertir docker-compose.yml a .raioz.json |
| `raioz compare` | Comparar config local vs producción |
| `raioz ci` | Comando optimizado para CI/CD |
| `raioz version` | Mostrar versión |
| `raioz lang` | Gestionar idioma |

---

## Ciclo de vida

### `raioz up`

Levanta todos los servicios e infraestructura del proyecto.

```bash
raioz up [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--profile` | `-p` | | Perfil a usar (frontend/backend) |
| `--force-reclone` | | `false` | Re-clonar todos los repos Git |
| `--dry-run` | | `false` | Mostrar qué se haría sin ejecutar |

**Flujo de ejecución:**

1. Carga configuración y aplica overrides
2. Filtra por profile, feature flags, servicios ignorados
3. Valida schema, reglas de negocio, dependencias
4. Resuelve conflictos de workspace/proyecto (interactivo)
5. Clona/actualiza repos Git
6. Genera archivos .env
7. Prepara Docker (imágenes, red, volúmenes)
8. Inicia infra primero, espera healthy
9. Inicia servicios
10. Guarda estado

**Ejemplos:**

```bash
raioz up                          # Levantar con defaults
raioz up -f gateway/.raioz.json   # Config en otra ruta
raioz up --profile frontend       # Solo servicios frontend
raioz up --dry-run                # Ver resumen sin ejecutar
raioz up --force-reclone          # Re-clonar repos editables
```

**Comportamiento multi-proyecto:**

- Si otro proyecto corre en el mismo workspace, ofrece:
  - [1] Merge: mantener ambos proyectos juntos
  - [2] Replace: reemplazar con el nuevo proyecto
  - [3] Keep: mantener el proyecto actual
  - [4-7] Recordar decisión para este workspace
  - [8] Cancelar

---

### `raioz down`

Detiene servicios del proyecto.

```bash
raioz down [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project` | `-p` | | Nombre del proyecto |
| `--all` | | `false` | Detener todo (servicios + infra) |
| `--prune-shared` | | `false` | Eliminar infra si ningún otro proyecto la usa |

**Comportamiento:**

- **Sin flags**: Detiene solo los servicios del proyecto. La infraestructura sigue corriendo (puede ser compartida).
- **`--all`**: Detiene servicios + infraestructura + limpia imágenes/volúmenes no utilizados.
- **`--prune-shared`**: Detiene servicios. Si la infra no es usada por otro proyecto, la elimina; si es compartida, la mantiene.

**Ejemplos:**

```bash
raioz down                        # Solo servicios (infra sigue)
raioz down --all                  # Todo, incluyendo infra
raioz down --prune-shared         # Servicios + infra si no compartida
raioz down --project my-project   # Por nombre de proyecto
```

**Protecciones:**

- Infraestructura compartida entre proyectos NO se elimina (a menos que sea el último proyecto)
- Volúmenes con datos persisten entre ciclos down/up
- Idempotente: no falla si ya está detenido

---

### `raioz status`

Muestra estado detallado del proyecto.

```bash
raioz status [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project` | `-p` | | Nombre del proyecto |
| `--json` | | `false` | Salida en formato JSON |

**Información mostrada:**

- Estado de cada servicio (running/stopped)
- Health (healthy/unhealthy/starting)
- Uptime, CPU, memoria
- Versión/imagen
- Servicios deshabilitados

**Ejemplos:**

```bash
raioz status                      # Tabla de estado
raioz status --json               # JSON para scripts
raioz status -p my-project        # Por nombre
```

---

### `raioz logs`

Muestra logs de servicios.

```bash
raioz logs [service...] [flags]
```

**Flags:**

| Flag | Default | Descripción |
|------|---------|-------------|
| `--file, -f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project, -p` | | Nombre del proyecto |
| `--follow` | `false` | Seguir logs en tiempo real |
| `--tail` | `0` | Últimas N líneas (0 = todas, default 100 sin --follow) |
| `--all` | `false` | Logs de todos los servicios |

**Ejemplos:**

```bash
raioz logs api                    # Logs de un servicio
raioz logs api web                # Logs de múltiples
raioz logs --all --tail 50        # Últimas 50 líneas de todos
raioz logs --follow api           # Streaming en tiempo real
```

---

### `raioz ports`

Lista todos los puertos en uso por proyectos activos.

```bash
raioz ports [flags]
```

**Flags:**

| Flag | Corto | Descripción |
|------|-------|-------------|
| `--project` | `-p` | Filtrar por proyecto |

**Ejemplo de salida:**

```
━━━ ACTIVE PORTS ━━━
PORT         PROJECT       SERVICE
────         ───────       ───────
3000:3000    my-project    api
5432:5432    my-project    postgres
8080:80      other         frontend
```

---

### `raioz health`

Verifica la salud del proyecto local.

```bash
raioz health [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |

Ejecuta `project.commands.health` (si está definido) y reporta si el proyecto está saludable.

### `raioz restart`

Reinicia uno o más servicios del proyecto.

```bash
raioz restart [service...] [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project` | `-p` | | Nombre del proyecto |
| `--all` | | `false` | Reiniciar todos los servicios |
| `--include-infra` | | `false` | Incluir infraestructura en el reinicio |
| `--force-recreate` | | `false` | Recrear contenedores en lugar de reiniciar |

**Ejemplos:**

```bash
raioz restart api               # Reiniciar un servicio específico
raioz restart api worker         # Reiniciar varios servicios
raioz restart --all              # Reiniciar todos los servicios
raioz restart --all --include-infra  # Reiniciar todo, incluyendo infra
raioz restart api --force-recreate   # Recrear contenedor desde cero
```

**Notas:**
- Los servicios host (`source.command`) no se pueden reiniciar con este comando. Usa `raioz down && raioz up`.
- Busca servicios tanto en el compose generado como en el `ProjectComposePath`.

### `raioz exec`

Ejecuta un comando dentro de un contenedor de servicio en ejecución.

```bash
raioz exec <service> [command...] [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project` | `-p` | | Nombre del proyecto |
| `--interactive` | `-i` | `true` | Mantener stdin abierto y asignar TTY |

**Ejemplos:**

```bash
raioz exec api sh                        # Abrir shell en servicio
raioz exec postgres psql -U postgres     # Conectar a PostgreSQL
raioz exec redis redis-cli               # Abrir CLI de Redis
raioz exec mongo mongosh --eval "db.stats()"  # Ejecutar comando en MongoDB
raioz exec -i=false api ls /app          # Ejecutar sin TTY
raioz exec --project myproject api sh    # Desde otro directorio
```

**Notas:**
- Si no se especifica comando, abre un shell (`sh`) por defecto.
- Los flags del comando destino (como `-U` en psql) se pasan directamente al contenedor.
- Busca servicios en el compose generado y en el `ProjectComposePath`.
- Los servicios host (`source.command`) no soportan exec — se muestra un error claro con sugerencia.

---

## Configuración y validación

### `raioz init`

Wizard interactivo para crear `.raioz.json`.

```bash
raioz init [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--output` | `-o` | `.raioz.json` | Ruta de salida |

**El wizard pregunta:**

1. Nombre del proyecto
2. Nombre de red Docker
3. ¿Agregar servicios? (loop, git o image por servicio)
4. ¿Agregar infraestructura? (presets: PostgreSQL, Redis, MySQL, MongoDB, o custom)

**Ejemplos:**

```bash
raioz init                        # Crear en directorio actual
raioz init -o gateway/.raioz.json # Crear en otra ruta
```

---

### `raioz check`

Valida la configuración y detecta drift con el estado guardado.

```bash
raioz check [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project` | `-p` | | Nombre del proyecto |

**Validaciones:**

1. Schema JSON (formato correcto)
2. Reglas de negocio (proyecto, servicios, infra, dependencias)
3. Alignment con estado guardado (si existe)

**Exit codes:**

- `0`: Todo válido / solo info
- `1`: Errores de validación o drift crítico

---

### `raioz list`

Lista todos los proyectos activos.

```bash
raioz list [flags]
```

**Flags:**

| Flag | Default | Descripción |
|------|---------|-------------|
| `--json` | `false` | Salida JSON |
| `--filter` | | Filtrar por nombre (parcial, case-insensitive) |
| `--status` | | Filtrar por estado (running/stopped) |

**Ejemplo de salida:**

```
━━━ ACTIVE PROJECTS ━━━

▸ billing-platform
  Workspace: /home/user/.raioz/workspaces/billing-platform
  Last Execution: 2 hours ago
  Active Services: 5
  Running: 5/5
  Services: ✓ api, ✓ web, ✓ worker, ✓ payments, ✓ notifications
```

---

## Gestión de workspace

### `raioz workspace`

Gestiona workspaces para organizar múltiples proyectos.

```bash
raioz workspace              # Muestra workspace activo
raioz workspace use <name>   # Establecer workspace activo
raioz workspace list          # Listar workspaces
raioz workspace delete <name> # Eliminar workspace
```

**Comportamiento:**

- `use`: Crea el workspace si no existe y lo marca como activo
- `list`: Muestra todos con `*` en el activo
- `delete`: Elimina directorio del workspace. Si era el activo, lo desactiva. NO elimina repos clonados.
- Sin subcomando: muestra el workspace actual

---

## Modificadores de config

### `raioz override`

Sobreescribe un servicio para usar una ruta local en vez del repo Git.

```bash
raioz override <service> --path <dir>  # Registrar override
raioz override list                     # Ver overrides
raioz override remove <service>         # Eliminar override
raioz override rm <service>             # Alias de remove
```

**Comportamiento:**

- NO modifica `.raioz.json` — se guarda en `~/.raioz/overrides.json`
- Override tiene precedencia: `override > .raioz.json > default`
- Se aplica automáticamente durante `raioz up`
- Se revierte si la ruta deja de existir
- Solo aplica a servicios Git (image services se ignoran)

---

### `raioz ignore`

Ignora servicios durante `raioz up`.

```bash
raioz ignore add <svc> [svc2 svc3...]  # Agregar (múltiples)
raioz ignore list                       # Ver ignorados
raioz ignore remove <svc> [svc2...]     # Eliminar (múltiples)
raioz ignore rm <svc>                   # Alias de remove
```

**Comportamiento:**

- El servicio no se clona, construye ni inicia durante `up`
- Se guarda en `~/.raioz/ignore.json`
- Advierte si otros servicios dependen del ignorado
- Operaciones idempotentes (duplicar add = no-op)

---

### `raioz link`

Crea symlinks del workspace a rutas externas.

```bash
raioz link add <service> <path>  # Crear symlink
raioz link list                   # Ver servicios enlazados
raioz link remove <service>       # Eliminar symlink
raioz link rm <service>           # Alias
raioz link unlink <service>       # Alias
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |

**Comportamiento:**

- Permite editar código desde una ubicación externa
- Los cambios se reflejan en el contenedor Docker (mismo filesystem)
- Remove elimina solo el symlink, NO el directorio externo

---

## Limpieza

### `raioz clean`

Limpia servicios detenidos y recursos Docker no utilizados.

```bash
raioz clean [flags]
```

**Flags:**

| Flag | Default | Descripción |
|------|---------|-------------|
| `--file, -f` | `.raioz.json` | Ruta al archivo de configuración |
| `--project, -p` | | Nombre del proyecto |
| `--all` | `false` | Limpiar todos los proyectos |
| `--images` | `false` | Eliminar imágenes Docker no utilizadas |
| `--volumes` | `false` | Eliminar volúmenes (requiere confirmación) |
| `--networks` | `false` | Eliminar redes Docker no utilizadas |
| `--dry-run` | `false` | Mostrar qué se haría sin ejecutar |
| `--force` | `false` | Omitir confirmaciones |

**Ejemplos:**

```bash
raioz clean --dry-run --all --images --volumes  # Ver qué se limpiaría
raioz clean --all --images --force              # Limpiar todo sin preguntar
```

### `raioz volumes`

Gestiona volúmenes Docker asociados con un proyecto.

```bash
raioz volumes <subcommand> [flags]
```

**Subcomandos:**

#### `raioz volumes list`

Lista todos los volúmenes del proyecto, mostrando su origen (servicio o infra) y si están compartidos con otros proyectos.

```bash
raioz volumes list
raioz volumes list --project myproject
```

#### `raioz volumes remove`

Elimina volúmenes del proyecto. Puede eliminar volúmenes específicos por nombre o todos con `--all`.

```bash
raioz volumes remove [volume...] [flags]
```

| Flag | Default | Descripción |
|------|---------|-------------|
| `--all` | `false` | Eliminar todos los volúmenes del proyecto |
| `--force` | `false` | Omitir confirmación |

**Ejemplos:**

```bash
raioz volumes list                                   # Ver volúmenes del proyecto
raioz volumes remove myproject_postgres-data          # Eliminar uno específico
raioz volumes remove --all                            # Eliminar todos (con confirmación)
raioz volumes remove --all --force                    # Eliminar todos sin confirmar
raioz volumes rm myproject_redis-data                 # Alias: rm = remove
```

**Notas:**
- Los volúmenes compartidos con otros proyectos se excluyen automáticamente.
- Detén el proyecto (`raioz down`) antes de eliminar volúmenes en uso.
- `volumes list` muestra lo declarado en la config; `volumes remove` opera sobre lo que existe en Docker.

---

## Producción y CI

### `raioz migrate`

Convierte un docker-compose.yml de producción a `.raioz.json`.

```bash
raioz migrate [flags]
```

**Flags:**

| Flag | Corto | Default | Requerido | Descripción |
|------|-------|---------|-----------|-------------|
| `--compose` | `-c` | | Sí | Ruta al docker-compose.yml |
| `--project` | `-p` | | Sí | Nombre del proyecto |
| `--output` | `-o` | `.raioz.json` | No | Ruta de salida |
| `--network` | | `{project}-network` | No | Nombre de red |

**Ejemplo:**

```bash
raioz migrate -c docker-compose.prod.yml -p my-project
```

Separa automáticamente servicios vs infraestructura (postgres, redis, mongo, etc.).

---

### `raioz compare`

Compara configuración local con producción.

```bash
raioz compare [flags]
```

**Flags:**

| Flag | Corto | Default | Requerido | Descripción |
|------|-------|---------|-----------|-------------|
| `--file` | `-f` | `.raioz.json` | No | Config local |
| `--production` | `-p` | | Sí | Docker-compose de producción |
| `--json` | | `false` | No | Salida JSON |

**Detecta diferencias en:**

- Imágenes (nombre y tag)
- Puertos mapeados
- Volúmenes
- Dependencias entre servicios

---

### `raioz ci`

Comando optimizado para pipelines CI/CD.

```bash
raioz ci [flags]
```

**Flags:**

| Flag | Corto | Default | Descripción |
|------|-------|---------|-------------|
| `--file` | `-f` | `.raioz.json` | Ruta al archivo de configuración |
| `--only-validate` | | `false` | Solo validar, no ejecutar |
| `--skip-build` | | `false` | Omitir build y start |
| `--skip-pull` | | `false` | Omitir pull de imágenes |
| `--ephemeral` | | `false` | Ambiente efímero (auto-cleanup) |
| `--keep` | | `false` | Mantener ambiente efímero (debug) |
| `--job-id` | | | ID del job CI |
| `--force-reclone` | | `false` | Re-clonar repos |

**Salida JSON** con validaciones, errores, warnings, y tiempos.

**Ejemplo:**

```bash
raioz ci --only-validate -f .raioz.json
```

---

## Utilidades

### `raioz version`

```bash
raioz version
```

Muestra: versión, schema version, commit, fecha de build.

### `raioz lang`

```bash
raioz lang              # Mostrar idioma actual
raioz lang set es       # Cambiar a español
raioz lang set en       # Cambiar a inglés
raioz lang list         # Listar idiomas disponibles
```

---

## Flags globales

Disponibles en todos los comandos:

| Flag | Descripción |
|------|-------------|
| `--lang` | Idioma de la interfaz (en, es) |
| `--log-level` | Nivel de log (debug, info, warn, error) |
| `--log-json` | Logs en formato JSON |
| `-h, --help` | Ayuda del comando |
