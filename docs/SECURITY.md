# Security policy and threat model

## Threat model

raioz is a **local developer tool**. The assumed posture is the
same one your editor and shell scripts run under: you trust the
contents of `raioz.yaml`, the projects it references, and the
binaries on your `$PATH`. Running raioz against a config file
you didn't author yourself — or against a sibling project whose
source you haven't reviewed — is **out of scope of raioz's
security guarantees**.

This is the same model that `make`, `cargo`, `npm`, `task`,
`docker compose`, and every other local-dev orchestrator
operates under. Documenting it lets you decide deliberately:
running someone else's `raioz.yaml` is roughly equivalent to
running their shell script.

If you need to evaluate a config file from someone else, **read
it first** — every executable surface listed below is plain text
in `raioz.yaml` or the project files it references.

## Executable surfaces

raioz invokes user-controlled code in five places. Each is
intentional and documented below.

### 1. Lifecycle hooks: `pre:` / `preUp:` / `post:`

```yaml
pre: ./scripts/fetch-secrets.sh
preUp: make createdb
post: rm -f .env.*.tmp
```

Each command runs via `exec.CommandContext(ctx, "sh", "-c",
<cmd>)` from `internal/app/upcase/hooks.go`. The host's `/bin/sh`
interprets the string — pipes, redirects, variable expansion, and
quoting are all in play.

- `pre:` runs before anything else (env rendering, secrets fetch).
- `preUp:` runs after infra/sibling-spawn, before service start
  (ADR-024 — `make createdb` against a sibling-spawned postgres).
- `post:` runs after services are up (cleanup).

**Implications:** anything `sh -c` can do, these hooks can do —
including reading your home directory, exfiltrating env vars,
calling out to external services.

### 2. Service commands: `services.<name>.command:` and `.stop:`

```yaml
services:
  api:
    command: make dev
    stop: make stop
```

`HostRunner.Start` invokes `exec.CommandContext(ctx, parts[0],
parts[1:]...)` where `parts = strings.Fields(command)`. This is
**not** `sh -c` — there's no shell expansion or pipe parsing —
but the first token is searched on `$PATH` and the rest are
passed verbatim as argv. A command running on your host has the
same authority as if you ran it from your shell.

**Implications:** a config that declares `command: rm -rf
/important/path` will delete `/important/path` as your user when
you `raioz up`. No prompt, no confirmation — that's the point of
`command:`.

### 3. Recursive `raioz up` (mode A sibling spawn)

```yaml
dependencies:
  postgres:
    project: ../keycloak     # mode A — ADR-008
```

When raioz encounters `dependencies.<n>.project:`, it spawns
itself via `exec.CommandContext(ctx, os.Executable(), "up")` with
`cmd.Dir` pointing at the sibling project. The child process
**inherits the parent's full environment** (`os.Environ()`) plus
two propagated values: `RAIOZ_SIBLING_STACK` (cycle detection)
and `RAIOZ_CORRELATION_ID` (audit trace).

The child is now an independent `raioz up` — it reads
`../keycloak/raioz.yaml` and executes everything in that file
(hooks, commands, more siblings).

**Implications:** pointing `project:` at a path you don't control
runs that project's hooks and commands with your env. Cycles are
caught (`RAIOZ_SIBLING_STACK`), but the transitive trust applies:
you trust A → you trust everything A's sibling chain reaches.

### 4. Git operations: `raioz clone`

```bash
raioz clone https://github.com/some-user/their-project
```

Runs `git clone <url>` and then, by default, `raioz up` in the
checkout. `git` itself executes any `core.hooksPath` /
`core.fsmonitor` / submodule `update` hook the cloned repo
configures. The subsequent `raioz up` runs the cloned project's
hooks and commands.

**Implications:** equivalent to `git clone` plus running the
project. Use `--no-up` to skip the up step if you want to read
the config first.

### 5. Container runtimes (Docker / Podman / nerdctl)

`internal/runtime/runtime.go` selects the runtime via
`RAIOZ_RUNTIME` (default `docker`). Container runtimes hold
Docker-socket-level privileges, which on most systems is
effectively root-equivalent on the host (a container can mount
`/`, run as host root, etc.). raioz delegates trust to the
runtime binary on your `$PATH`.

**Implications:** if the runtime binary has been tampered with,
raioz can't help. The same is true for `make`, `docker`, any
other tool you run.

## YAML hygiene gates (ADR-036)

Three preflight rules in `internal/config/` reject or warn on
`raioz.yaml` content before the parsed config reaches any
executable surface. They catch a narrow class of incidents that
recur in real-world configs.

**H1 — Secret detection (error, no override).**
`internal/config/secret_scan.go` scans the raw yaml bytes before
`yaml.Unmarshal` for known credential formats: GitHub
PAT/OAuth/user-to-server/server-to-server/refresh tokens, GitLab
PATs, Slack tokens, AWS access key IDs, and PEM private keys.
Match → hard error. The matched secret never appears in the error
message — only the pattern name and approximate line number. No
flag, env var, or hash whitelist can suppress this.

**H2 — Path traversal (error).**
`internal/config/path_safety.go` requires every path in
`raioz.yaml` to resolve inside the project directory and to not
target sensitive system locations (`/etc`, `/root`, `/var/lib`,
`/sys`, `/proc`, `/dev`, `/boot`). Validated fields:
`services.<n>.{path,env,compose,command,stop}`,
`dependencies.<n>.{env,compose,dev.path}`, and
`pre:`/`preUp:`/`post:`. Sibling project paths
(`dependencies.<n>.{project,siblingProject}`) are exempt from the
containment check by design (ADR-008) but still get the
system-dir block. `command:`/`stop:`/`pre:`/`preUp:`/`post:` use a
heuristic — the first token must look path-shaped (`./`/`../`/`/`)
to be validated; bare shell commands like `make build` are
ignored. Shell constructions with embedded paths
(`bash ./scripts/foo.sh`) are intentionally not validated; that's
the user's responsibility.

**H3 — Image tag pinning (warning).**
`internal/config/image_pinning.go` emits a warning when
`dependencies.<n>.image` has no explicit tag or uses `:latest`.
Digest pinning (`@sha256:...`) is accepted. Compose-backed deps
without `image:` are not affected. Warning only — `raioz up`
proceeds.

### Auth selector for private git repos (issue 067)

`services.<n>.auth` is the schema-level expression of the
"secrets never in yaml" policy. It's a **selector**, never a
carrier — the value is one of:

- omit (default): strict / public-only hardening (v0.1 behavior).
- `inherit`: raioz removes its hardening for that clone, delegating
  fully to the dev's global git config (credential helper,
  ssh-agent, OS keychain, Kerberos, smart card). Whatever
  `git clone <repo>` would do in the dev's shell is what raioz
  does.
- `gh` / `ssh`: provider placeholders in fase 1 — fail at clone
  with a "not implemented" error that points at `inherit` as the
  working alternative. The functional implementations land in
  fase 2 / fase 3 of issue 067.

The yaml carries the **selector**; the credentials live in the
dev's environment. A teammate who clones the repo and runs
`raioz up` uses *their* credentials, never the original author's.
That is the entire reason `auth:` exists as a string field
instead of a map carrying token values.

### What this policy intentionally does NOT do

ADR-036 "won't do" section preserves the rationale for trust-pass
scope that was evaluated and rejected. In short:

- No URL classification / SSRF protection (no URL fields exist in
  the schema today).
- No allowlist of git hosts (`github.com/atacante/malware` passes
  any plausible allowlist).
- No heuristic detection of dangerous shell in hooks (`curl|sh`,
  `rm -rf`) — false-positive rate is high (legitimate `nvm`,
  `rustup` installers), and obfuscation defeats the heuristic
  trivially.
- No interactive first-run confirmation, no yaml hash persistence,
  no `--accept-script`.
- No cryptographic signing of yamls.
- No sandboxing of `pre:`/`preUp:`/`post:`.

Reconsider any of these if a concrete incident demands them. The
full case-by-case rationale is in
[ADR-036](decisions/036-trust-model-yaml.md).

### Transitive trust via sibling projects (mode A)

`dependencies.<n>.project: ../sibling` (ADR-008 mode A) spawns a
recursive `raioz up` in `../sibling`. The sibling's `pre:` /
`preUp:` / `post:` hooks run as part of the parent invocation,
even though the developer never opened the sibling's
`raioz.yaml`. The threat model treats the sibling yaml as
**code-equivalent** to the local yaml: the same H1/H2/H3
gates from ADR-036 apply to the child's own yaml when it
loads, but the parent does **not** preflight the sibling
before spawning.

The implication is symmetric with `make -C ../sibling` or
`npm --prefix ../sibling run x`: by declaring the dependency,
you are asserting trust in the sibling project and everyone
who can write to it.

Mitigations:

- **Use mode B (`siblingProject:` + `image:`) when you do not
  trust the sibling fully.** Mode B pulls the image by default
  and only delegates to the sibling raioz when the developer
  themselves brought it up locally. No transitive spawn.
- **Audit `project:` paths during yaml review** the same way
  you audit a new `make` target or shell script. ADR-040 keeps
  the case-by-case rationale and lists what raioz deliberately
  does NOT do (no sibling sandboxing, no script classifier).

## Sensitive data raioz handles

### TLS certificates (mkcert mode)

- Path: `~/.raioz/certs/<domain>/cert.pem` + `cert-key.pem`
  (ADR-003).
- Permissions: whatever `mkcert` writes (typically `0644` for the
  cert, `0600` for the key — raioz does not chmod).
- Trust: keys are signed by the mkcert local CA you installed
  with `mkcert -install`. Adding new domains does NOT re-prompt
  for trust because the CA is already in your system trust
  store. **This is a feature, not a bug** — but a hostile
  `raioz.yaml` pointing at `domain: my-bank.com` could mint a
  trusted cert for that domain. Reject configs that declare a
  `domain:` you don't own.
- Cleanup: raioz never deletes certs. `rm -rf ~/.raioz/certs/`
  is the manual purge; the local CA itself is removed by
  `mkcert -uninstall`.

### Environment variable propagation

raioz inherits the **entire** `os.Environ()` when spawning:

- Lifecycle hooks (`pre`/`preUp`/`post`) — see the inherited
  list with `printenv` inside the hook.
- Service `command:` — same; HostRunner sets `cmd.Env =
  os.Environ()` and appends declared env vars.
- Sibling spawn (`os.Executable()`) — child gets parent's full
  env (ADR-008).
- Custom `stop:` (`down`) — same; explicit fix in issue 044.
- Meta `runSingle` sub-spawns (ADR-037) — every consumer
  sub-up and the router project run with the parent meta's full
  env. raioz only **adds** signaling vars (`RAIOZ_SIBLING_STACK`,
  `RAIOZ_ROUTER_ACTIVE`, `RAIOZ_CORRELATION_ID`).

raioz does **not** scrub `AWS_*`, `GITHUB_TOKEN`, or any other
common secret env vars. If you don't want a hook or sibling to
see them, unset them before invoking `raioz`.

#### Implication for sibling / router projects

A `raioz.yaml` that names a sibling (`dependencies.<n>.project:`
or `siblingProject:`, ADR-008 / ADR-040) or a workspace router
(`router.project:`, ADR-037) at a contributor-provided path runs
that project's code — including its own `pre:` / `command:` /
`stop:` hooks — with whatever secrets the operator has exported
in their shell. The flag `--router-off` only bypasses the router
phase; it does NOT atenuate env inheritance for the other
spawn points.

#### Recommended posture

- Only run `raioz up` against meta workspaces with known
  siblings. Don't load a meta yaml from an untrusted source.
- For CI and shared runners, prefer a sanitized invocation:

  ```bash
  env -i HOME="$HOME" PATH="$PATH" RAIOZ_HOME="$RAIOZ_HOME" raioz up
  ```

  Restrict inheritance to the env vars raioz itself documents in
  [CONFIG_REFERENCE.md § Environment variables (read by raioz)](CONFIG_REFERENCE.md#environment-variables-read-by-raioz).

#### Why raioz doesn't filter automatically

A filter would have to know what each sibling / router project
needs to read (database URLs, tool tokens, OIDC issuer URLs,
...). raioz does not publish that contract today — every sibling
consumes its own env at will. A strict allowlist would break
most real workflows; ADR-040 documents this explicitly as
"trust the operator, surface the surface area" instead of
adding a filter. For the opt-in preflight scanner over sibling
yamls, see `raioz up --audit-siblings` (issue 031).

> **Scope note.** `--audit-siblings` walks the dep graph
> transitively from v0.10.0: every yaml reachable through
> `project:` / `siblingProject:` gets the strict H1/H2/H3 gate,
> not just the consumer's direct deps. A breadth-first walk
> keyed by absolute yaml path bounds the work and breaks cycles
> silently. The meta router/sub-projects are audited at the
> meta layer (see `internal/app/meta_audit.go`).

### Audit log

`audit.log` (ADR-020 / ADR-022) records project names, service
names, paths, and event types. By design, it does NOT contain:

- Contents of `.env` files.
- Values of env vars (only their names, when relevant — e.g. for
  drift detection).
- Stdout/stderr of hooks or commands.
- Network response bodies.

**Rule for new audit emitters:** never include the contents of
env files, secrets, or arbitrary command output in the `details`
map. Keep the payload to names, paths, and counts.

If you find an audit event leaking sensitive payload, that is a
security bug — file it (see "Reporting" below).

## Network exposure

### Proxy (`proxy: true`)

By default the Caddy proxy binds host ports **80** and **443**
on all interfaces. The `bindHost:` option can constrain to
`127.0.0.1`. `publish: false` skips host port binding entirely
(only the container IP is reachable — Linux only; ADR-005).

A misconfigured proxy on a multi-user box exposes every routed
service to anyone who can reach the host's network. Default
posture is "binds 80/443" because that's the most common dev
workflow; constrain explicitly when running on a shared machine.

### Tunnels (`raioz tunnel start`)

Opt-in only. Backends: `cloudflared` (Cloudflare Quick Tunnels)
or `bore` (self-hosted / bore.pub). Running `raioz tunnel start
api` publishes the named service to the public internet
**deliberately**. Tunnel state persists in `<RaiozStateDir>/tunnels/`
until you `raioz tunnel stop`.

**Implications:** a forgotten tunnel keeps your local service
public. `raioz tunnel list` shows active tunnels; check it after
demos.

### `/etc/hosts` modification

raioz **never writes** to `/etc/hosts` automatically. The `raioz
hosts` command prints the line ready for you to pipe through
`sudo tee -a /etc/hosts`. The decision to modify a privileged
system file is always explicit.

## CI / release tokens

Two GitHub Actions workflows take credentials. Both follow
least-privilege:

### `release.yml` — goreleaser

- Token: `GITHUB_TOKEN` (auto-issued per run, scoped to this
  repo, expires when the workflow finishes).
- Permissions: `contents: write` (upload release assets).
- Blast radius if leaked: limited to this repo for ≤6 hours
  (token lifetime).
- Rotation: automatic.

### `release-please.yml` — automated release PRs

- Token: `RELEASE_PLEASE_TOKEN` (fine-grained PAT, repo-scoped).
- Permissions: Contents (read/write) + Pull requests (read/write).
  Nothing else.
- Why a PAT instead of `GITHUB_TOKEN`:
  - `GITHUB_TOKEN` cannot create PRs unless the repo enables
    "Allow GitHub Actions to create and approve pull requests",
    which would also grant that capability to every other
    workflow on the repo. The PAT keeps the capability scoped
    to this one workflow.
  - Commits/tags signed by `GITHUB_TOKEN` do not trigger
    downstream workflows (GitHub anti-loop). The tag push from
    release-please must trigger `release.yml`, so PAT signing
    is required.
- Blast radius if leaked: write access to this repo's contents
  and PRs. Cannot read other secrets; cannot reach other repos
  (fine-grained).
- Rotation: PAT owner is responsible. Set an expiry on the PAT
  and recreate before expiry. The workflow fails loud (no
  silent fallback) if the secret is missing or invalid.

## What raioz does NOT protect against

- **Hostile `raioz.yaml` from third parties.** As above —
  treat it like a shell script.
- **Compromised binaries on your `$PATH`.** raioz delegates to
  `docker`, `git`, `make`, `mkcert`, `cloudflared`, `bore`. If
  any of those are tampered with, raioz can't tell.
- **Workspace-shared deps containing exploits.** If you and
  your teammate share a workspace and they declare a hostile
  dep, the dep's container runs in your workspace network with
  access to your other containers. The workspace boundary
  isolates Docker; it does not isolate the contents of shared
  images.
- **Side-channel attacks across siblings.** Mode A spawn shares
  the host with siblings (same OS, same kernel, same Docker
  socket). Standard host isolation rules apply.

## Reporting a vulnerability

Open an issue at
<https://github.com/ingeniomaps/raioz/security/advisories>
(GitHub Security Advisories — private, encrypted, viewable
only by the maintainer until disclosed). Include:

- raioz version (`raioz version`).
- Minimum reproduction (config file + command).
- Expected vs observed behavior.
- Your assessment of severity.

If you cannot use GitHub's private channel, fall back to an
issue at <https://github.com/ingeniomaps/raioz/issues> with the
title `[security] ...` — the maintainer will move it to a
private advisory before triaging.

### Triage expectations

raioz is maintained by a small team. Realistic response SLA:

- **Acknowledged** within 7 days.
- **Triaged** (severity + plan) within 14 days.
- **Fixed and released** for high-severity issues within 30
  days; medium-severity may be folded into the next minor
  release.

We do not run a bug bounty.

## References

- ADRs touching security surfaces:
  [ADR-003](decisions/003-cert-namespacing.md) (cert
  namespacing + SAN validation),
  [ADR-008](decisions/008-sibling-projects-as-deps.md) (recursive
  spawn, cycle detection),
  [ADR-022](decisions/022-unified-state-paths.md) (where state
  lives),
  [ADR-024](decisions/024-pre-up-hook.md) (`preUp:` runs on the
  host, not in a container),
  [ADR-026](decisions/026-signal-handling-and-pdeathsig.md)
  (signal handling + Pdeathsig — affects cleanup, not
  authorization).
- Layered docs:
  [STATE.md](STATE.md) lists every file raioz writes;
  [LOCKS.md](LOCKS.md) lists every serialization mechanism;
  [OBSERVABILITY.md](OBSERVABILITY.md) lists every emitter
  (incl. audit, where the no-secrets rule applies).
- Issue: 053.
