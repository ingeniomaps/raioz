---
name: 012 — `raioz down <service>` ignora el argumento y baja todo el proyecto
description: El CLI acepta nombres de servicio como args posicionales sin error, pero el caso de uso los descarta y ejecuta el down completo. UX-trap silencioso: el usuario cree que bajó solo `front` y se llevó también `back` y la dep.
type: project
---

# 012 — `raioz down <service>` ignora el argumento y baja todo el proyecto

## Resumen

`raioz down` no soporta selección por servicio, pero **acepta** args posicionales sin marcarlos como inválidos. El comando ejecuta el down completo silenciosamente. Esto es peor que dar error: el usuario tiene la falsa expectativa de que bajó un solo servicio y termina con el proyecto entero abajo (otros servicios + dependencies).

Encontrado mientras se validaban las issues 007–011 con smoke tests reales: al probar `raioz down front` en un proyecto con `back`, `front` y `redis`, raioz detuvo los tres sin advertencia.

## Reproducción

```bash
mkdir -p /tmp/repro/{back,front}
cat > /tmp/repro/raioz.yaml <<'EOF'
project: multi
services:
  back:
    path: ./back
    command: sleep 600
  front:
    path: ./front
    command: sleep 600
dependencies:
  redis:
    image: redis:7-alpine
    publish: 6390
EOF

cd /tmp/repro
raioz up                # back, front, redis arriba
raioz down front        # ← intento "solo front"
raioz status            # → todo stopped, no solo front
```

## Síntoma

```
$ raioz down front
[--] Stopping project multi...
[ok] Project 'multi' stopped

$ raioz status
# back, front y redis están todos stopped
```

Sin warning, sin "did you mean --all?", sin "down does not accept service names".

## Root cause

Tres puntos coordinados:

1. **`internal/cli/down.go:25`** — `RunE: func(cmd *cobra.Command, args []string)` recibe `args` pero **nunca los lee**. No hay `cmd.Args = cobra.NoArgs` que los rechazara, ni se pasan al usecase.
2. **`internal/app/down.go:15-28`** — `DownOptions` no tiene un campo `Services []string`. La forma del struct sólo conoce flags globales (`All`, `PruneShared`, `Conflicting`, `AllProjects`).
3. **`internal/app/down.go:43`** — `Execute` opera siempre sobre el proyecto completo. No hay path "down de un subset".

## Fix propuesto

Decidir entre dos opciones — **B** es más conservadora; **A** es la que cierra la brecha contra el modelo mental "raioz orquesta servicios".

### Opción A — soportar `raioz down <service>...`

```diff
// internal/app/down.go
 type DownOptions struct {
     ProjectName string
     ConfigPath  string
+    // Services restricts the down to a subset. Empty means "whole project"
+    // (legacy behavior). When set, only these services + their deps that
+    // become unused are stopped.
+    Services    []string
     All         bool
     PruneShared bool
     ...
 }
```

```diff
// internal/cli/down.go
 var downCmd = &cobra.Command{
-    Use:   "down",
+    Use:   "down [service...]",
+    Args:  cobra.ArbitraryArgs,
     Short: "Bring down project dependencies",
     ...
     RunE: func(cmd *cobra.Command, args []string) (err error) {
         ...
         downErr := downUseCase.Execute(ctx, app.DownOptions{
+            Services:    args,
             ProjectName: projectName,
             ...
         })
```

Y en `Execute`, ramificar al inicio: si `len(opts.Services) > 0`, parar sólo ese subset (host PIDs + Docker compose `stop <svc>` selectivo). El path completo queda intacto cuando `Services` está vacío.

### Opción B — rechazar args explícitamente (parche corto)

```diff
 var downCmd = &cobra.Command{
     Use:   "down",
+    Args:  cobra.NoArgs,
```

`cobra` falla con `unknown command "front"` (o similar) y el usuario entiende que la sintaxis no existe. **Pro**: 30 segundos, elimina la trampa. **Contra**: no resuelve el use case real ("apagar solo el frontend").

## Recomendación

**B ahora** + abrir un sub-issue para **A** como work cuando haya tiempo. La trampa silenciosa es el riesgo grave, no la ausencia del feature.

## Tradeoffs

- A requiere pensar dependencias: si el usuario pide `down redis` pero `back` aún la usa, ¿warn + skip, o forzar y dejar `back` en estado inconsistente? El path completo no enfrenta ese caso porque tira todo.
- B rompe scripts (poco probables) que hoy invocan `raioz down algo` y dependen del comportamiento actual. Riesgo bajo: hoy ese comportamiento es justamente el bug.

## Prioridad

**Media-alta**. La pérdida silenciosa de estado (volúmenes intactos, pero servicios derribados sin querer) es el peor tipo de UX-bug — el usuario no se entera hasta que le toca volver a arrancar todo. Frecuencia: probable en uso multi-servicio (que es la mayoría de los proyectos reales).

## Contexto de descubrimiento

Durante los smoke tests post-merge de issues 007–011 (sesión 2026-05-06), al validar manualmente que las correcciones funcionaran end-to-end. El issue es preexistente — no introducido por esos cambios.

## Relacionados

- **013** — `raioz restart <host-service>` falla con error de Docker.
- **014** — `raioz status <service>` ignora el argumento.

Los tres comparten el patrón "el CLI acepta args que el usecase desconoce". Vale considerarlos juntos cuando se aborde el rediseño de selección por servicio en comandos no-up.
