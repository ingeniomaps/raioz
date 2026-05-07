---
name: 014 — `raioz status <service>` acepta el arg pero lo ignora
description: El CLI no rechaza args posicionales y el usecase no los lee. Resultado: `raioz status front` muestra todo el proyecto sin filtrar, sin warning. Mismo patrón que 012 pero sin daño destructivo — sólo UX engañosa.
type: project
---

# 014 — `raioz status <service>` acepta el arg pero lo ignora

## Resumen

`raioz status` no soporta filtrado por servicio, pero acepta el arg posicional sin error. El comando imprime el reporte completo. El usuario tiene la impresión de que pidió un check focalizado y recibió todo el ruido del proyecto entero.

A diferencia de 012, esto es sólo una molestia de UX (no destructivo). Pero es el mismo patrón "el CLI promete un feature que el usecase no implementa".

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
EOF
cd /tmp/repro
raioz up
raioz status front
```

## Síntoma

```
$ raioz status front
  MULTI

  Services (2)
    front              make       running    pid:1598189
    back               make       running    pid:1598185
```

`back` aparece a pesar de que pedí sólo `front`. Sin warning de "status no acepta args".

## Root cause

1. **`internal/cli/status.go:21`** — `RunE: func(cmd *cobra.Command, args []string)` recibe `args` y nunca los lee. No hay `cmd.Args = cobra.NoArgs`.
2. **`internal/app/status.go:18-22`** — `StatusOptions` sólo tiene `ProjectName`, `ConfigPath`, `JSON`. Sin `Services []string`.
3. **`internal/app/yaml_commands.go::StatusYAML` y `internal/app/status_orchestrated.go::showOrchestratedStatus`** — ambos paths iteran TODOS los `Deps.Services` y `Deps.Infra`, sin filtro.

## Fix propuesto

### Opción A — soportar `raioz status [service...]`

```diff
// internal/app/status.go
 type StatusOptions struct {
     ProjectName string
     ConfigPath  string
     JSON        bool
+    // Services restricts the report to a subset. Empty means "everything"
+    // (legacy behavior).
+    Services    []string
 }
```

```diff
// internal/cli/status.go
 var statusCmd = &cobra.Command{
-    Use:   "status",
+    Use:   "status [service...]",
+    Args:  cobra.ArbitraryArgs,
     ...
     RunE: func(cmd *cobra.Command, args []string) error {
         ...
         return statusUseCase.Execute(ctx, app.StatusOptions{
+            Services:    args,
             ...
         })
     },
 }
```

Y en los dos imprintors (`StatusYAML`, `showOrchestratedStatus`), filtrar el iterador:

```diff
-for name, svc := range proj.Deps.Services {
+for name, svc := range proj.Deps.Services {
+    if !shouldShow(name, opts.Services) { continue }
     ...
```

`shouldShow` es trivial: si `len(filter)==0` retorna true; si no, busca `name` en `filter`.

### Opción B — rechazar args (parche corto)

```diff
 var statusCmd = &cobra.Command{
     Use:   "status",
+    Args:  cobra.NoArgs,
```

Cobra dirá `Error: unknown command "front" for "raioz status"`. Resuelve la trampa silenciosa pero no la falta de feature.

## Recomendación

**A**. A diferencia de 012, status es READ-ONLY: implementar el filtro es trivial, sin riesgo de destruir state. Mejor cerrar el feature bien que dejar el reject como deuda.

## Tradeoffs

- A requiere decidir qué pasa con los headers (`SERVICES`, `DEPENDENCIES`): si el usuario filtra a un solo servicio, ¿se muestra el header igual? Recomendado: sí — el output sigue siendo lecturable como reporte parcial.
- A no resuelve el output JSON: hay que filtrar el map de `servicesInfo` antes de serializar.

## Prioridad

**Baja-media**. No es destructivo. Frecuencia: alta en proyectos grandes (status completo es mucho ruido cuando uno está debuggeando un servicio puntual). Vale como QoL.

## Contexto de descubrimiento

Smoke tests post-merge de issues 007–011 (2026-05-06). Bug preexistente.

## Relacionados

- **012** — `raioz down <service>` ignora args (destructivo silencioso).
- **013** — `raioz restart <host-service>` falla con error de Docker.
