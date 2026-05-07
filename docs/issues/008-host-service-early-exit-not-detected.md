# 008 — raioz no detecta muertes tempranas de servicios host

## Resumen

Tras `cmd.Start()` raioz registra el PID y continúa sin esperar. Si el proceso hijo muere inmediatamente (ej: `bind: address already in use`, crash de config, binario faltante), `raioz status` reporta el servicio como **running** mientras `raioz logs <svc>` muestra claramente `exit status 1`. El usuario se da cuenta solo al ver logs o al intentar consumir el servicio.

## Reproducción

En `gouduet/api`:

1. Otro proceso ocupa el puerto 8080 (ej: un `go run ./cmd/api` huérfano de una sesión anterior).
2. `raioz up`
3. Summary imprime "Todos los servicios levantados".
4. `raioz status` → `api` aparece running.
5. `raioz logs api` → muestra `listen tcp :8080: bind: address already in use` + `exit status 1`.
6. `curl http://localhost:8080/v1/health` → fail.

## Síntoma

```
$ raioz status
Service         Status     Health
api             running    -          ← mentira

$ raioz logs api
{"level":"INFO","msg":"api.listening","addr":":8080"}
{"level":"ERROR","msg":"api.listen_failed","err":"listen tcp :8080: bind: address already in use"}
exit status 1
```

## Root cause probable

- `internal/host/process.go:197` — `cmd.Start()` lanza y no hace `Wait()`.
- `internal/host/process.go:204-210` — `ProcessInfo` captura PID pero no el canal de exit.
- `internal/app/upcase/usecase.go` — el flow de bring-up asume éxito si `Start()` no devuelve error, sin confirmar que el proceso sigue vivo.

`cmd.Start()` sólo falla si no puede ejecutar el binario (ej: exec: no such file). Un proceso que arranca y muere 10ms después es "éxito" desde la perspectiva de `Start()`.

## Fix propuesto

### Opción A — verificación breve post-start (conservadora)

```diff
// internal/host/process.go, después de cmd.Start()
+ // Espera corta: si el proceso muere en <settleTime> lo marcamos como error.
+ const settleTime = 500 * time.Millisecond
+ done := make(chan error, 1)
+ go func() { done <- cmd.Wait() }()
+ select {
+ case err := <-done:
+     return nil, fmt.Errorf("service %q exited immediately: %w", name, err)
+ case <-time.After(settleTime):
+     // El proceso sigue vivo — registrar PID y continuar.
+ }
  info := &ProcessInfo{
      Name: name,
      PID:  cmd.Process.Pid,
+     // Mantener el canal para que status/logs puedan reflejar muerte posterior.
+     waitDone: done,
  }
```

Pro: simple, cubre el 90% de crashes tempranos (puerto ocupado, binario roto).
Contra: 500ms de latencia extra por servicio. No detecta crashes tardíos (ej: 3s después del start).

### Opción B — supervisor persistente (correcta)

Lanzar un goroutine de supervisión por proceso que `cmd.Wait()`-ea en background y actualiza el state file con el exit status. `raioz status` lee el state en vez de chequear PID.

```diff
+ // En StartService:
+ go func() {
+     err := cmd.Wait()
+     stateManager.MarkServiceExited(name, err)
+ }()
```

Pro: refleja siempre el estado real. Permite restart automático opcional. Base para `raioz up --watch` auto-restart y para alertas de servicios caídos.
Contra: más código; hay que pensar en race conditions del state file (ya existe este problema, no se agrava).

## Recomendación

**A como fix corto** (mitiga el 90% del dolor hoy). **B como trabajo derivado** para un sprint dedicado a observabilidad de servicios host.

## Tradeoffs

- Opción A deja servicios que mueren tarde (segundos después) sin detectar. Para el MVP está OK; para prod no.
- Opción B requiere que `status` y `logs` lean del state file (ya lo hacen parcialmente); hay que unificar.

## Prioridad

**Media**. No bloquea workflows pero genera confusión y tiempo perdido — hoy me costó 20 minutos diagnosticar que un puerto ocupado era la causa. En un equipo mayor, este patrón produce tickets del tipo "raioz dice que está arriba pero mi API no responde" repetidos.

## Contexto de descubrimiento

Descubierto en `gouduet/api` el 2026-04-17. Sesión de integración keycloak → app → api. Un `gouduet-api` leftover de un test manual ocupaba 8080; `raioz up` pasó el bring-up y siguió reportando "running".

## Resolución

Implementada Opción A en `fix/up-detach-default`:

1. `internal/host/process.go`: nueva `startSettleWindow` (default 500 ms). Tras `cmd.Start()` esperamos esa ventana en un `select`; si `cmd.Wait()` retorna primero, devolvemos error con la cola de stderr embebida.
2. `internal/host/process_helpers.go`: `formatEarlyExitError` + `readLogTail` para producir un error legible (`service "x" exited within 500ms: exit status 1\n--- stderr tail ---\n...`).
3. `internal/host/process.go` (StopServiceWithCommandAndPath): tolera `"no child processes"` en `process.Wait()` porque el goroutine de la settle window ya pudo reapearlo — antes esto se reportaba como error de stop espurio.

Tests añadidos en `process_test.go`: early-exit detectado, stderr-tail incluido, sobreviviente no marcado como error. Helpers cubiertos en `process_helpers_test.go`.

Pendiente para una iteración futura (Opción B): supervisor goroutine post-settle que actualice estado en `.raioz.state.json` para detectar muertes tardías; hoy `raioz status` sigue creyendo en el PID y eso cubre crashes después de la ventana solo si el PID muere de verdad.
