# Roadmap

Plan de desarrollo de Raioz organizado en fases incrementales.

## Estado actual: v0.9 Beta

Raioz es funcional y cubre el ciclo completo de desarrollo con microservicios. Las 23 comandos existentes permiten a un equipo levantar, operar, depurar y limpiar entornos locales de manera declarativa.

### Comandos disponibles (23)

| Categoría | Comandos |
|-----------|----------|
| Ciclo de vida | `up`, `down`, `status`, `restart`, `exec`, `logs`, `ports`, `health` |
| Configuración | `init`, `check`, `list` |
| Modificadores | `override`, `ignore`, `link` |
| Limpieza | `clean`, `volumes` |
| Producción/CI | `migrate`, `compare`, `ci` |
| Workspace | `workspace` |
| Utilidades | `doctor`, `version`, `lang` |

### Features recientes

- `raioz up --only <service>` — Levantamiento parcial con resolución transitiva de dependencias
- `seed` en infra — Datos iniciales automáticos para bases de datos
- `raioz exec` — Ejecución de comandos en contenedores
- `raioz restart` — Reinicio selectivo de servicios
- `raioz volumes list/remove` — Gestión granular de volúmenes
- `raioz doctor` — Diagnóstico del entorno

---

## Fase 1: Developer Experience (v1.0)

**Objetivo:** Hacer que raioz sea indispensable en el día a día del desarrollador.

### 1.1 `raioz graph` — Visualización de dependencias

```bash
raioz graph                # ASCII en terminal
raioz graph --format dot   # Graphviz DOT
raioz graph --format json  # JSON para herramientas
```

Muestra el grafo de servicios e infraestructura con sus dependencias. Invaluable para onboarding y para entender proyectos complejos.

- **Impacto:** Alto
- **Esfuerzo:** ~200 líneas
- **Spec:** [FEATURE_GRAPH.md](./FEATURE_GRAPH.md)

### 1.2 `raioz up --watch` — Hot-Reload

```bash
raioz up --watch           # Levanta y observa cambios
raioz up --watch --only api  # Solo api, observando
```

Detecta cambios en archivos de código y reinicia automáticamente solo el servicio afectado. Elimina el ciclo manual de restart.

- **Impacto:** Muy alto
- **Esfuerzo:** ~640 líneas
- **Spec:** [FEATURE_WATCH.md](./FEATURE_WATCH.md)
- **Dependencia:** `github.com/fsnotify/fsnotify`

### 1.3 `raioz snapshot` — Guardar/Restaurar estado de datos

```bash
raioz snapshot create "datos-limpios"
raioz snapshot restore "datos-limpios"
raioz snapshot list
raioz snapshot delete "datos-viejos"
```

Guarda el estado de todos los volúmenes Docker del proyecto y permite restaurarlos instantáneamente. El "undo" para datos de desarrollo.

- **Impacto:** Muy alto
- **Esfuerzo:** ~500 líneas
- **Spec:** [FEATURE_SNAPSHOT.md](./FEATURE_SNAPSHOT.md)

---

## Fase 2: Productividad del equipo (v1.1)

**Objetivo:** Features que benefician al equipo completo, no solo al individuo.

### 2.1 Dashboard TUI — Interfaz interactiva

```bash
raioz dashboard    # Abre interfaz interactiva en terminal
```

Vista en tiempo real de todos los servicios con logs, métricas y controles. Reemplaza la necesidad de múltiples terminales.

- **Impacto:** Alto
- **Esfuerzo:** ~1200 líneas
- **Spec:** [FEATURE_TUI.md](./FEATURE_TUI.md)
- **Dependencia:** `github.com/charmbracelet/bubbletea`

### 2.2 `raioz tunnel` — Exponer servicios a Internet

```bash
raioz tunnel api           # https://abc123.tunnel.dev
raioz tunnel list
raioz tunnel stop api
```

Expone servicios locales a Internet para testing de webhooks (Stripe, GitHub), desarrollo mobile y demos a stakeholders.

- **Impacto:** Medio
- **Esfuerzo:** ~400 líneas
- **Spec:** [FEATURE_TUNNEL.md](./FEATURE_TUNNEL.md)
- **Dependencia:** `cloudflared` o `bore`

### 2.3 Service Templates — Stacks predefinidos

```bash
raioz init --template node-postgres
raioz init --template go-redis
raioz init --template python-mongo
```

Templates predefinidos para stacks comunes. Reduce el tiempo de setup de un nuevo proyecto de minutos a segundos.

- **Impacto:** Medio
- **Esfuerzo:** ~300 líneas (templates + CLI)

---

## Fase 3: Escala (v1.2+)

**Objetivo:** Soportar equipos grandes y proyectos complejos.

### 3.1 Entornos remotos compartidos

```bash
raioz up --remote              # Levanta en servidor compartido
raioz connect api              # Port-forward al servicio remoto
raioz remote status            # Ver entornos remotos del equipo
```

Para equipos donde las laptops no tienen suficiente RAM/CPU para correr todos los servicios. Un servidor compartido corre la infra, los desarrolladores se conectan via port-forward.

- **Impacto:** Muy alto
- **Esfuerzo:** ~2000 líneas
- **Dependencia:** SSH, servidor dedicado

### 3.2 Plugin system

```bash
raioz plugin install raioz-datadog    # Instalar plugin
raioz plugin list                      # Ver plugins
```

Permite extender raioz con plugins para: métricas (Datadog/Grafana), notificaciones (Slack), logging centralizado, etc.

- **Impacto:** Medio
- **Esfuerzo:** ~800 líneas (framework)

### 3.3 Multi-proyecto orquestado

```bash
raioz up --projects billing,auth,gateway  # Levantar múltiples proyectos coordinados
```

Para equipos que trabajan en múltiples proyectos que interactúan entre sí. Actualmente cada proyecto se gestiona por separado.

- **Impacto:** Alto
- **Esfuerzo:** ~600 líneas

---

## Priorización

```
                    Alto impacto
                         │
        ┌────────────────┼────────────────┐
        │                │                │
        │  snapshot   watch              │
        │  graph                          │
        │                │                │
  Bajo  ├────────────────┼────────────────┤  Alto
esfuerzo│                │                │  esfuerzo
        │  templates     │     TUI        │
        │                │     tunnel     │
        │                │     remote     │
        │                │                │
        └────────────────┼────────────────┘
                         │
                    Bajo impacto
```

### Orden recomendado de implementación

| Orden | Feature | Justificación |
|-------|---------|---------------|
| 1 | `graph` | Bajo esfuerzo, alto impacto en onboarding |
| 2 | `snapshot` | Problema real que nadie resuelve bien |
| 3 | `watch` | Mayor ahorro de tiempo diario |
| 4 | `templates` | Reduce fricción de adopción |
| 5 | `dashboard` | "Wow factor" para demos y adopción |
| 6 | `tunnel` | Nicho pero valioso para ciertos equipos |
| 7 | `remote` | Requiere infraestructura, fase tardía |
| 8 | `plugins` | Solo cuando haya comunidad |

---

## Métricas de éxito

| Métrica | Objetivo v1.0 | Objetivo v1.2 |
|---------|---------------|---------------|
| Tiempo de onboarding | < 5 min | < 2 min |
| Ciclo edit-test | < 10 seg (con watch) | < 5 seg |
| Equipos adoptando | 3+ | 10+ |
| Comandos diarios por dev | 5-10 | 3-5 (dashboard reduce) |
| Uptime del entorno | 95% | 99% |

---

## Cómo contribuir

Cada feature tiene su documento de especificación detallado en `docs/FEATURE_*.md`. Para contribuir:

1. Lee la spec de la feature que te interesa
2. Revisa los criterios de aceptación
3. Sigue la arquitectura existente (Clean Architecture, DI, i18n)
4. Escribe tests unitarios + al menos un test de integración
5. Abre un PR con referencia a la spec
