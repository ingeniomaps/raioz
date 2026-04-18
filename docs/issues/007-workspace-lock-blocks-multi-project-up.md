# 007 — Workspace lock bloquea correr múltiples proyectos del mismo workspace

## Resumen

El lock de `raioz up` es per-workspace y se mantiene durante toda la vida del proceso foreground. En un workspace multi-proyecto (ej: `gouduet` con `keycloak/`, `app/`, `api/`) no se pueden correr dos `raioz up` simultáneos porque el segundo falla con `LOCK_ERROR`. Y `raioz up` no ofrece modo detach — queda foreground para logs aunque los contenedores ya estén arriba.

## Reproducción

```bash
# Dos proyectos en el mismo workspace (ambos con raioz.yaml workspace: gouduet)
cd /home/manuel/Code/gouduet/gouduet/app && raioz up   # OK, se queda foreground
cd /home/manuel/Code/gouduet/gouduet/api && raioz up   # FAIL
```

## Síntoma

```
time=... level=ERROR msg="Failed to acquire lock" operation="raioz up"
workspace=/home/manuel/.raioz/workspaces/gouduet
error="lock already exists: another raioz process may be running"
[error] [LOCK_ERROR] Error al adquirir lock
```

## Root cause (probable — a confirmar en el código)

- Archivo de lock en `~/.raioz/workspaces/<workspace>/<algo>.lock`.
- Se adquiere al inicio de `raioz up` y se libera al salir del proceso.
- El proceso no sale solo tras traer up los servicios: se queda tailando logs hasta `Ctrl+C`.
- Consecuencia: solo un `raioz up` foreground por workspace, siempre.

Pistas para el grep:
- `internal/cli/up.go` o similar — donde se instancia el lock
- `internal/app/state` / `internal/infra/state` — adquisición y release
- Ver si `raioz down` dejaría servicios huérfanos o si hay un modo detach ya implementado

## Fix propuesto (dos opciones)

### Opción A — lock per-project (cambio de scope)

```diff
- lockPath := filepath.Join(workspaceDir, "workspace.lock")
+ lockPath := filepath.Join(workspaceDir, "projects", projectName+".lock")
```

Pro: el modelo conceptual ya es "projects sharing a workspace network". El lock no necesita ser workspace-wide si las operaciones de state son por-proyecto.

Contra: si hay operaciones que SÍ requieren lock workspace-wide (ej: crear/borrar la red Docker), hay que distinguir.

### Opción B — `raioz up --detach` / `-d`

```diff
+ var detach bool
+ cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Exit after services are up; keep them running")
  ...
  // tras bring-up:
  if detach {
+     releaseLock()
+     return nil
  }
+ // si no --detach: mantener foreground para tailing, pero también liberar el lock aquí si el modelo es workspace-wide
```

Pro: no cambia el modelo del lock, sólo agrega una salida temprana. Más conservador.

Contra: cada usuario tiene que acordarse de `-d` en N-1 terminales; UX mediocre en workspaces grandes.

Recomendación combinada: implementar **A** (lock per-project) y mantener `raioz up` foreground como default. Operaciones workspace-wide escalan a un lock global separado y explícito.

## Tradeoffs

- Si el lock per-project destapa race conditions en writes a `~/.raioz/workspaces/<workspace>/state.json` (archivo compartido), hay que o bien segmentar el state, o serializar writes con un mutex in-process/file-per-segment. Esto es trabajo real.
- Opción B es un parche que no arregla el modelo, pero desbloquea usuarios en 30 minutos de trabajo.

## Prioridad

Alta para workflows multi-proyecto reales. En `gouduet/` tenemos 3+ proyectos en el mismo workspace (`keycloak`, `app`, `api`, pronto más microservicios); el bloqueo impide usar raioz como meta-orquestador, que es su valor central. Hoy el workaround en `gouduet` es correr servicios por fuera de raioz (rompe el model).

## Workaround actual (documentado por si alguien más lo hace)

Correr los servicios secundarios directamente (`go run`, `bun run dev`, `docker compose up`) sin raioz. Se pierde:
- Resolución de hostnames via Caddy (`*.gouduet.dev`)
- Service discovery de envs (`POSTGRES_HOST`, etc.)
- Teardown unificado vía `raioz down`

## Resolución

Resuelto en la rama `fix/up-detach-default` con dos cambios coordinados:

1. `internal/lock/lock.go`: `Release()` es idempotente y puede llamarse temprano.
2. `internal/app/upcase/usecase.go`: el lock se libera explícitamente después de `showSummary`, antes de cualquier loop foreground.

Cambio de comportamiento asociado: `raioz up` sin flags ahora **sale limpio** tras traer los servicios (default detach). Los modos foreground quedan opt-in:

- `raioz up --attach`: stream de logs (ya existía)
- `raioz up --watch`: file watching + restart (nuevo flag; antes era implícito como default)

Esto resuelve el bloqueo y además alinea raioz con su rol de meta-orquestador: dejar la terminal viva para ver logs no es el default, es elección del usuario.
