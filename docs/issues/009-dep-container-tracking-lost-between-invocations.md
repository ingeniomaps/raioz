# 009 — raioz pierde tracking de dep containers entre invocaciones

## Resumen

Después de que `raioz up --only <dep>` crea y deja corriendo un contenedor (postgres, redis, etc.), invocaciones posteriores de `raioz status` y `raioz up` no reconocen ese contenedor como suyo. Se manifiesta en dos síntomas con la misma raíz: **el mapping entre la `dep` declarada en `raioz.yaml` y el contenedor Docker real que creó raioz en un run anterior se pierde**.

Mientras el contenedor sigue siendo totalmente funcional (healthy, sirviendo en su puerto), raioz lo trata como "algo externo" y obliga al usuario a borrarlo manualmente para continuar.

## Reproducción

En `yemdiou/` (config: `raioz.yaml` con `dependencies.postgres.compose: .infra/services/postgres.yml`, `publish: 5432`):

```bash
cd /home/manuel/Code/yemdiou

# 1. Levanta solo la dep
raioz up --only postgres
# → Container yemdiou-postgres Created / Started / healthy

# 2. Consulta el estado
raioz status
# → postgres  stopped    -        -          :latest     ❌ (el contenedor está running healthy)

# 3. Intenta levantar el service que depende de la dep
raioz up --only api
# → [!!] El puerto 5432 esta en uso por el contenedor 'yemdiou-postgres'   ❌
# → (raioz mismo creó ese contenedor hace 30 segundos)
#   Como deseas resolver esto?
#     1) Asignar automaticamente el siguiente puerto libre
#     2) Especificar un puerto
#     3) Lo resolvere yo mismo (detener raioz up)
```

Workaround actual que usamos:

```bash
docker stop yemdiou-postgres && docker rm yemdiou-postgres
raioz up    # ahora sí, en una sola pasada desde cero
```

## Síntoma 1 — `status` reporta stopped

```
$ raioz status
--------------------------------------------------
YEMDIOU
--------------------------------------------------

Dependencies (1)
  postgres           stopped    -        -          :latest

Services (1)
  api                npm        running    pid:609652
```

Pero:

```
$ docker ps --filter name=yemdiou-postgres
CONTAINER ID   IMAGE                 STATUS                    PORTS                     NAMES
a1b2c3d4e5f6   postgres:18.3-alpine  Up 5 minutes (healthy)    127.0.0.1:5432->5432/tcp  yemdiou-postgres
```

## Síntoma 2 — pre-flight falsea conflicto con sí mismo

El pre-flight de puertos ve que 5432 está bindeado por `yemdiou-postgres` (container externo desde su perspectiva) y lo reporta como conflicto. Idealmente debería reconocer que ese container **es el dep que íbamos a levantar** y saltarse la creación (reutilizarlo).

## Root cause probable

Ambos síntomas comparten una hipótesis: **raioz no persiste ni reconstruye el mapping `dep name → container name` entre invocaciones**.

Evidencia en el state file creado por `raioz up --only postgres`:

```json
// /home/manuel/Code/yemdiou/.raioz.state.json
{
  "project": "yemdiou",
  "lastUp": "2026-04-21T14:25:03-05:00",
  "hostPIDs": { "api": 609652 },
  "networkName": "yemdiou-net"
}
```

Notar: **`hostPIDs` sólo trackea host services**. No hay equivalente `depContainers: { postgres: "yemdiou-postgres" }` ni similar. Raioz parece delegar en Docker ("Docker sabe lo suyo") pero no consulta Docker para reconstruir el mapping al arrancar.

Archivos a revisar:

- `internal/state/` — qué se persiste al `up` y al `down`
- `internal/orchestrator/up.go` (o equivalente) — dónde se decide "este dep ya está corriendo vs crear de nuevo"
- `internal/preflight/ports.go` — dónde se calculan conflictos de puerto, cómo distingue procesos/contenedores externos de los propios
- `internal/docker/discover.go` (o equivalente) — si existe lógica de descubrir containers por labels

## Fix propuesto

### Opción A — agregar labels raioz a cada container + consultar al arrancar (recomendada)

Al crear containers para deps, poner labels determinísticas:

```
raioz.project=yemdiou
raioz.dep=postgres
raioz.managed=true
```

Y en cada invocación de `raioz up`/`status`:

1. `docker ps -a --filter label=raioz.project=yemdiou` → reconstruir map.
2. Para cada dep en la config:
   - Si existe container con `raioz.dep=<name>` y está running+healthy → **reutilizar, saltar pre-flight de puerto para ese container**.
   - Si existe stopped → decidir (restart vs recreate) según strategy.
   - Si no existe → crear.

```diff
// internal/orchestrator/up.go (pseudocódigo)
  for _, dep := range config.Dependencies {
+     existing, err := docker.FindByLabel("raioz.project", project, "raioz.dep", dep.Name)
+     if err == nil && existing.IsHealthy() {
+         log.Info("reusing existing container", "name", existing.Name)
+         continue // no recrear, no preflight de puerto
+     }
      createContainer(dep)
  }
```

Y en preflight de puertos:

```diff
// internal/preflight/ports.go
  owner := findProcessOnPort(port)
- if owner != nil {
+ if owner != nil && !isOwnManagedContainer(owner, project) {
      return PortConflict{...}
  }
```

Pro: arreglo estructural. Habilita `raioz up` idempotente (mi intent 99% del tiempo: "asegurá que estén corriendo, no me hagas pensar"). Base para features como `raioz up --recreate` explícito.

Contra: requiere migración para containers pre-existentes (los que ya fueron creados sin labels). Podés tratarlos como "externos" hasta que el user haga un `down+up` limpio.

### Opción B — persistir mapping en state file (quick fix)

Agregar a `.raioz.state.json`:

```json
{
  "depContainers": {
    "postgres": { "containerName": "yemdiou-postgres", "containerId": "a1b2c3d4..." }
  }
}
```

Se escribe al crear cada container, se lee al arrancar.

Pro: menos invasivo, no requiere cambios a containers creados.

Contra: doble fuente de verdad (state file + Docker). Si el user hace `docker rm yemdiou-postgres` fuera de raioz, el state queda mintiendo. A se auto-recupera (consulta Docker en vivo).

## Recomendación

**A**. Los labels son baratos de agregar, desbloquean reuso idempotente, y resuelven los 2 síntomas de un tiro. B es un hack sobre algo que Docker ya sabe.

Un mix razonable: implementar A como la fuente principal, y en state file guardar sólo el `networkName` + `lastUp` (lo que hoy ya guarda) — sin duplicar metadata de containers.

## Tradeoffs

- A requiere que el usuario haga un `raioz down` único después de upgradear raioz, para que los containers queden labelled. Se puede mitigar con un command de migración (`raioz migrate labels` que los relabela).
- B es tentador para shippar hoy, pero va a arrastrar bugs de sincronización que A elimina.

## Prioridad

**Media-alta**. No bloquea workflows (hay workaround: `docker rm` + `raioz up` fresh), pero:

1. **Rompe la confianza** en `raioz status` — una mentira constante baja la utilidad del comando.
2. **Rompe el mental model** de `raioz up --only` — el usuario espera "levantá esto y lo que dependa de esto, reusá lo que ya esté". Actualmente se comporta como "armá todo desde cero y si hay algo en el medio me plantás".
3. Es la primera impresión que se lleva alguien que usa raioz para su stack (lo vivimos hoy integrando postgres al proyecto yemdiou).

## Contexto de descubrimiento

Descubierto el 2026-04-21 configurando `yemdiou/` (NestJS + Prisma + PostgreSQL). Setup: `.infra/` clonado de `ingeniomaps/infra-services`, `raioz.yaml` en la raíz con `postgres` como dep via `compose:`, `api` como service. Primera invocación (`raioz up --only postgres`) funcionó impecable. La segunda (`raioz up --only api`) falló el preflight. La tercera (`raioz status`) mintió. El flow que terminó funcionando fue `docker stop && docker rm && raioz up` (sin `--only`).

Sin esto, cada vez que cortés el flujo para correr `bun run db:migrate` y luego querés levantar el api, te topás con el mismo obstáculo.

## Resolución

Implementada Opción A en `fix/up-detach-default`. Los labels `com.raioz.*` ya se aplicaban a deps creadas vía `compose:` (vía el overlay de `ImageRunner.writeInfraOverlay`), pero no se consultaban en los puntos donde el bug se manifestaba:

1. **Síntoma 1 (status miente)** — `internal/app/status_orchestrated.go::queryDepStatus`: si el lookup por nombre canónico devuelve "stopped", se hace un segundo intento por labels (`com.raioz.project` + `com.raioz.service`) y, si encuentra un container, se reporta su estado real. Para esto se agregó `DockerRunner.FindManagedContainerByService(ctx, project, service)` en la interfaz, su implementación en `internal/infra/docker/runner_impl.go` y el stub correspondiente en el mock.
2. **Síntoma 2 (preflight falsea conflicto)** — `internal/docker/ports.go::IdentifyPortOccupant`: ahora consulta primero `com.raioz.managed=true` y deriva `ProjectName` desde `com.raioz.project`. El nombre-prefix-heuristic queda como fallback para containers viejos pre-labels. La consecuencia inmediata: en `port_resolve.go:42`, los containers del propio proyecto se detectan como tales aunque el usuario haya nombrado al container con un literal en su compose.

Tests añadidos en `internal/app/status_extra_test.go`: fallback usado solo cuando el canonical falla, no usado cuando hay hit, y caso "no matches" sigue siendo "stopped". El método `FindManagedContainerByService` reutiliza el helper `docker.ListContainersByLabels` que ya existía.

Pendiente (no bloquea esta resolución):
- Migración explícita: aún hay containers en disco creados antes de los labels (improbable en flota actual). Si aparece, un `raioz down`/`up` los relabela.
- `IdentifyPortOccupant` queda con el fallback prefix-name; cuando los labels se asuman como invariante, se puede eliminar.
