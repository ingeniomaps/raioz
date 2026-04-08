# Feature: `raioz up --watch` (Hot-Reload)

## Resumen

Modo watch que detecta cambios en archivos y reinicia automáticamente solo los servicios afectados. Elimina el ciclo manual de "editar → guardar → raioz restart api → esperar → probar".

```bash
raioz up --watch           # Levanta todo y queda observando
raioz up --watch --only api  # Solo api + deps, observando cambios
```

## Valor para el desarrollador

**Sin watch:**
```
1. Editar código del API          (2 seg)
2. Cambiar a terminal             (1 seg)
3. raioz restart api              (3 seg)
4. Esperar healthy                (5 seg)
5. Probar cambio                  (2 seg)
Total: ~13 seg por iteración × 50 veces/día = ~11 min/día perdidos
```

**Con watch:**
```
1. Editar código del API          (2 seg)
2. [automático] servicio reiniciado (3 seg)
3. Probar cambio                  (2 seg)
Total: ~7 seg por iteración → ahorro de ~5 min/día por desarrollador
```

## Diseño técnico

### Arquitectura

```
┌──────────────┐     ┌────────────┐     ┌──────────────┐
│  fsnotify    │────→│  Debouncer │────→│  Dispatcher  │
│  (watchers)  │     │  (300ms)   │     │              │
└──────────────┘     └────────────┘     └──────┬───────┘
                                               │
                          ┌────────────────────┼────────────────────┐
                          │                    │                    │
                     ┌────▼─────┐     ┌───────▼──────┐    ┌──────▼───────┐
                     │ Reiniciar│     │ Regenerar    │    │ Recargar    │
                     │ servicio │     │ compose +    │    │ env vars    │
                     │          │     │ reiniciar    │    │             │
                     └──────────┘     └──────────────┘    └──────────────┘
```

### Componentes

#### 1. File Watcher (`internal/watch/watcher.go`)

Usa `github.com/fsnotify/fsnotify` para monitorear:

| Qué monitorea | Directorio | Evento |
|----------------|-----------|--------|
| Código de servicios git | `~/.raioz/workspaces/{project}/services/{service}/` | Write/Create |
| Código de servicios local | Ruta del `source.path` | Write/Create |
| Código de servicios override | Ruta del override activo | Write/Create |
| Config del proyecto | `.raioz.json` | Write |
| Templates de env | `env/templates/` | Write/Create |

**No monitorea:**
- `node_modules/`, `.git/`, `__pycache__/`, `vendor/`, `dist/`, `build/`
- Archivos ocultos (`.DS_Store`, `.env`)
- Archivos binarios (imágenes, compilados)

#### 2. Debouncer (`internal/watch/debounce.go`)

Agrupa cambios rápidos en una sola acción:

```go
type Debouncer struct {
    delay    time.Duration  // 300ms default
    timers   map[string]*time.Timer  // por servicio
    callback func(serviceName string)
}
```

**Por qué:** Un save en el editor genera múltiples eventos (write tmp, rename, write final). Sin debounce, reiniciaría 3 veces por cada guardado.

#### 3. Service Resolver (`internal/watch/resolver.go`)

Mapea "qué archivo cambió" → "qué servicio reiniciar":

```go
// Dado un path de archivo cambiado, retorna el nombre del servicio afectado
func (r *Resolver) ResolveService(changedPath string) (string, error)
```

Lógica:
1. Si el path está dentro de `services/{name}/` → reiniciar ese servicio
2. Si el path es un override → reiniciar el servicio overrideado
3. Si el path es `.raioz.json` → regenerar compose y reiniciar todo
4. Si el path es un template de env → regenerar env y reiniciar servicios que lo usan

#### 4. Dispatcher (`internal/watch/dispatcher.go`)

Ejecuta la acción apropiada:

```go
type Action int
const (
    ActionRestartService Action = iota  // docker compose restart {service}
    ActionRecreateService               // docker compose up -d --force-recreate {service}
    ActionRegenerateCompose             // regenerar compose + up
    ActionReloadEnv                     // regenerar .env + restart
)
```

### Flujo de ejecución

```
raioz up --watch
│
├── 1. Ejecutar raioz up normal (levantar todo)
│
├── 2. Construir mapa de servicios → directorios
│      - Leer config, overrides, workspace paths
│      - Registrar watchers en fsnotify
│
├── 3. Mostrar resumen de qué se monitorea
│      ┌─────────────────────────────────────────┐
│      │ 👁 Watch mode active                     │
│      │                                          │
│      │ Monitoring:                               │
│      │   api      → ~/.raioz/.../services/api/  │
│      │   frontend → ~/dev/frontend/ (override)   │
│      │   config   → ./.raioz.json               │
│      │                                          │
│      │ Press Ctrl+C to stop                     │
│      └─────────────────────────────────────────┘
│
├── 4. Loop de eventos (bloqueante)
│      ├── Evento: archivo cambiado
│      │   ├── Resolver → servicio afectado
│      │   ├── Debounce → esperar 300ms
│      │   └── Dispatcher → reiniciar servicio
│      │       └── Output: "↻ Restarting api... ✔ (1.2s)"
│      │
│      ├── Evento: .raioz.json cambiado
│      │   ├── Recargar config
│      │   ├── Regenerar compose
│      │   └── docker compose up -d (aplica diff)
│      │       └── Output: "↻ Config changed, regenerating... ✔"
│      │
│      └── Evento: Ctrl+C (SIGINT)
│          └── Salir limpiamente (NO ejecutar raioz down)
│
└── 5. Salida limpia
       - Cerrar watchers
       - Servicios siguen corriendo (el usuario hace raioz down después)
```

### Flags del CLI

```go
upCmd.Flags().BoolVar(&watch, "watch", false, "Watch for file changes and restart affected services")
upCmd.Flags().DurationVar(&watchDebounce, "watch-debounce", 300*time.Millisecond, "Debounce delay for watch events")
```

### Patrones de exclusión por defecto

```go
var defaultExcludes = []string{
    "node_modules",
    ".git",
    "__pycache__",
    ".pytest_cache",
    "vendor",
    "dist",
    "build",
    ".next",
    ".nuxt",
    "target",       // Rust/Java
    "bin",
    "obj",          // .NET
    ".DS_Store",
    "*.swp",
    "*.swo",
    "*~",
    "*.tmp",
}
```

### Edge cases a manejar

| Caso | Comportamiento |
|------|---------------|
| Múltiples archivos cambian a la vez | Debounce agrupa en un solo restart |
| Servicio dependiente cambia | Solo reinicia el servicio cambiado, no sus dependientes |
| `.raioz.json` cambia | Regenera compose completo, aplica diff |
| Override agregado/removido | Recargar watchers, reiniciar servicio |
| Servicio host (source.command) | No aplica restart via docker, skip con warning |
| Error en restart | Mostrar error, continuar observando |
| Directorio eliminado | Cerrar watcher, warning |
| Nuevo archivo creado | Detectado por fsnotify |
| Disco lleno | Warning, continuar |

### Output en modo watch

```
✔ Project 'billing' started successfully

👁 Watch mode — monitoring 4 services
  api      → /home/dev/.raioz/workspaces/billing/services/api/
  frontend → /home/dev/frontend/ (override)
  worker   → /home/dev/.raioz/workspaces/billing/services/worker/
  config   → .raioz.json

Press Ctrl+C to stop watching

[14:32:05] ↻ api — file changed: src/handler.go
[14:32:06] ✔ api restarted (1.1s)
[14:35:12] ↻ frontend — file changed: src/App.tsx
[14:35:13] ✔ frontend restarted (0.8s)
[14:40:01] ↻ .raioz.json changed — regenerating compose...
[14:40:03] ✔ compose regenerated, 1 service updated (2.1s)
^C
👋 Watch stopped. Services are still running. Use 'raioz down' to stop.
```

### Dependencia nueva

```bash
go get github.com/fsnotify/fsnotify@latest
```

### Archivos a crear

```
internal/watch/
├── watcher.go          # Core file watcher con fsnotify
├── debounce.go         # Debouncer por servicio
├── resolver.go         # Mapeo path → servicio
├── dispatcher.go       # Ejecuta restart/recreate/regenerate
├── excludes.go         # Patrones de exclusión
├── watcher_test.go     # Tests del watcher
├── debounce_test.go    # Tests del debouncer
└── resolver_test.go    # Tests del resolver
```

### Cambios en archivos existentes

| Archivo | Cambio |
|---------|--------|
| `internal/cli/up.go` | Flag `--watch`, `--watch-debounce` |
| `internal/app/up.go` | `Watch` y `WatchDebounce` en `UpOptions` |
| `internal/app/upcase/usecase.go` | Después del up normal, entrar en watch loop si `--watch` |
| `internal/i18n/locales/en.json` | ~10 keys para mensajes de watch |
| `internal/i18n/locales/es.json` | Traducciones |
| `go.mod` | Dependencia fsnotify |

### Estimación de complejidad

| Componente | Complejidad | Líneas estimadas |
|-----------|-------------|-----------------|
| Watcher core | Media | ~120 |
| Debouncer | Baja | ~60 |
| Resolver | Media | ~80 |
| Dispatcher | Media | ~100 |
| Excludes | Baja | ~40 |
| CLI + options | Baja | ~20 |
| i18n | Baja | ~20 |
| Tests | Media | ~200 |
| **Total** | | **~640 líneas** |

### Riesgos

1. **Performance en repos grandes:** fsnotify tiene límites de watchers por OS. Linux default: 8192 (`/proc/sys/fs/inotify/max_user_watches`). Un repo con 10k archivos puede excederlo.
   - **Mitigación:** Excluir `node_modules/`, `.git/`, etc. Monitorear solo el directorio raíz del servicio con recursión limitada.

2. **Docker restart lento:** Si un servicio tarda 10s en reiniciar, cambios durante ese tiempo se acumulan.
   - **Mitigación:** Cola de restart con deduplicación. Si un restart está en progreso para el mismo servicio, ignorar el nuevo evento.

3. **Windows compatibility:** fsnotify funciona en Windows pero con diferencias.
   - **Mitigación:** Testar en Linux/macOS primero. Windows es tier-2.

### Criterios de aceptación

- [ ] `raioz up --watch` levanta servicios y queda observando
- [ ] Cambio en archivo de servicio → reinicia solo ese servicio
- [ ] Cambio en `.raioz.json` → regenera compose y aplica diff
- [ ] Cambio en template de env → regenera env y reinicia
- [ ] Ctrl+C sale limpiamente sin detener servicios
- [ ] Funciona con `--only`: solo monitorea servicios seleccionados
- [ ] Funciona con overrides: monitorea la ruta del override
- [ ] Debounce: múltiples saves rápidos = un solo restart
- [ ] Excluye `node_modules/`, `.git/`, etc.
- [ ] Output claro con timestamps
- [ ] Tests unitarios para watcher, debouncer, resolver
