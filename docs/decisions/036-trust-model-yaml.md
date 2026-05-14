# ADR-036: yaml hygiene policy + política "secretos nunca en `raioz.yaml`"

- **Status:** Accepted — 2026-05-14
- **Date:** 2026-05-14
- **Related issue:** docs/issues/066-yaml-hygiene-checks.md

## Context

Raioz consume un `raioz.yaml` y a partir de ahí ejecuta acciones
con side-effects (clone, pull, exec). Surge la pregunta: ¿cuánto
debe raioz validar el contenido del yaml antes de actuar?

La respuesta conservadora ("trust pass con 6 capas de defensa,
threat model T1-T10, sub-comando dedicado, hash persistence,
confirmación interactiva, heurística de scripts peligrosos") fue
considerada y **descartada** en discusión. Razones:

- El modelo de amenazas ("yaml malicioso recibido por Slack")
  no refleja el uso actual de raioz. Hoy los yamls viven en el
  repo del proyecto, los devs los escriben para su propio
  equipo, y los teammates clonan el repo — no se pasan yamls
  por canales no auditados.
- Heurísticas de scripts peligrosos (`curl|sh`, `rm -rf`)
  tienen falsos positivos altos (`nvm`/`rustup` son legítimos)
  y atacantes reales obfuscan trivialmente. Patrón clásico de
  security theater que se ve bien en review y atrapa cero.
- Allowlist de hosts git no mitiga lo que dice mitigar:
  `github.com` siempre está en la lista, y un atacante usa
  `github.com/atacante/malware`. La protección real es revisar
  el repo, no el dominio.
- Sub-comando `raioz doctor --config` agrega superficie de
  schema (comando, flags, estado) sin valor distinguible de
  correr `preventive.go` en preflight.

La respuesta pragmática es **mucho más chica**: tres chequeos
concretos que sí previenen incidentes reales del patrón de uso
actual + una política estructural sobre secretos.

## Decision

### Hygiene checks (3 reglas, todas en `preventive.go`)

`internal/validate/preventive.go` se extiende con 3 chequeos.
**No se crea paquete nuevo, ni sub-comando nuevo, ni estado
nuevo en `LocalState`.**

**Regla H1 — Detección de secrets en el yaml bruto.**

Antes del `yaml.Unmarshal`, raioz escanea los bytes del archivo
contra una lista de regex de patrones de credenciales conocidos:

- GitHub PAT: `gh[ps]_[A-Za-z0-9]{36,}`, `gho_[A-Za-z0-9]{36,}`,
  `ghu_[A-Za-z0-9]{36,}`, `ghs_[A-Za-z0-9]{36,}`
- GitLab PAT: `glpat-[A-Za-z0-9_\-]{20,}`
- Slack: `xox[boa]-[A-Za-z0-9-]+`
- AWS access key: `AKIA[0-9A-Z]{16}`
- PEM private key: `-----BEGIN [A-Z ]*PRIVATE KEY-----`

Match → **error duro** antes de procesar el yaml. Mensaje:
"raioz.yaml contiene lo que parece un <name>. Los secretos
nunca van en el yaml — movelos a env vars o credential
manager."

**Regla H2 — Path traversal en referencias del yaml.**

Cualquier path referenciado en `services.<n>.path`,
`services.<n>.compose`, `dependencies.<n>.compose`,
`services.<n>.command` o `pre:`/`preUp:`/`post:` que:

- Use `..` para resolver fuera del project dir.
- Sea absoluto a paths fuera del repo (`/etc/`, `/root/`,
  `/var/lib/`, etc.).

Match → **error**. Mensaje accionable que muestra el path
resuelto y por qué fue rechazado.

**Regla H3 — Image tag pinning warning.**

`dependencies.<n>.image` sin tag explícito o con tag `:latest`
→ **warning** (no error). Mensaje sugiere pinear un tag
específico para reproducibilidad. No bloquea `raioz up`.

### Política transversal: secretos nunca en `raioz.yaml`

**Regla dura**: ningún campo de `raioz.yaml` contiene un token,
PAT, password, private key, o cualquier credencial. La regla H1
arriba implementa la detección.

**Sources legítimas de credenciales** (NO en el yaml):

| Tipo de credencial | Fuente legítima |
|---|---|
| Token de Git provider (PAT) | `gh auth` / `glab auth` / git credential helper / env var del dev |
| SSH key | ssh-agent / `~/.ssh/` |
| API tokens (Stripe, etc.) | `.env` files locales, env vars, secret managers (Vault, AWS SSO) |
| Cualquier otra credencial | Misma regla: env vars o secret manager |

**Lo que SÍ va en el yaml:**

- Un **selector** de proveedor (issue 067): `auth: gh`,
  `auth: ssh`, `auth: inherit`. Selecciona el mecanismo, no la
  credencial.
- **Referencias** a env vars del dev:
  `headers: ["Authorization: Bearer ${PAYMENTS_DEV_TOKEN}"]`.
  El **nombre** está en el yaml; el **valor** vive en el
  entorno.

**Por qué error y no warning para H1:**

Un token committed es un incidente caro (rotación de
credenciales, posible compromiso si el repo es público). Para
cuando se notaría como warning, el `git push` ya ocurrió. La
política tiene que rechazar el yaml antes de que sea consumible.

**Por qué no se puede excepcionar (sin `--accept-secret` ni
override):**

Un string que casualmente matchea `ghp_[A-Za-z0-9]{36}` por
colisión es virtualmente imposible. La política no contempla
override de ningún tipo.

### Explícitamente fuera de scope (won't do)

Las siguientes piezas se consideraron y descartaron. Documentar
acá evita re-proponerlas sin saber que ya fueron evaluadas.

- **URL classification (RFC1918 vs pública / loopback)**. No hay
  campos URL en el yaml hoy (issue 065 que los proponía fue
  abandonado — ver `docs/issues/ARCHIVED-065-remotes-vision.md`).
  Aún si en el futuro se agregaran, valor marginal — los devs
  escriben sus propios yamls, no consumen URLs ajenas.
- **Allowlist de hosts git**. `github.com` siempre estaría en
  la lista; el atacante usaría `github.com/atacante/malware`.
  No mitiga lo que dice mitigar.
- **Heurística de scripts peligrosos** (`curl|sh`, `rm -rf`,
  redirecciones a `~/.ssh/`). Falsos positivos altos en
  scripts legítimos (`nvm`, `rustup`, instaladores). Atacantes
  reales obfuscan. Security theater.
- **Confirmación interactiva first-run + hash del yaml en
  state**. Defensa contra "yaml de Slack desconocido" —
  escenario que no refleja el uso actual de raioz. Friction
  garantizada por valor cero; devs harán `--yes` siempre.
- **Sub-comando `raioz doctor --config <path>`**. Las
  validaciones útiles ya viven en `preventive.go` y corren en
  el preflight de `raioz up`. Un comando dedicado agrega
  superficie sin valor distinguible.
- **`--explain <finding-id>` con textos educativos extensos**.
  Bonito en demos, raramente leído.
- **`--accept-script` con hash whitelisting**. Existe solo para
  mitigar los falsos positivos de la heurística de scripts, que
  tampoco vale.
- **Firma criptográfica de yamls (sigstore, HMAC)**. Trabajo
  enorme (key management, revocation, rotación) para un threat
  model que raioz no tiene hoy.
- **Sandbox real de `pre:`/`preUp:`/`post:`** (container,
  chroot, seccomp). Portabilidad cara (macOS/Windows), valor
  especulativo.

**Reconsiderar si:**

- Aparece un incidente concreto que una de estas piezas habría
  prevenido (no hipotético — algo realmente reportado).
- Raioz se adopta a escala donde "share-the-yaml" pasa a ser
  flujo común y los yamls empiezan a circular fuera del repo
  del equipo.
- Un usuario reporta demanda específica para una pieza con un
  caso de uso real.

## Consequences

### Positive
- Defensa real contra una clase de incidente concreto (secretos
  committed), no teatro de seguridad.
- Cero superficie nueva: sin paquete `trust/`, sin sub-comando,
  sin campos en `LocalState`, sin flags nuevos. Las 3 reglas
  viven en un archivo existente.
- Implementación pequeña (~150 líneas + tests). Fácil revisar,
  fácil mantener.
- Política transversal sobre secretos es enforceable y
  documentada — issue 067 puede referenciarla.
- La sección "won't do" preserva la memoria institucional. En 6
  meses, cualquiera que proponga "agreguemos heurística de
  scripts" puede leer el contexto y la evaluación previa.

### Negative
- Si raioz pivota a "share-the-yaml" en serio, varias de las
  piezas abandonadas se vuelven relevantes. Documentado en
  "reconsiderar si" arriba.
- No detecta yamls maliciosos que pasan los 3 chequeos. La
  protección real para ese caso es review humano de yamls de
  terceros — fuera del scope de raioz.

### Neutral
- ADR-036 draft inicial de esta misma sesión tenía las 6 capas
  + threat model T1-T10. Este re-scoping es de la misma sesión
  post-discusión crítica; el ADR salió Accepted con el scope
  reducido, no como Superseded de una versión previa shipped.

## Alternatives considered

- **(A) Trust pass completo con 6 capas + threat model T1-T10**.
  Rechazado: defensa especulativa contra amenazas que raioz no
  tiene. El "won't do" arriba documenta cada capa con su razón
  específica.
- **(B) Cero chequeos — confianza total en el dev**. Rechazado:
  la regla H1 (secret detection) sí intercepta un incidente
  real y caro (PAT committed por accidente). Costo de la regla
  es trivial frente al costo del incidente.
- **(C) Solo regla H1 (secrets), sin H2 y H3**. Considerado.
  Rechazado porque H2 (path traversal) es chequeo
  determinístico de 40 líneas que previene typos reales, y H3
  (image pinning) es 30 líneas que mejora reproducibilidad sin
  bloquear nada. Costo marginal cero.

## References

- **Issue 066** (`docs/issues/066-yaml-hygiene-checks.md`):
  implementación de las 3 reglas.
- **Issue 067** (`docs/issues/067-git-auth-providers.md`):
  depende de la "política transversal — secretos nunca en yaml"
  para justificar que `auth:` selecciona proveedor, no carga
  credencial.
- **ADR-024** (pre-up-hook): orden de ejecución preservado —
  preventive.go (incluyendo H1/H2/H3) corre antes de `pre:`.
- **ADR-027** (i18n-source-discipline): mensajes nuevos de las
  3 reglas pasan por `i18n.T()`.
- Código:
  - `internal/validate/preventive.go`: archivo donde se
    extiende con las 3 reglas.
  - `internal/config/yaml_loader.go`: H1 corre antes del
    `Unmarshal` que vive acá.
- `docs/SECURITY.md`: actualizar para referenciar este ADR y
  resumir las 3 reglas + política de secretos.
