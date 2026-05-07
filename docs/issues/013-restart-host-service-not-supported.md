---
name: 013 — `raioz restart <host-service>` falla con "No such container"
description: El use case de restart asume Docker compose y delega a `DockerRunner.RestartServicesWithContext`. Para servicios host (npm, make, go, python, ...) no hay container con ese nombre y falla con un error de Docker engañoso.
type: project
---

# 013 — `raioz restart <host-service>` falla con "No such container"

## Resumen

`raioz restart <name>` siempre invoca docker compose contra el `composePath` derivado del state. Para servicios host (que no viven como containers), no existe `raioz-<project>-<name>` y Docker responde `No such container`. El usuario ve un error que no aplica a su servicio: la app está corriendo como `node`/`go`/`make`, no como container.

El fallo es ruidoso (a diferencia de 012) — el usuario al menos sabe que algo anda mal — pero el mensaje confunde.

## Reproducción

```bash
mkdir -p /tmp/repro
cat > /tmp/repro/raioz.yaml <<'EOF'
project: multi
services:
  front:
    path: .
    command: sleep 600   # host-only
EOF
cd /tmp/repro
raioz up
raioz restart front
```

## Síntoma

```
$ raioz restart front
[--] Restarting front...
[fail] front: Error response from daemon: No such container: raioz-multi-front
```

`raioz-multi-front` nunca existió: el servicio corre como PID host, no como container.

## Root cause

`internal/app/restart.go:145-167` — `doRestart` siempre llama `DockerRunner.RestartServicesWithContext(ctx, composePath, services)` (o `ForceRecreateServicesWithContext`). No hay branch para servicios host:

```go
func (uc *RestartUseCase) doRestart(
    ctx context.Context, w io.Writer, composePath string,
    services []string, forceRecreate bool,
) error {
    ...
    if forceRecreate {
        err = uc.deps.DockerRunner.ForceRecreateServicesWithContext(ctx, composePath, services)
    } else {
        err = uc.deps.DockerRunner.RestartServicesWithContext(ctx, composePath, services)
    }
    ...
}
```

`resolveRestartServices` arma la lista, pero NO clasifica entre Docker y host. El loop downstream va contra docker compose como si todos lo fueran.

## Fix propuesto

Clasificar cada servicio en host vs Docker antes de delegar:

```diff
 func (uc *RestartUseCase) doRestart(
     ctx context.Context, w io.Writer, composePath string,
-    services []string, forceRecreate bool,
+    services []string, forceRecreate bool,
+    deps *config.Deps, ws *interfaces.Workspace, projectDir string,
 ) error {
+    // Split into host vs Docker: host services need StopServiceWithCommand
+    // + StartService (re-spawn), Docker services delegate to compose.
+    var hostNames, dockerNames []string
+    for _, n := range services {
+        if isHostService(deps.Services[n]) {
+            hostNames = append(hostNames, n)
+        } else {
+            dockerNames = append(dockerNames, n)
+        }
+    }
+
+    for _, n := range hostNames {
+        if err := uc.restartHost(ctx, ws, deps, n, projectDir); err != nil {
+            return err
+        }
+    }
+
+    if len(dockerNames) == 0 {
+        return nil
+    }
+
+    // existing docker compose path:
     if forceRecreate {
-        err = uc.deps.DockerRunner.ForceRecreateServicesWithContext(ctx, composePath, services)
+        err = uc.deps.DockerRunner.ForceRecreateServicesWithContext(ctx, composePath, dockerNames)
     } else {
-        err = uc.deps.DockerRunner.RestartServicesWithContext(ctx, composePath, services)
+        err = uc.deps.DockerRunner.RestartServicesWithContext(ctx, composePath, dockerNames)
     }
     ...
 }
```

`restartHost` se compone de la pipeline existente:

1. Buscar el PID en `host.LoadProcessesState(ws)` o en `.raioz.state.json::HostPIDs`.
2. `host.StopServiceWithCommandAndPath(ctx, pid, svc.Commands.Down, servicePath)`.
3. `host.StartService(ctx, ws, deps, name, svc, projectDir)` — esto reusa la lógica del up, incluida la settle window (issue 008).

## Tradeoffs

- Llamar a `StartService` en restart introduce un ciclo de imports si el path está mal organizado. La fix limpia es delegar a un helper compartido en `app/upcase` o `internal/host`.
- `--force-recreate` no aplica a host services (no hay container para recrear). Decidir: ignorar el flag para hosts o devolver error claro. Recomendado: ignorar + warning.
- Restart de un launcher con `command: make dev-docker` (issue 010) tiene que respetar el `stop:` declarado, no SIGTERMear el PID — el PID ya estaría muerto. Esto se cae solo si reusamos `StopServiceWithCommandAndPath` (que ya tiene la lógica).

## Prioridad

**Media**. No bloquea workflows (workaround: `raioz down` + `raioz up`), pero rompe el modelo mental: el usuario espera "raioz orquesta mis servicios sin importar el runtime", y en restart aparece la abstracción rota.

Encaja con el rediseño general de "comandos por servicio" listado en 012/014.

## Contexto de descubrimiento

Smoke tests post-merge de issues 007–011 (2026-05-06). Bug preexistente.

## Relacionados

- **012** — `raioz down <service>` ignora args.
- **014** — `raioz status <service>` ignora args.
