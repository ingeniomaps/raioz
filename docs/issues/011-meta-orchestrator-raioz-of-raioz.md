---
name: 011 — meta-orchestrator (raioz de raioz) para arrancar N proyectos del mismo workspace con un solo comando
description: Workspaces con varios proyectos raioz independientes (cada uno con su raioz.yaml) hoy requieren script externo (Makefile/bash) para "levantar todo". Propuesta de un raioz.yaml en modo meta que delegue el up/down/status a sub-proyectos en orden, sin duplicar su config.
type: project
---

# 011 — meta-orchestrator (raioz de raioz) para arrancar N proyectos del mismo workspace con un solo comando

## Resumen

Cuando un workspace tiene **N proyectos raioz independientes** (cada uno con su propio `raioz.yaml`, repo git separado o subdir distinto), no hay forma nativa de orquestarlos con un solo comando. Las opciones actuales son:

1. **`cd` por servicio** y correr `raioz up` en cada uno → tedioso, frágil al orden, no scripteable de forma idiomática.
2. **Duplicar todos los `services:` y `dependencies:` en un `raioz.yaml` top-level** → drift garantizado contra los `raioz.yaml` per-servicio, pierde la capacidad de levantar un servicio aislado, fights el modelo "cada servicio es repo independiente".
3. **Wrapper externo (Makefile, bash, justfile)** → funciona pero deja a raioz fuera del flujo: `raioz status` desde la raíz no sabe qué proyectos están arriba; el `.raioz.state.json` se fragmenta entre subdirs sin vista unificada.

Falta un modo nativo en raioz que ya tiene el concepto de `workspace` para que **un `raioz.yaml` declare ser un meta-orchestrator y delegue a sub-proyectos por path**.

## Caso de uso (hypixo)

Workspace `hypixo` con 5 proyectos raioz, cada uno con su `raioz.yaml`:

```
hypixo/
├── keycloak/raioz.yaml          # keycloak + infra (postgres, redis, pgadmin, redisinsight, mailpit)
├── ai-strategist-service/raioz.yaml  # cerebro Go — repo git propio
├── ui/portal-app/raioz.yaml     # SPA + BFF
├── ui/accounts/raioz.yaml       # auth/onboarding
└── ai-ad-service/raioz.yaml     # SSE provider (futuro tras migrar de .raioz.json)
```

Todos comparten `workspace: hypixo` → mismo `hypixo-net` → DNS interno (`sso.hypixo.dev`, `app.hypixo.dev`, etc.).

Orden de arranque: keycloak primero (trae infra + SSO del workspace) → strategist (consume postgres/redis del cluster compartido) → frontends (consumen SSO + strategist).

Hoy el founder hace `cd keycloak && raioz up`, luego `cd ../ai-strategist-service && raioz up`, etc. Eso es 5 cambios de directorio + 5 invocaciones manuales en orden, sin status unificado al final.

## Por qué importa

1. **Onboarding**. Un nuevo dev clona el workspace y la primera fricción es entender el orden correcto. Un `raioz up` desde la raíz que sabe qué hacer es UX clave.
2. **Status unificado**. `raioz status` desde la raíz hoy no muestra nada (no hay `raioz.yaml` ahí). El usuario tiene que recordar qué proyectos hay, ir a cada uno, y agregar mentalmente el estado.
3. **Workspace lifecycle real**. Conceptualmente el workspace ES la unidad: una red docker, un dominio (`hypixo.dev`), un proxy Caddy. Que la herramienta no tenga un primitivo para "levantar el workspace" obliga a reinventar la rueda con Makefiles externos.
4. **Drift vs duplicación**. Sin meta-orchestrator, la única alternativa "todo-en-uno" es duplicar config — lo cual rompe el modelo de "una source of truth por servicio".

## Propuesta

Agregar un modo `meta` (o `kind: meta`) al schema de `raioz.yaml`. Un raioz.yaml meta NO declara `services:` ni `dependencies:` — solo lista paths de sub-proyectos y orquesta `up`/`down`/`status` sobre ellos.

### Schema propuesto

```yaml
# /home/manuel/Code/hypixo/raioz.yaml (top-level, meta-orchestrator)
workspace: hypixo
kind: meta             # nuevo discriminador

projects:
  - path: keycloak
  - path: ai-strategist-service
  - path: ui/portal-app
  - path: ui/accounts
  - path: ai-ad-service
    optional: true     # no falla el up si este sub-proyecto falla

# Orden explícito para up; down va en orden inverso por default.
startOrder:
  - keycloak           # primero (trae infra: postgres, redis, sso)
  - ai-strategist-service
  - ui/portal-app
  - ui/accounts
  - ai-ad-service
```

Cada sub-proyecto referenciado debe tener su propio `raioz.yaml` con el mismo `workspace: hypixo` (raioz lo valida al cargar el meta).

### Comportamiento esperado

- **`raioz up`** desde el dir del meta:
  - Lockea el workspace (issue 007 en mente — un solo "up" del meta toma el lock global).
  - Itera `startOrder` (o el orden de `projects:` si no se declara `startOrder`), ejecuta `raioz up` en cada sub-path en serie.
  - Sub-proyectos `optional: true` no abortan en error (warn + continue).
  - Al final, imprime un resumen consolidado: cada sub-proyecto + sus URLs + status.
- **`raioz down`** desde el dir del meta:
  - Itera en orden inverso al `startOrder`. Si un sub falla, sigue (down debe ser tolerante).
- **`raioz status`** desde el dir del meta:
  - Para cada sub-proyecto, ejecuta `raioz status` en su path y agrega la salida.
  - Resumen final: N/M servicios running.
- **`raioz logs <project>/<service>`** (nice-to-have):
  - Permite hacer drill-down sin cambiar de directorio.
- **`.raioz.state.json` del meta**: solo guarda el path lock + los sub-proyectos invocados; el state real sigue viviendo en cada sub-dir.

### Lo que NO hace el meta

- NO redefine `services:` o `dependencies:` (eso es responsabilidad del sub-proyecto).
- NO mergea environments (cada sub mantiene sus `.env`).
- NO toca `pre`/`post` de los subs — los respeta tal cual.
- NO impone proxy: solo el sub que define `proxy: true` o `proxy:` tiene Caddy.

## Tradeoffs

- **Complejidad de implementación**: medio. La mayoría es shell-out a `raioz up/down/status` en otros paths con CWD ajustado, agregando salida. La parte delicada es el manejo del workspace lock — debe coordinarse con el lock global existente para no auto-bloquearse.
- **Vs. `extends`/`imports`**: un mecanismo alternativo sería que un raioz.yaml top-level "incluya" el contenido de los sub-proyectos via `imports: [keycloak/raioz.yaml, ...]`. Más potente pero abre una caja de Pandora (resolución de paths relativos, conflictos de nombres, semántica de override). El modo `meta` es deliberadamente menos expresivo: solo orquesta, no compone.
- **Vs. `make`/`just` externos**: el wrapper externo funciona hoy y es trivial. La justificación del primitivo nativo es que workspaces multi-proyecto son **el caso esperado** del modelo workspace de raioz; sin meta, el modelo está incompleto desde la perspectiva de UX.
- **Vs. seguir haciendo nada**: aceptable a corto plazo. El cliente actual (hypixo) puede vivir con un `Makefile` top-level. Pero a medida que el workspace crezca (más servicios, más onboarding), la fricción aumenta y la propuesta vale más.

## Prioridad

**Media-baja**. No bloquea ningún cliente — el workaround del Makefile externo es de 10 líneas. Pero es la pieza que cierra el modelo `workspace` conceptualmente y mejora UX para workspaces grandes (3+ proyectos).

Sugerido roadmap:

1. v1 mínima: `kind: meta` con `projects:` + `startOrder:` + delegación serial de `up/down/status`. Sin `optional`, sin `logs`. Suficiente para hypixo.
2. v1.1: `optional: true`, `--continue` en up para no abortar al primer fail.
3. v2: `raioz logs <project>/<service>`, `raioz exec <project>/<service> ...` desde el meta.

## Cliente que lo pidió

Hypixo (workspace `hypixo`, 5 proyectos raioz). Reportado 2026-05-06 al revisar la opción de un `raioz.yaml` top-level y descartarla por drift; el founder pidió documentar la limitación como issue de raioz antes de aplicar el workaround del Makefile.

## Resolución

Implementada la **v1 mínima** en `fix/up-detach-default`:

1. Schema (`internal/config/yaml_types.go`): `RaiozConfig` ahora soporta `kind: meta`, `projects: [{path, optional}]`, `startOrder: []`. Backward-compatible — un raioz.yaml normal sigue funcionando exactamente igual.
2. Loader específico (`internal/config/yaml_meta.go`): `LoadMetaConfig(path) (*MetaConfig, isMeta bool, err)`. Resuelve paths a absolutos relativo al meta yaml, valida `projects` no-vacío, valida que las entradas de `startOrder` matcheen `projects[*].path`. Cuando el archivo es un proyecto regular retorna `(nil, false, nil)` y deja al loader normal seguir.
3. Runner (`internal/app/meta.go`): `MetaRunner` shellea al binario actual (`os.Args[0]`) por sub-proyecto con `cmd.Dir = sub.Path`. Decisión explícita: cada sub-proyecto corre en su propio proceso para aislar global state (i18n, naming prefix, locks) — no se mezcla con el meta. `Up` aborta al primer fallo no-opcional; `Down` y `Status` son tolerantes y siguen.
4. Dispatcher CLI (`internal/cli/meta_dispatch.go`): `tryHandleMeta(ctx, path, subCmd)` se invoca al inicio de up/down/status. Si es meta, ejecuta y retorna `handled=true`. Si no, fall-through.

Tests:
- `internal/config/yaml_meta_test.go`: 6 casos (regular vs meta, happy path, startOrder, errores).
- `internal/app/meta_test.go`: 6 casos (up, down reverso, down/status tolerantes, optional vs required, abortar al primer fail).
- `internal/cli/meta_dispatch_test.go`: 5 casos (fall-through, auto-detect marker, dispatch e2e con binario falso, errores de loader).

Pendiente para v1.1+ (no incluido):
- Flag `--continue` para que `up` no aborte ante un sub no-opcional.
- `raioz logs <project>/<service>` y `raioz exec <project>/<service> ...` desde el meta.
- Lock workspace-wide explícito coordinado entre meta y subs (hoy no es necesario gracias al detach-default del issue 007: cada sub libera su lock antes de continuar).
