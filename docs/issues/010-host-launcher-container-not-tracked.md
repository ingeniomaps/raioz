---
name: 010 — host launcher con compose project propio queda como "stopped" aunque el contenedor está vivo
description: Cuando el `command` de un service host es un wrapper (make/bash) que arranca docker compose con un `-p <project>` y `-f <file>` no estándar, la autodetección de compose no lo encuentra y `raioz status` reporta stopped pese a que el contenedor sigue running.
type: project
---

# 010 — host launcher con compose project propio queda como "stopped" aunque el contenedor está vivo

## Resumen

Service host configurado con `command: make dev-docker` (un wrapper que ejecuta `docker compose -p hypixo-accounts-dev -f docker/compose.hotreload.yaml up -d --build`). El contenedor (`hypixo-accounts`) queda corriendo después del `up`, Caddy lo enruta correctamente, y `https://accounts.hypixo.dev` responde 307 a `/en` como se espera. Sin embargo, segundos después de `raioz up`, `raioz status` reporta el servicio como **stopped**.

Es **el caso espejo del issue 008**:
- 008 = host process muere con error 1, raioz dice running (false positive).
- 010 = host launcher termina OK con exit 0 (porque hizo `compose up -d`), contenedor sigue vivo, raioz dice stopped (false negative).

## Reproducción

En `hypixo/ui/accounts` (`raioz.yaml`):

```yaml
workspace: hypixo
project: accounts

services:
  accounts:
    path: .
    command: make dev-docker
    stop: make dev-docker-stop
    health: /api/health
    hostname: accounts
    proxy:
      target: hypixo-accounts        # container_name del compose.hotreload.yaml
      port: 4002
```

`make dev-docker` ejecuta:

```
docker compose -p hypixo-accounts-dev -f docker/compose.hotreload.yaml --env-file .env up -d --build
```

Pasos:

1. `raioz up`
2. Output incluye `accounts -> https://accounts.hypixo.dev` y `[ok] accounts (make)`. Justo después, `raioz status` muestra `accounts running pid:NNN` por unos segundos (el PID del `make` que aún corre el `--build`).
3. Apenas el `make` termina (exit 0, su trabajo era lanzar `compose up -d`), el PID muere.
4. `raioz status` → `accounts stopped`.
5. `docker ps --filter name=hypixo-accounts` → `Up 2 minutes` ✅
6. `curl -k https://accounts.hypixo.dev/` → `HTTP/2 307 location: /en` ✅

El estado real: contenedor up, ruteo Caddy ok, app respondiendo. El estado reportado por raioz: stopped.

Diff con el `.raioz.state.json` que se queda escrito:

```json
{
  "project": "accounts",
  "workspace": "hypixo",
  "lastUp": "2026-04-21T20:03:53-05:00",
  "hostPIDs": { "accounts": 1327746 },   // ← PID muerto, era el make
  "networkName": "hypixo-net"
}
```

## Síntoma

Doble UX-bug derivado:

1. **Falso negativo en status** — el usuario ve "stopped" y asume que `raioz up` falló. En mi caso me llevó a reportar "no se levantó el contenedor de accounts" cuando estaba 100% arriba.
2. **El URL `accounts -> https://accounts.hypixo.dev` se imprime una sola vez al final del up** y queda enterrado en el bloque de tip de `/etc/hosts`. Si el usuario después corre `raioz status` para "ver el URL otra vez", no aparece (porque el servicio figura stopped). No hay un `raioz urls` ni equivalente.

## Root cause

En `internal/app/status_host.go:60-82` la prioridad 1 es chequear docker compose status sobre `composePathToCheck`, calculado por `DetectComposePath` en `internal/host/compose.go:15`.

`DetectComposePath` con `command="make dev-docker"` y servicePath=`ui/accounts/`:
- No es `docker-compose` ni `docker compose` en el comando → no extrae `-f`.
- Busca compose files estándar en el root del servicio → encuentra `compose.yaml` (que existe en `accounts/` pero **no es el que usa `make dev-docker`** — ése es `docker/compose.hotreload.yaml`, con project name `hypixo-accounts-dev`).
- `GetServicesStatusWithContext("compose.yaml")` consulta el compose project derivado de ese path (`accounts` por default), encuentra 0 servicios running, retorna `[]`.
- `status_host.go:78-80` → marca `info.Status = "stopped"`.

Prioridad 2 (health command) sólo aplica si `svc.Commands.Health` está definido como **comando local**, no como endpoint HTTP. El `health: /api/health` del YAML va a `service.HealthEndpoint`, que esta función no consulta.

Prioridad 3 (PID) sólo "rescata" a "running" si el PID sigue vivo. El PID del `make` murió a los segundos. Game over.

**El núcleo del bug**: cuando un service host declara `proxy.target: <container_name>`, raioz **ya sabe** cuál es el contenedor que sirve ese servicio. Pero `getHostServiceInfo` no lo usa como fuente de verdad. Termina haciendo arqueología (compose autodetect + PID alive) cuando tiene la respuesta servida.

Archivos relevantes:

- `internal/app/status_host.go:60-149` — lógica de status para host services.
- `internal/host/compose.go:15` — `DetectComposePath` no maneja `-p <project>` (sólo `-f <file>`), y los nombres no estándar de compose project pasan desapercibidos.
- `internal/config/yaml_bridge.go:169-174` — `proxy.target` se persiste en `ServiceProxyOverride`, listo para ser consultado.
- `internal/docker/inspect.go:89` — ya existe el helper de inspección por nombre que retornaría `State.Status`.

## Fix propuesto

### Opción A — usar `proxy.target` como fuente de verdad para status (recomendada)

En `getHostServiceInfo`, agregar **prioridad 0** antes de las demás:

```diff
// internal/app/status_host.go, justo después de inicializar info{}
+ // Priority 0: if service declares proxy.target, that container is the source of truth
+ if svc.ProxyOverride != nil && svc.ProxyOverride.Target != "" {
+     containerStatus, err := uc.deps.DockerRunner.InspectContainerStatus(ctx, svc.ProxyOverride.Target)
+     if err == nil && containerStatus != "" {
+         switch containerStatus {
+         case "running":
+             info.Status = "running"
+             info.Health = "unknown" // refinar con HealthEndpoint en una pasada posterior
+         case "exited", "dead", "removing":
+             info.Status = "stopped"
+         }
+         // si encontramos algo definitivo, devolvemos sin caer a las demás prioridades
+         if info.Status != "" {
+             return info
+         }
+     }
+ }
```

Pro: 
- Cero ambigüedad: si vos mismo declaraste qué container te respalda, eso manda.
- Cero acoples al runtime usado (make/bash/python/whatever) — el container es el contrato.
- Habilita que `raioz status` muestre el URL aunque el launcher haya muerto.

Contra: 
- Requiere que `svc.ProxyOverride` esté propagado hasta `getHostServiceInfo` (hoy llega `svc config.Service` — verificar el schema interno post-bridge).
- Si el container fue creado por otra herramienta con el mismo nombre, raioz lo "adopta" silenciosamente. Mitigable con label `raioz.project=<project>` (alineado con la propuesta del issue 009).

### Opción B — soportar `commands.composePath` + `commands.composeProject` en YAML mínimo

Exponer en `raioz.yaml` (formato minimal) los campos que ya existen internamente:

```yaml
services:
  accounts:
    command: make dev-docker
    composePath: docker/compose.hotreload.yaml
    composeProject: hypixo-accounts-dev
```

Pro: arregla la autodetección sin tocar la lógica de status.

Contra:
- Le pasa al usuario la responsabilidad de duplicar lo que ya está en su Makefile. Anti-DRY.
- No resuelve el caso "el launcher arranca varios containers y sólo uno me importa".
- A es estrictamente más general.

### Opción C — supervisor que mantiene viva la asociación service→container

Combinar A con un goroutine post-`up` que registra en `.raioz.state.json`:

```json
{
  "containerByService": { "accounts": "hypixo-accounts" }
}
```

Esto sobrevive a `raioz status` desde otra terminal sin tener que re-leer el config.

Pro: rápido (sin docker inspect en cada status), funciona offline si docker está caído.
Contra: doble fuente de verdad. Salvable si A es la fuente primaria y el state es sólo cache (no usado para decisiones).

## Recomendación

**A**. El `proxy.target` ya está en el config, ya está propagado, ya hay `docker inspect` disponible. Es 20 líneas de código y arregla el bug de raíz.

Como follow-up: agregar un `raioz urls` que liste `<service> -> <url>` consultando proxy + status, para que el usuario pueda recuperar el URL cuando se le perdió en el scroll del `up`.

## Tradeoffs

- A asume que `proxy.target` representa "el" container del service. Si en el futuro un service host es un compose multi-container con varios `target`, hay que extender a `target: [...]` o caer al modo actual cuando no haya un único candidato.
- A no detecta si el contenedor está running pero la app dentro está crasheada — ese es trabajo del `HealthEndpoint`, que merece su propio paso (no parte de este fix).

## Prioridad

**Media**. No bloquea workflows (la app funciona) pero:
1. Mina la confianza en `raioz status` igual que el issue 009.
2. Hace que el usuario reabra `raioz up` "por las dudas" — perdiendo tiempo y ensuciando logs.
3. La info ya está disponible, sólo no se está usando — el costo del fix es bajo y el ROI alto.

## Contexto de descubrimiento

Descubierto el 2026-04-21 en `hypixo/ui/accounts`. Tras un `raioz up` exitoso, reporté a Claude "no veo el contenedor de accounts ni el URL". La verificación mostró container running, Caddy enrutando, app respondiendo 307 — y `raioz status` diciendo `stopped`. Desde la perspectiva del usuario, raioz mintió dos veces: primero al no destacar el URL, después al reportar el servicio caído.

## Resolución

Implementada Opción A en `fix/up-detach-default`. En `internal/app/status_host.go::getHostServiceInfo`, antes de las prioridades 1–3 existentes, se inserta una **prioridad 0**:

- Si `svc.ProxyOverride.Target != ""`, se hace `DockerRunner.GetContainerStatusByName(target)`.
- Si el container está `running`, se reporta `running` y se retorna inmediatamente.
- Si está `exited|dead|removing|created|paused|restarting`, se reporta `stopped` y se retorna.
- Si no existe (`""`), se hace fall-through a la lógica anterior — esto preserva el comportamiento esperado antes del primer `up`, cuando `proxy.target` ya está declarado pero el launcher aún no lo ha creado.

La idea es respetar el "contrato" que el usuario ya escribió: si declaró `proxy.target`, ese container ES el servicio, y la heurística de PID/compose se vuelve irrelevante.

Tests añadidos en `internal/app/status_extra_test.go`: target running wins sobre PID-alive heuristic, target exited wins sobre PID-alive (anti-falso-positivo), target ausente cae al fallback existente.

Pendiente como follow-up (no incluido aquí):
- `raioz urls` para listar `<service> -> <url>` sin tener que recordar el output de `up`.
- Soporte multi-target (`target: [...]`) si aparecen launchers con varios containers que importen.
