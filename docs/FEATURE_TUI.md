# Feature: `raioz dashboard` / `raioz tui` (Terminal UI)

## Resumen

Dashboard interactivo en terminal que muestra el estado de todos los servicios, sus logs en tiempo real, consumo de recursos y acciones rapidas, todo en una sola vista. Elimina la necesidad de ejecutar multiples comandos (`raioz status`, `raioz logs`, `docker stats`) en terminales separadas.

```bash
raioz dashboard          # Abre el dashboard interactivo
raioz tui                # Alias del mismo comando
raioz dashboard --only api,frontend  # Solo muestra servicios seleccionados
```

## Valor para el desarrollador

**Sin dashboard:**
```
Terminal 1: raioz status             (ver estado)
Terminal 2: raioz logs api -f        (seguir logs del api)
Terminal 3: docker stats             (ver CPU/memoria)
Terminal 4: raioz logs worker -f     (seguir logs del worker)
Total: 4 terminales abiertas, cambiar constantemente entre ellas
```

**Con dashboard:**
```
Terminal 1: raioz dashboard          (todo en una vista)
  - Estado de todos los servicios con health checks
  - Logs del servicio seleccionado en tiempo real
  - CPU y memoria de cada servicio
  - Atajos de teclado para restart, stop, exec
Total: 1 terminal, 0 cambios de contexto
```

## Mockup del UI

```
┌─ billing-platform ─────────────────────────────────────────┐
│ SERVICE       STATUS     CPU    MEM     PORTS               │
│ ► api         healthy    2.1%   128MB   :3000               │
│   frontend    healthy    1.8%   256MB   :3001               │
│   worker      running    0.3%   64MB    -                   │
│   postgres    healthy    0.5%   48MB    :5432               │
│   redis       healthy    0.1%   12MB    :6379               │
│                                                             │
│ ─── Logs (api) ────────────────────────────────────────── │
│ [14:32:05] GET /api/users 200 12ms                         │
│ [14:32:06] POST /api/orders 201 45ms                       │
│ [14:32:07] GET /api/health 200 1ms                         │
│ [14:32:08] GET /api/products 200 8ms                       │
│ [14:32:10] PUT /api/orders/42 200 23ms                     │
│                                                             │
│ [r]estart [l]ogs [e]xec [s]top [q]uit                     │
└─────────────────────────────────────────────────────────────┘
```

### Vista expandida de logs (al presionar `L`)

```
┌─ billing-platform ─ Logs: api ─────────────────────────────┐
│ [14:32:05] GET /api/users 200 12ms                         │
│ [14:32:06] POST /api/orders 201 45ms                       │
│ [14:32:07] GET /api/health 200 1ms                         │
│ [14:32:08] GET /api/products 200 8ms                       │
│ [14:32:10] PUT /api/orders/42 200 23ms                     │
│ [14:32:11] slog level=INFO msg="cache hit" key="prod:42"   │
│ [14:32:12] GET /api/orders?status=pending 200 15ms         │
│ [14:32:14] DELETE /api/orders/12 204 6ms                   │
│ [14:32:15] GET /api/health 200 1ms                         │
│                                                             │
│ [ESC] volver  [/] buscar  [f] filtrar  [c] limpiar         │
└─────────────────────────────────────────────────────────────┘
```

## Diseno tecnico

### Arquitectura

```
┌──────────────────┐     ┌───────────────┐     ┌──────────────┐
│  Docker Events   │────→│    Model      │────→│    View      │
│  (stream)        │     │  (estado)     │     │  (render)    │
└──────────────────┘     └───────┬───────┘     └──────────────┘
                                 │
┌──────────────────┐             │              ┌──────────────┐
│  Docker Stats    │────→        │         ←────│  Keyboard    │
│  (polling 2s)    │             │              │  (input)     │
└──────────────────┘     ┌───────▼───────┐     └──────────────┘
                         │    Update     │
┌──────────────────┐     │  (mensajes)  │
│  Docker Logs     │────→│              │
│  (stream)        │     └──────────────┘
└──────────────────┘
```

Se utiliza el patron **Model-View-Update** (MVU / Elm Architecture) que es nativo de Bubble Tea:

1. **Model**: Estado inmutable de la aplicacion (servicios, logs, seleccion, metricas).
2. **Update**: Funcion pura que recibe un mensaje y retorna el nuevo modelo.
3. **View**: Funcion pura que renderiza el modelo como string.

### Libreria principal

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
```

- **bubbletea**: Framework TUI con patron MVU, manejo de input, resize y rendering eficiente.
- **lipgloss**: Estilos CSS-like para terminal (bordes, colores, padding).
- **bubbles**: Componentes reutilizables (viewport para scroll de logs, table para servicios).

### Componentes

#### 1. Dashboard Model (`internal/tui/model.go`)

Estado central de la aplicacion:

```go
type Model struct {
    services    []ServiceRow       // Lista de servicios con estado
    selected    int                // Indice del servicio seleccionado
    logs        map[string][]string // Buffer de logs por servicio (ultimas 1000 lineas)
    view        ViewMode           // Normal | LogsExpanded | Exec
    width       int                // Ancho de terminal
    height      int                // Alto de terminal
    project     string             // Nombre del proyecto
    err         error              // Ultimo error
}

type ServiceRow struct {
    Name   string
    Status string  // healthy | running | stopped | error
    CPU    string  // "2.1%"
    Memory string  // "128MB"
    Ports  string  // ":3000"
}

type ViewMode int
const (
    ViewNormal ViewMode = iota
    ViewLogsExpanded
)
```

#### 2. Mensajes (`internal/tui/messages.go`)

Mensajes que alimentan el ciclo Update:

```go
type StatsMsg struct {
    Stats map[string]ContainerStats  // CPU, memoria por servicio
}

type LogMsg struct {
    Service string
    Line    string
}

type EventMsg struct {
    Service string
    Status  string  // "start", "stop", "die", "health_status"
}

type ActionResultMsg struct {
    Service string
    Action  string  // "restart", "stop"
    Err     error
}
```

#### 3. Subscripciones (`internal/tui/subscriptions.go`)

Goroutines que alimentan el modelo con datos en tiempo real:

```go
// Escucha docker events (container start/stop/die/health)
func listenDockerEvents(ctx context.Context) tea.Cmd

// Polling de docker stats cada 2 segundos
func pollDockerStats(ctx context.Context, interval time.Duration) tea.Cmd

// Stream de logs del servicio seleccionado
func streamLogs(ctx context.Context, service string) tea.Cmd
```

#### 4. Vista (`internal/tui/view.go`)

Renderiza el modelo como string usando lipgloss:

```go
func (m Model) View() string {
    header := renderHeader(m.project, m.width)
    table := renderServiceTable(m.services, m.selected, m.width)
    logs := renderLogPanel(m.logs[m.selectedService()], m.height)
    footer := renderShortcuts(m.view)

    return lipgloss.JoinVertical(lipgloss.Left, header, table, logs, footer)
}
```

#### 5. Acciones (`internal/tui/actions.go`)

Ejecuta acciones del usuario en background:

```go
func restartService(project, service string) tea.Cmd
func stopService(project, service string) tea.Cmd
func execInService(project, service, command string) tea.Cmd
```

### Flujo de ejecucion

```
raioz dashboard
│
├── 1. Leer estado actual del proyecto (.state.json)
│      - Obtener lista de servicios activos
│      - Obtener nombre del proyecto
│
├── 2. Inicializar modelo con estado inicial
│      - docker compose ps → llenar ServiceRow
│      - docker stats --no-stream → metricas iniciales
│
├── 3. Iniciar Bubble Tea program
│      ├── Subscripciones iniciales:
│      │   ├── Docker events stream
│      │   ├── Docker stats polling (cada 2s)
│      │   └── Logs stream del primer servicio
│      │
│      ├── Loop de eventos:
│      │   ├── Tecla ↑/↓/j/k → cambiar servicio seleccionado
│      │   │   └── Reanexar stream de logs al nuevo servicio
│      │   ├── Tecla 'r' → restart servicio seleccionado
│      │   ├── Tecla 's' → stop servicio seleccionado
│      │   ├── Tecla 'l' → expandir vista de logs
│      │   ├── Tecla 'e' → abrir shell en servicio
│      │   ├── Tecla 'q' / Ctrl+C → salir
│      │   ├── StatsMsg → actualizar CPU/MEM en tabla
│      │   ├── LogMsg → agregar linea al buffer
│      │   ├── EventMsg → actualizar status del servicio
│      │   └── WindowSizeMsg → recalcular layout
│      │
│      └── Salida limpia
│          - Cancelar contextos de subscripciones
│          - Servicios siguen corriendo
│
└── 4. Restaurar terminal
```

### Flags del CLI

```go
dashboardCmd.Flags().StringSliceVar(&only, "only", nil, "Show only specified services")
dashboardCmd.Flags().DurationVar(&statsInterval, "stats-interval", 2*time.Second, "Interval for stats polling")
dashboardCmd.Flags().IntVar(&logBuffer, "log-buffer", 1000, "Max log lines to keep in memory per service")
```

## Keyboard shortcuts

| Tecla | Accion |
|-------|--------|
| `↑` / `k` | Seleccionar servicio anterior |
| `↓` / `j` | Seleccionar servicio siguiente |
| `r` | Reiniciar servicio seleccionado |
| `s` | Detener servicio seleccionado |
| `l` | Expandir/colapsar panel de logs |
| `e` | Abrir shell interactivo en servicio (`docker exec -it ... sh`) |
| `/` | Buscar en logs (modo logs expandido) |
| `f` | Filtrar logs por nivel (INFO/WARN/ERROR) |
| `c` | Limpiar buffer de logs |
| `tab` | Alternar entre panel de servicios y logs |
| `ESC` | Volver a vista normal / cancelar accion |
| `q` / `Ctrl+C` | Salir del dashboard |

## Edge cases a manejar

| Caso | Comportamiento |
|------|---------------|
| Terminal muy pequena (< 80x24) | Mostrar mensaje pidiendo agrandar la terminal |
| Muchos servicios (> 20) | Scroll en la tabla de servicios con indicador visual |
| Lineas de log muy largas (> ancho terminal) | Truncar con `...`, mostrar completa en vista expandida con scroll horizontal |
| Servicio sin logs | Mostrar mensaje "No logs available" en el panel |
| Servicio que crashea en loop | Mostrar status `restarting` con contador de restarts |
| Docker no disponible | Mostrar error y salir con mensaje claro |
| Proyecto sin servicios activos | Mostrar mensaje "No services running. Run 'raioz up' first" |
| Resize de terminal | Recalcular layout completo via `WindowSizeMsg` de Bubble Tea |
| Perdida de conexion con Docker daemon | Mostrar warning, reintentar conexion cada 5s |
| Servicio tipo `command` (host) | Mostrar en tabla con status "host", sin opcion de exec/logs docker |
| Ctrl+C durante una accion (restart) | Cancelar accion, mantener dashboard abierto |

## Archivos a crear

```
internal/tui/
├── model.go            # Model principal, Init(), tipos de estado
├── update.go           # Update() — manejo de mensajes y teclas
├── view.go             # View() — renderizado con lipgloss
├── messages.go         # Tipos de mensajes (Stats, Log, Event)
├── subscriptions.go    # Goroutines: docker events, stats, logs
├── actions.go          # Acciones: restart, stop, exec
├── styles.go           # Constantes de estilo lipgloss
├── model_test.go       # Tests del modelo y Update
├── view_test.go        # Tests de renderizado
└── actions_test.go     # Tests de acciones
```

### Cambios en archivos existentes

| Archivo | Cambio |
|---------|--------|
| `cmd/raioz/main.go` | Registrar comando `dashboard` con alias `tui` |
| `cmd/zzz_i18n_descriptions.go` | Descripcion del comando via `i18n.T()` |
| `internal/i18n/locales/en.json` | ~15 keys para mensajes del dashboard |
| `internal/i18n/locales/es.json` | Traducciones al espanol |
| `go.mod` | Dependencias bubbletea, lipgloss, bubbles |

## Estimacion de complejidad

| Componente | Complejidad | Lineas estimadas |
|-----------|-------------|-----------------|
| Model + Init | Media | ~100 |
| Update (mensajes + teclas) | Alta | ~200 |
| View (renderizado) | Alta | ~250 |
| Messages (tipos) | Baja | ~50 |
| Subscriptions (docker streams) | Alta | ~200 |
| Actions (restart/stop/exec) | Media | ~100 |
| Styles (lipgloss) | Baja | ~50 |
| CLI command + flags | Baja | ~30 |
| i18n | Baja | ~30 |
| Tests | Media | ~200 |
| **Total** | | **~1210 lineas** |

## Riesgos

1. **Complejidad de Bubble Tea:** El patron MVU requiere manejar concurrencia cuidadosamente. Los streams de Docker generan mensajes asincrono que deben ser thread-safe.
   - **Mitigacion:** Usar los `tea.Cmd` nativos de Bubble Tea que manejan concurrencia internamente. No compartir estado mutable entre goroutines.

2. **Consumo de recursos del polling:** `docker stats` cada 2 segundos en un proyecto con 15 servicios genera carga.
   - **Mitigacion:** Usar `docker stats --no-stream --format json` con un solo llamado que retorna todos los contenedores. Hacer el intervalo configurable.

3. **Terminal sin soporte de colores:** Terminales minimalistas o pipes no soportan ANSI.
   - **Mitigacion:** Bubble Tea detecta automaticamente el nivel de color del terminal. Lipgloss degrada graciosamente.

4. **Shell interactivo (exec):** Abrir un shell dentro del dashboard requiere suspender la TUI.
   - **Mitigacion:** Usar `tea.ExecProcess` de Bubble Tea que suspende el programa, ejecuta el proceso interactivo y restaura la TUI al salir.

5. **Tamano del buffer de logs:** 1000 lineas por servicio x 20 servicios = 20,000 lineas en memoria.
   - **Mitigacion:** Ring buffer con limite configurable. Servicios no seleccionados mantienen solo las ultimas 100 lineas.

## Criterios de aceptacion

- [ ] `raioz dashboard` abre una interfaz interactiva con la lista de servicios
- [ ] La tabla muestra nombre, status, CPU, memoria y puertos de cada servicio
- [ ] Las metricas se actualizan automaticamente cada 2 segundos
- [ ] Al seleccionar un servicio se muestran sus logs en tiempo real
- [ ] `r` reinicia el servicio seleccionado con feedback visual
- [ ] `s` detiene el servicio seleccionado
- [ ] `l` expande el panel de logs a pantalla completa
- [ ] `e` abre un shell interactivo en el servicio
- [ ] `q` y `Ctrl+C` salen limpiamente sin detener servicios
- [ ] Resize de terminal recalcula el layout correctamente
- [ ] `--only` filtra los servicios mostrados
- [ ] Funciona en terminales de 80x24 minimo
- [ ] Docker events actualizan el status en tiempo real (sin polling)
- [ ] Los mensajes del UI pasan por `i18n.T()`
- [ ] Tests unitarios para model, update y acciones
- [ ] `raioz tui` funciona como alias de `raioz dashboard`
