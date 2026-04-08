# Feature: `raioz tunnel` (Exposicion de servicios locales)

## Resumen

Comando para exponer servicios locales a internet mediante tunnels seguros. Permite probar webhooks (Stripe, GitHub, Twilio), desarrollar con dispositivos moviles en la misma red, y compartir entornos de desarrollo con companeros sin desplegar.

```bash
raioz tunnel api              # Expone api en https://abc123.raioz.dev
raioz tunnel api --port 3000  # Especifica puerto manualmente
raioz tunnel list             # Ver tunnels activos
raioz tunnel stop api         # Detener tunnel de un servicio
raioz tunnel stop --all       # Detener todos los tunnels
```

## Valor para el desarrollador

### Webhook testing (Stripe, GitHub, Twilio)

**Sin tunnel:**
```
1. Push a branch de staging             (30 seg)
2. Esperar CI/CD                        (3-5 min)
3. Configurar webhook URL en Stripe     (30 seg)
4. Trigger evento                       (10 seg)
5. Ver logs en staging                  (30 seg)
6. Encontrar bug, repetir desde paso 1
Total: ~5-10 min por iteracion
```

**Con tunnel:**
```
1. raioz tunnel api                     (2 seg)
2. Copiar URL en Stripe webhook config  (10 seg)
3. Trigger evento                       (10 seg)
4. Ver logs locales en tiempo real      (inmediato)
5. Editar codigo, probar de nuevo       (inmediato)
Total: ~30 seg por iteracion
```

### Desarrollo movil

**Sin tunnel:**
```
1. Buscar IP local                      (10 seg)
2. Configurar app con IP:puerto         (30 seg)
3. Descubrir que HTTPS es requerido     (5 min)
4. Configurar certificados locales      (10 min)
5. Repetir cuando cambia la IP          (cada vez)
```

**Con tunnel:**
```
1. raioz tunnel api                     (2 seg)
2. Usar URL HTTPS publica en la app     (10 seg)
3. URL persiste mientras el tunnel viva (sin repetir)
```

### Compartir con companeros

```bash
raioz tunnel api
# → https://abc123.trycloudflare.com
# Compartir URL por Slack — companero puede probar sin clonar ni levantar nada
```

## Diseno tecnico

### Opciones de backend

#### Opcion A: cloudflared (Cloudflare Tunnel) — Recomendada

| Aspecto | Detalle |
|---------|---------|
| Licencia | Apache 2.0 |
| Costo | Gratis (quick tunnels, sin cuenta) |
| HTTPS | Automatico |
| Instalacion | Binario unico, disponible en brew/apt/go install |
| Comando | `cloudflared tunnel --url http://localhost:3000` |
| Salida | URL aleatoria: `https://abc123.trycloudflare.com` |
| Latencia | Baja (~50ms overhead) |
| Limitaciones | URLs aleatorias en modo gratis, sin subdominio fijo |

#### Opcion B: bore (open source, self-hostable)

| Aspecto | Detalle |
|---------|---------|
| Licencia | MIT |
| Costo | Gratis (servidor publico en bore.pub) |
| HTTPS | No incluido (HTTP only con bore.pub) |
| Instalacion | `cargo install bore-cli` o binario |
| Comando | `bore local 3000 --to bore.pub` |
| Salida | URL: `bore.pub:XXXXX` |
| Latencia | Baja |
| Limitaciones | Sin HTTPS nativo, servidor publico puede ser inestable |

#### Opcion C: SSH reverse tunnel (custom)

| Aspecto | Detalle |
|---------|---------|
| Costo | Requiere un servidor propio con SSH |
| HTTPS | Manual (nginx + certbot en servidor) |
| Complejidad | Alta — configurar servidor, DNS, certificados |
| Ventaja | Control total, sin dependencias externas |

### Recomendacion

**cloudflared como primario, bore como fallback.**

Razon: cloudflared ofrece HTTPS gratis sin cuenta, URLs publicas inmediatas, y es mantenido activamente por Cloudflare. bore sirve como alternativa si el desarrollador prefiere self-hosting o tiene restricciones de red con Cloudflare.

```go
// Orden de deteccion
func detectBackend() TunnelBackend {
    if commandExists("cloudflared") {
        return CloudflaredBackend{}
    }
    if commandExists("bore") {
        return BoreBackend{}
    }
    return nil // error: ningun backend disponible
}
```

## Arquitectura

```
┌─────────────────┐     ┌──────────────┐     ┌──────────────────┐
│  raioz tunnel   │────→│  PortResolver│────→│  TunnelManager   │
│  (CLI command)  │     │              │     │                  │
└─────────────────┘     └──────────────┘     └───────┬──────────┘
                                                     │
                                    ┌────────────────┼────────────────┐
                                    │                │                │
                               ┌────▼─────┐   ┌─────▼──────┐  ┌─────▼──────┐
                               │cloudflared│   │   bore     │  │  Registro  │
                               │ Backend   │   │  Backend   │  │  tunnels   │
                               │          │   │            │  │  .json     │
                               └──────────┘   └────────────┘  └────────────┘
```

### Componentes

#### 1. Port Resolver (`internal/tunnel/resolver.go`)

Detecta automaticamente el puerto del servicio a partir de la configuracion:

```go
// ResolvePort determina el puerto expuesto de un servicio
func (r *Resolver) ResolvePort(serviceName string) (int, error)
```

Logica de resolucion:
1. Buscar en la config del servicio (`services[name].ports`) el primer puerto mapeado al host
2. Si el servicio tiene override, usar los puertos del override
3. Si se pasa `--port`, usar ese valor directamente
4. Si no se encuentra, retornar error con sugerencia de usar `--port`

#### 2. Tunnel Backend Interface (`internal/tunnel/backend.go`)

```go
type TunnelBackend interface {
    // Name retorna el nombre del backend ("cloudflared", "bore")
    Name() string
    // Available verifica si el binario esta instalado
    Available() bool
    // Start inicia un tunnel y retorna la URL publica
    Start(ctx context.Context, localPort int) (*TunnelInfo, error)
    // Stop detiene un tunnel por PID
    Stop(pid int) error
}

type TunnelInfo struct {
    ServiceName string    `json:"service_name"`
    LocalPort   int       `json:"local_port"`
    PublicURL   string    `json:"public_url"`
    Backend     string    `json:"backend"`
    PID         int       `json:"pid"`
    StartedAt   time.Time `json:"started_at"`
}
```

#### 3. Cloudflared Backend (`internal/tunnel/cloudflared.go`)

```go
func (c *CloudflaredBackend) Start(ctx context.Context, localPort int) (*TunnelInfo, error) {
    // Ejecuta: cloudflared tunnel --url http://localhost:{port}
    // Parsea stdout para extraer la URL publica
    // Retorna TunnelInfo con PID del proceso hijo
}
```

Deteccion de URL: cloudflared imprime la URL en stderr con el patron:
```
INF +---------------------------------------------------+
INF |  Your quick Tunnel has been created! Visit it at:  |
INF |  https://abc123.trycloudflare.com                  |
INF +---------------------------------------------------+
```

#### 4. Bore Backend (`internal/tunnel/bore.go`)

```go
func (b *BoreBackend) Start(ctx context.Context, localPort int) (*TunnelInfo, error) {
    // Ejecuta: bore local {port} --to bore.pub
    // Parsea stdout para extraer puerto remoto
    // Retorna TunnelInfo con URL bore.pub:{remotePort}
}
```

#### 5. Tunnel Registry (`internal/tunnel/registry.go`)

Persiste tunnels activos en `~/.raioz/tunnels.json`:

```json
{
  "tunnels": [
    {
      "service_name": "api",
      "local_port": 3000,
      "public_url": "https://abc123.trycloudflare.com",
      "backend": "cloudflared",
      "pid": 12345,
      "started_at": "2026-04-06T10:30:00Z"
    }
  ]
}
```

Operaciones:
- `Save(info TunnelInfo)` — agrega o actualiza entrada
- `Remove(serviceName string)` — elimina entrada
- `List() []TunnelInfo` — retorna todos los tunnels
- `Cleanup()` — elimina entradas cuyo PID ya no existe

#### 6. Tunnel Manager (`internal/tunnel/manager.go`)

Orquesta el flujo completo:

```go
type Manager struct {
    backend  TunnelBackend
    registry *Registry
    resolver *Resolver
}

func (m *Manager) StartTunnel(serviceName string, portOverride int) (*TunnelInfo, error)
func (m *Manager) StopTunnel(serviceName string) error
func (m *Manager) StopAll() error
func (m *Manager) ListTunnels() ([]TunnelInfo, error)
```

### Flujo de ejecucion

```
raioz tunnel api
│
├── 1. Verificar que el proyecto esta levantado (state.json existe)
│
├── 2. Detectar backend disponible (cloudflared > bore)
│      └── Si ninguno: error con instrucciones de instalacion
│
├── 3. Resolver puerto del servicio
│      ├── Leer config (.raioz.json) → buscar ports del servicio "api"
│      ├── Si hay --port, usar ese
│      └── Si no se encuentra: error + sugerencia --port
│
├── 4. Verificar que no hay tunnel activo para ese servicio
│      └── Si existe: mostrar URL existente, preguntar si reemplazar
│
├── 5. Iniciar tunnel
│      ├── Spawn proceso en background
│      ├── Esperar URL publica (timeout 15s)
│      └── Registrar en tunnels.json
│
├── 6. Mostrar resultado
│      ┌─────────────────────────────────────────────┐
│      │  Tunnel activo para api                     │
│      │                                              │
│      │  URL:   https://abc123.trycloudflare.com    │
│      │  Local: http://localhost:3000               │
│      │  PID:   12345                                │
│      │                                              │
│      │  URL copiada al portapapeles                │
│      └─────────────────────────────────────────────┘
│
└── 7. Proceso sigue en background — terminal queda libre
```

### Integracion con `raioz down`

```
raioz down
│
├── ... (flujo normal de down)
│
└── Cleanup de tunnels
    ├── Leer tunnels.json
    ├── Matar procesos activos (SIGTERM)
    ├── Esperar 5s, SIGKILL si no mueren
    └── Limpiar tunnels.json
```

### Flags del CLI

```go
// raioz tunnel <servicio>
tunnelCmd.Flags().IntVar(&port, "port", 0, i18n.T("tunnel.flag.port"))
tunnelCmd.Flags().StringVar(&backend, "backend", "", i18n.T("tunnel.flag.backend"))

// raioz tunnel stop
tunnelStopCmd.Flags().BoolVar(&all, "all", false, i18n.T("tunnel.flag.stop_all"))
```

## Edge cases a manejar

| Caso | Comportamiento |
|------|---------------|
| Binario del backend no instalado | Error con instrucciones de instalacion por OS |
| Puerto no encontrado en config | Error con sugerencia de usar `--port` |
| Servicio no existe en el proyecto | Error con lista de servicios disponibles |
| Tunnel ya activo para el servicio | Mostrar URL existente, ofrecer reemplazar |
| Puerto local no accesible | Warning: "el servicio puede no estar corriendo" |
| Proceso del tunnel muere inesperadamente | `raioz tunnel list` detecta PIDs muertos y limpia |
| Multiples tunnels simultaneos | Soportado — cada servicio tiene su proceso |
| `raioz down` con tunnels activos | Limpieza automatica de todos los tunnels |
| Sin conexion a internet | Error claro del backend, timeout controlado |
| cloudflared no retorna URL en 15s | Timeout con error y sugerencia de reintentar |
| Servicio tipo `command` (host) | Soportado si se especifica `--port` |
| Servicio tipo `image` sin ports | Error: "servicio no expone puertos" |

## Archivos a crear

```
internal/tunnel/
├── backend.go          # Interface TunnelBackend + TunnelInfo
├── cloudflared.go      # Implementacion cloudflared
├── bore.go             # Implementacion bore
├── resolver.go         # Resolucion automatica de puertos
├── registry.go         # Persistencia en tunnels.json
├── manager.go          # Orquestacion del flujo
├── backend_test.go     # Tests de backends (con mocks)
├── resolver_test.go    # Tests de resolucion de puertos
└── registry_test.go    # Tests del registro
```

### Cambios en archivos existentes

| Archivo | Cambio |
|---------|--------|
| `cmd/raioz/main.go` | Registrar subcomando `tunnel` |
| `cmd/tunnel.go` | Comando Cobra con subcomandos (start, list, stop) |
| `cmd/zzz_i18n_descriptions.go` | Descripciones i18n del comando tunnel |
| `internal/app/upcase/usecase.go` | Llamar cleanup de tunnels en el flujo de `down` |
| `internal/app/down.go` | Integrar `tunnel.Manager.StopAll()` |
| `internal/i18n/locales/en.json` | ~20 keys para mensajes de tunnel |
| `internal/i18n/locales/es.json` | Traducciones al espanol |

## Estimacion de complejidad

| Componente | Complejidad | Lineas estimadas |
|-----------|-------------|-----------------|
| Backend interface + types | Baja | ~40 |
| Cloudflared backend | Media | ~80 |
| Bore backend | Media | ~70 |
| Port resolver | Baja | ~50 |
| Registry (persistencia) | Baja | ~60 |
| Manager (orquestacion) | Media | ~100 |
| CLI command | Baja | ~60 |
| i18n keys | Baja | ~20 |
| Tests | Media | ~180 |
| **Total** | | **~660 lineas** |

## Riesgos

1. **Dependencia de binarios externos:** cloudflared y bore deben estar instalados por el usuario. Raioz no los empaqueta.
   - **Mitigacion:** Mensaje de error claro con instrucciones de instalacion por OS (`brew install cloudflared`, `apt install cloudflared`, link a releases).

2. **Parseo fragil de stdout/stderr:** La deteccion de URL depende del formato de salida de cloudflared/bore, que puede cambiar entre versiones.
   - **Mitigacion:** Parsear con regex flexible, incluir version minima testeada en docs, tests de integracion con binario real.

3. **Procesos zombi:** Si raioz crashea, los procesos de tunnel quedan huerfanos.
   - **Mitigacion:** `raioz tunnel list` detecta PIDs muertos via `os.FindProcess`. `raioz tunnel stop --all` como limpieza manual.

4. **Firewalls corporativos:** Algunos entornos bloquean conexiones salientes a Cloudflare.
   - **Mitigacion:** bore como fallback, posibilidad de configurar servidor bore propio.

## Criterios de aceptacion

- [ ] `raioz tunnel api` expone el servicio y muestra URL publica
- [ ] `raioz tunnel api --port 3000` usa el puerto especificado
- [ ] `raioz tunnel list` muestra tunnels activos con URL, puerto, PID y tiempo
- [ ] `raioz tunnel stop api` detiene el tunnel del servicio
- [ ] `raioz tunnel stop --all` detiene todos los tunnels
- [ ] Deteccion automatica de puerto desde la configuracion del servicio
- [ ] Fallback de cloudflared a bore si cloudflared no esta disponible
- [ ] Error claro si ningun backend esta instalado, con instrucciones
- [ ] Registro de tunnels en `~/.raioz/tunnels.json`
- [ ] Limpieza automatica de tunnels en `raioz down`
- [ ] Limpieza de PIDs muertos en `raioz tunnel list`
- [ ] Funciona con servicios de tipo `git`, `image`, `local`
- [ ] Tests unitarios para resolver, registry y manager
- [ ] Mensajes i18n en ingles y espanol
