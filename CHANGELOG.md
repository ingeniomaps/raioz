# Changelog

All notable changes to this project are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.8.2] - 2026-05-16

### Added

- **`--router-off` now overrides an inherited
  `RAIOZ_ROUTER_ACTIVE=1` in project mode.** Previously the flag
  was meta-only: it controlled whether the meta runner stamped the
  env var on consumer sub-ups. A consumer invoked directly with
  the var still leaked into its environment would silently skip
  the bundled Caddy. Project mode now also honors the flag, so
  `raioz up --router-off` recovers the bundled Caddy after a meta
  exit. Threaded through `UpOptions` → `upcase.Options` →
  `processOrchestration` → `maybeStartProxy`.
- **`raioz up --audit-siblings` opt-in preflight.** Runs ADR-036
  hygiene gates (H1 secret scan, H2 path containment, H3 image
  pinning) against every sibling-dep / router-project yaml before
  spawn. Off by default — transitive trust stays the documented
  v0.7+ policy; CI and paranoid setups get the escape hatch.
  Implementation: `config.AuditYAMLStrict`,
  `upcase.auditSiblingYAMLs`, meta-side `auditMetaTargets`.
- **`MetaRunner.Up/Down/Status` emit lifecycle audit events.**
  Start fires before any sub spawns; complete is deferred so every
  return path (success, partial failure, panic-after-recover)
  closes the pair with status + duration + error. Per-sub failures
  and optional skips also record their own audit entries so a meta
  run is greppable in `audit.log` next to project-mode runs.

### Fixed

- **`internal/docker` daemon-down detection covers podman and
  nerdctl.** `wrapDaemonError` previously only matched Docker
  prose, so podman 4.x/5.x and nerdctl 1.x/2.x users got a raw
  exec failure instead of the typed
  `interfaces.ErrDaemonUnreachable` the app layer expects.
  Fixtures cover the new substrings (`podman.sock`,
  `containerd is not running`, `containerd.sock`).
- **`HostRunner.Start` no longer SIGKILLs slow launchers on clean
  raioz exit.** `exec.CommandContext` bound the child to cobra's
  signal context; every clean `raioz up` cancel reaped launchers
  like `make start` mid-build. Plain `exec.Command` plus an
  explicit `ctx.Done` case during the settle window decouples the
  long-running child while keeping SIGINT handling intact.
- **`lock.replaceStaleLock` differentiates a live-PID racer from a
  PID-reused dead racer.** A second raioz with a live PID planting
  between `Remove` and re-`OpenFile` now surfaces the actionable
  "concurrent acquire" message; a dead-PID re-grab (PID reuse by
  an unrelated process) keeps the generic "after cleaning stale
  lock" wrap. A `afterStaleRemoveHook` test hook exercises both
  branches deterministically. Strings routed through `i18n.T()`.
  Follow-up (`c13ee3c`, post go-quality + security review):
  the hook is now mutex-guarded so concurrent `go test -race -count`
  runs don't trip the race detector, and the "after stale cleanup"
  error wraps the underlying fs error with `%w` so callers can
  `errors.Is()` down to the cause.
- **`lock.isProcessRunning` works on Windows.** The previous
  `process.Signal(syscall.Signal(0))` probe returned
  `ERROR_CALL_NOT_IMPLEMENTED` on Windows — every live PID was
  reported as dead, so a concurrent racer fell through to the
  generic stale-cleanup error instead of "concurrent acquire".
  Split into `process_unix.go` (FindProcess + Signal(0)) and
  `process_windows.go` (`OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)`
  + `GetExitCodeProcess == STILL_ACTIVE`). Caught by the ADR-030
  Windows CI gate against PR #60 (`54438fa`).
- **`streamPrefixed` survives sibling log lines > 64 KiB.**
  Previously the default `bufio.Scanner` buffer truncated single
  JSON / stack-trace lines silently. The cap rises to 16 MiB and a
  `Warn` on `Scanner.Err()` surfaces any future truncation.
- **`spawnSibling` deadline branch names `RAIOZ_SIBLING_TIMEOUT`.**
  The end-to-end timeout error now points the operator at the
  knob to turn instead of a bare "timed out" message. Error
  strings for the sibling spawn path (cycle / start / timeout /
  run-failed) routed through `i18n.T()`.

### Refactor

- **`RAIOZ_CORRELATION_ID` migrates to `internal/protocol`.**
  Joins `RouterActive` and `SiblingStack` so every parent→child
  env var raioz uses lives in one place. `internal/logging.
  CorrelationIDEnv` keeps a const alias for pre-protocol callers;
  both names resolve to the same compile-time literal so producer
  and consumer can't drift.
- **Meta dispatch strings + options cleanup + `resolveBinary`.**
  Banner, summary rows, and error messages now route through
  `i18n.T()`. `MetaDispatchOptions` collapses into
  `app.MetaUpOptions` so a new knob lands in one place.
  `MetaRunner.resolveBinary` prefers `os.Executable()` with
  `filepath.Abs(os.Args[0])` fallback — dev builds invoked as
  `./raioz` now survive `runSingle`'s `cmd.Dir = sub-project`.
  A `newMetaRunner` package-level factory in `cli/meta_dispatch.go`
  lets tests inject `Binary` without monkey-patching `os.Args[0]`.
  Follow-up (`779507b`, post review): `resolveBinary` now refuses
  the fallback under `go test` via `testing.Testing()`, turning a
  silent recurse-into-the-test-runner footgun into an explicit
  error that forces tests to set `m.Binary` upfront.
- **`metaProjectYAMLPath` uses `filepath.Join`.** Replaces the
  string-concat-with-`os.PathSeparator` form so the path stays
  correct on Windows under the ADR-030 CI gate (`d53755e`).
- **Comment hygiene pass across the release.** Dropped ephemeral
  issue references from code comments (ADR refs stay; issue
  numbers move to PR / changelog scope), condensed doc comments
  that restated the diff, and replaced a hand-rolled `fmtPID`
  helper in `host_runner_test` with `strconv.Itoa`. Net −44 lines
  across 18 files; no behavior change (`3aec158`).

### Documentation

- **ADR-023 § Degraded mode.** Documents why
  `down --force-state-cleanup` relaxes the
  state-mirrors-reality invariant when the Docker daemon is
  unreachable: the container teardown can't run, the operator has
  opted in, and `forceOfflineCleanup` removes state files +
  emits a warning naming the `com.raioz.project=<name>` label
  filter so orphans can be cleared manually once the daemon is
  back.
- **`docs/SECURITY.md` § Meta env inheritance.** Names the four
  sub-spawn points that inherit the operator's full env (`pre:`
  hook, sibling spawn, custom `stop:`, meta `runSingle`), warns
  that `AWS_*` / `GITHUB_TOKEN` etc leak unfiltered, and
  recommends `env -i HOME=$HOME PATH=$PATH RAIOZ_HOME=$RAIOZ_HOME
  raioz up` for CI / untrusted yamls.
- **`--audit-siblings` scope note.** SECURITY.md and the CLI flag
  help spell out that the gate is one-hop only: it scans the
  current run's direct sibling / router yamls but the flag does
  not propagate to recursive child spawns, so a sibling's own
  siblings get only H1/H2 (no H3 escalation). Transitive preflight
  is tracked for a follow-up release (`206fffc`).
- **`docs/RATCHETS.md` + `internal/app/flow.go`.** A
  `TODO(ADR-038)` at the top of `flow.go` marks the v1.0 cleanup
  site for the legacy JSON loader; the ratchets table cross-links
  back so anyone reading the baseline lands on the file when the
  loader is finally removed.

## [0.8.1] - 2026-05-15

### Fixed

- **`raioz down --project <name>` no longer panics outside the
  project directory.** The fall-through path from the `SelectFlow`
  legacy branch left `stateDeps` nil; `down.go:198` then
  dereferenced `stateDeps.Project.Name`. Now guards the success
  message and the network-in-use logging behind a nil check,
  falling back to the CLI-resolved `projectName`. Caught by the
  `go-quality-reviewer` audit of v0.8.0.
- **`internal/lock` differentiates concurrent races from
  unremovable stale locks.** After evicting a PID-dead or aged-out
  lock, if the re-acquire fails with `os.IsExist` the helper now
  re-reads the new lock's PID and reports `"another raioz process
  acquired the lock concurrently"` instead of the generic "after
  cleaning stale lock" message. Filesystem-level failures keep the
  original wrapped error so the cause stays in the chain.

### Refactor

- **`MetaRunner` user-visible strings route through `i18n.T()`.**
  The `=== [UP|DOWN|STATUS] <project> ===` banner and the two
  optional/best-effort warning lines bypassed both the catalog and
  the i18n-source ratchet (they were dynamic `fmt.Sprintf` calls
  the ratchet does not catch). New keys `meta.banner`,
  `meta.banner_optional`, `meta.optional_failed`,
  `meta.sub_error_continuing` in en + es.

### Documentation

- **`docs/RATCHETS.md`** indexes the four shrinking-baseline
  ratchets (i18n-source, app-infra-imports, dual-flow, errorlint)
  with their target-zero ADR and current size. Future ratchets
  must publish a target — a baseline without one is permanent
  drift in disguise.
- **`docs/LOCKS.md` § Meta runner sits outside both locks** —
  documents that the meta runner takes neither the project nor
  the workspace lock, so a SIGKILL relies on Pdeathsig
  (`host.AttachPdeathsig`) on Linux and the 24h project-lock age
  cap as the cross-platform floor.
- **ADR-037 § Implementation status** updated to name the four
  shipping commits, including the E2E integration test that the
  earlier write-up listed as "deferred follow-up".
- **ADR-037 ↔ ADR-040 cross-referenced.** ADR-037 now carries a
  Trust subsection pointing at ADR-040 and noting the router is
  strictly more trusted than mode A (mandatory, no mode-B
  parallel). ADR-040 enumerates the three transitive-trust
  surfaces (`dependencies.<n>.project`,
  `dependencies.<n>.siblingProject` + `image:`, `router.project`)
  so SECURITY.md only needs one anchor.
- **ADR-038 couples v0.8 SchemaVersion removal with the JSON
  loader cut.** Once the loader hard-errors, every `LoadDeps`
  returns `SourceFormatYAML`; migrating the 5 remaining
  dual-flow entries independently would silently delete their
  YAML branch. The ADR's Timeline now states the two ship
  together.

### Refactor

- **Typed `interfaces.ErrDaemonUnreachable`** replaces the
  substring scan in the app layer. The docker adapter
  (`internal/docker/daemon_error.go`) owns the CLI-prose →
  sentinel translation; `app/down_offline.go::isDockerUnreachable`
  is now a one-liner `errors.Is(err, interfaces.ErrDaemonUnreachable)`.
  Aligns with ADR-029 — the app layer no longer reads docker
  stdout strings. `IsProjectActive` switched to `CombinedOutput`
  so the adapter sees the daemon-down message that lives on
  stderr.
- **`RAIOZ_ROUTER_ACTIVE` lives in `internal/protocol`** so the
  meta producer and the upcase consumer can no longer drift on
  the literal. New `protocol.RouterActive` const; the local
  copies in `internal/app/meta.go` and
  `internal/app/upcase/router_env.go` are gone. Future
  parent→child env-var contracts (e.g. `RAIOZ_SIBLING_STACK`)
  can move here too.
- **`envshow.go` reads `SourceFormat`** (one entry off the
  dual-flow baseline). `deps.SchemaVersion == "2.0"` was the
  easiest of the 5 inline readers to migrate cleanly because
  the branch only gates discovery-var resolution. Baseline now
  at 5 entries; target 0 by v0.8.

### Fixed

- **Meta runner now wires `Pdeathsig` on its sub-processes**
  (architecture review of v0.8.0). `MetaRunner.runSingle` shelled
  out to router + consumer raioz instances without setting
  `Pdeathsig`, so a SIGKILL on the meta parent orphaned the whole
  tree — each child still holding its own project lock,
  potentially mid-`docker compose up`. The Pdeathsig wiring moved
  from `internal/app/upcase/sibling_spawn_*` to
  `internal/host.AttachPdeathsig`; both call sites (sibling spawn
  + meta runner) now share it. Linux + non-Linux tests pin the
  wiring at both packages.

## [0.8.0] - 2026-05-15

### Added

- **Replaceable edge router** (ADR-037, issue 066). Workspaces
  can declare `router.project: ./gateway` in a meta config to
  swap raioz's bundled Caddy for a sibling raioz project that
  owns edge routing. The router project comes up first
  (non-optional, abort-on-fail), consumer sub-ups see
  `RAIOZ_ROUTER_ACTIVE=1` and skip their bundled Caddy via the
  new `maybeStartProxy` gate, and `raioz down` tears the
  router down last. `raioz up --router-off` bypasses the
  branch for debugging. V1 ships without a service-discovery
  contract from raioz to the router — the router owns its own
  templates. New corpus fixture `21-router.yaml`, 9 unit tests
  (`meta_router_test.go` + `router_env_test.go`), end-to-end
  CI gate via `scripts/integration-test-router.sh` (push-only).
  Documented in `docs/CONFIG_REFERENCE.md § Router project` and
  the README "Bring your own edge" section.
- **`.raioz.json` deprecation banner** (ADR-038, issue 068). The
  JSON loader now emits a loud one-shot warning the first time
  it runs in a process: "`.raioz.json` is deprecated — run
  `raioz migrate yaml` to convert. Support is removed in v0.8."
  The banner fires through `sync.Once` so a repo with multiple
  JSON-shaped sub-projects sees the message once per `raioz`
  invocation instead of once per scanned dir. The published
  timeline (v0.7 warning → v0.8 hard-error → v1.0 loader
  deletion) unblocks issue 069 (`isYAMLMode` dual-flow
  consolidation).
- **`Deps.SourceFormat` discriminator** (ADR-039, issue 070).
  A typed `SourceFormat` enum (`"yaml"` / `"legacy-json"`) is
  now stamped at every loader site (yaml bridge, json loader,
  auto-detect, test helpers, migrate) and preserved in every
  clone (filter-by-profile, feature-flag filter, ignore filter,
  workspace-project-conflict merge). `isYAMLMode` reads the new
  field; `SchemaVersion`'s magic literals (`"1.0"` / `"2.0"`)
  remain in seven inline call sites until issue 069 collapses
  them through `SelectFlow`. SchemaVersion is scheduled for
  removal in v1.0.
- **`SelectFlow` helper + dual-flow ratchet** (issue 069). New
  `internal/app/flow.go` centralizes the YAML/legacy-JSON
  branching that was duplicated inline across inspection
  commands. `raioz down` migrated as the proof-of-pattern;
  remaining six readers stay on the inline check and are
  tracked in `scripts/dual-flow-baseline.txt`. The
  `make check-dual-flow` ratchet fails on new readers and lets
  existing ones leave the baseline as they migrate. Target: all
  entries gone by v0.8 alongside the JSON loader removal.

### Changed

- `internal/infra/config/loader_impl.go::LoadDeps` no longer
  appends `.raioz.json is deprecated...` to the warnings slice —
  the message is now a direct `output.PrintWarning` at the
  loader source, deduped per process (ADR-038).

### Fixed

- **`raioz logs <svc>` returned 0 bytes for Dockerfile-runtime
  services** (issue 077). `DockerfileRunner.Logs` was wiring
  `cmd.Stdout` to the (always-nil) `Stdout` field of a freshly
  constructed `exec.Cmd`, so every byte from `docker logs` was
  discarded. Now wires to `os.Stdout` / `os.Stderr` the way the
  host runner does. New regression test pins the capture.

### Security

- **`install.sh` verifies the SHA-256 checksum of the downloaded
  archive** before extracting (issue 081). Previously the
  installer ignored the `checksums.txt` that goreleaser already
  publishes with every release. A poisoned tarball would have
  installed silently. The installer now fetches `checksums.txt`,
  finds the exact-name entry for the archive, and hard-fails
  on missing/mismatched checksums. `scripts/test-install.sh`
  exercises match / missing / mismatch / substring-poisoning
  cases; wired into `make check-install` and CI lint.

### Refactor

- **Caddyfile routes are sorted by hostname** (issue 074).
  Map iteration order in Go is randomized; without sorting the
  Caddyfile differed between runs with identical inputs.
  `loadAllProjectRoutes` already sorted projects and
  `HostsLine` already sorted hostnames — this closes the
  intra-project gap. New `TestGenerateCaddyfileContent_Deterministic`
  pins it.
- **`raioz proxy status/stop` now i18n-aware** (issue 084).
  `fmt.Println("Proxy is not configured")` and friends used to
  bypass both `output.Print*` and the i18n catalog, so
  `--lang es` rendered mixed English/Spanish. Migrated the
  three hardcoded strings to `output.PrintInfo` /
  `PrintSuccess` with new i18n keys (`proxy.not_configured`,
  `proxy.status_running`, `proxy.status_stopped`). The other
  CLI files flagged by the issue (env / tunnel / check) print
  structured data rows that intentionally bypass i18n; left
  as-is.
- **`errorlint` ratchet enabled** (issue 083, paso 1). Existing
  25 violations (`%s`-formatted errors, `==`/`!=` against
  errors, type-asserting raw errors) are pinned in
  `scripts/errorlint-baseline.txt`. New violations fail
  `make check-errorlint`; entries leave the baseline as call
  sites migrate to `%w` / `errors.Is` / `errors.As`. Wired
  into CI. Target: empty baseline.
- **`proxy.Manager` mutex protects configuration fields too**
  (issue 080). The lock that already guarded the `routes` map
  was renamed from `routesMu` to `mu` and extended to cover
  the 8 fields `Configure` writes (`domain`, `tlsMode`,
  `bindHost`, `projectName`, `workspaceName`, `networkSubnet`,
  `containerIP`, `publish`). Today the path is single-shot at
  startup; the lock is preventive for the hot-reconfig flows
  (`raioz dev`, watch-mode reload) that ADR-028 already
  hardened for the routes map. New
  `TestConfigureConcurrentWithReaders` pins it under `-race`.
- **`UseCase.Execute` early phases extracted** (issue 079, step
  1 of an incremental decomposition). The project-dir
  resolution + legacy ADR-011 state sweep + project-env
  resolution moved to `usecase_prepare.go::resolveProjectContext`.
  `usecase.go` drops from 391 → 359 LoC, restoring headroom
  under the 400-line cap. Remaining phases (`runOrchestrationOrCompose`,
  `persistAndAnnounce`, `attachOrWatch` and the
  `processOrchestration` sub-phases) land in follow-ups.

### Documentation

- **Sibling mode A trust model written down** (ADR-040, issue
  076). ADR-008 introduced `project:` siblings without
  explicitly publishing the trust model, leaving reviewers
  unsure whether H1/H2/H3 ran against sibling yamls before
  spawn. ADR-040 records the answer ("transitive and
  unaudited, same threat model as ADR-036") and SECURITY.md
  gains a "Transitive trust via sibling projects" subsection
  recommending mode B when the developer does not fully
  trust the sibling. An opt-in `--audit-siblings` flag is
  scoped out for a future release.

### Added

- **`raioz down --force-state-cleanup`** (issue 071). Escape
  hatch for "Docker is dead and I just want my local state
  gone." When the project liveness probe fails with a known
  daemon-down signature (e.g. "Cannot connect to the Docker
  daemon", "connection refused on docker.sock"), the flag
  bypasses the probe and runs host-process teardown + state
  file removal. The success message names the
  `com.raioz.project` label so the user can `docker rm` the
  orphan containers when the daemon recovers. Without the
  flag, the same daemon-down detection now surfaces an
  actionable suggestion pointing at the flag.

### Fixed

- **Project lock evicts after 24h to survive PID wraparound**
  (issue 075). `internal/lock/lock.go` uses an `O_EXCL` + PID
  file (portable to Windows; not `flock`). A SIGKILL'd parent
  that left its PID number reusable could see the kernel
  reassign that PID to a non-raioz process, making
  `isProcessRunning` return `true` and pinning the lock until
  the user `rm`-ed `.raioz.lock` by hand. The acquire path now
  also evicts when the lock file is older than 24h, regardless
  of liveness. Documented in `docs/LOCKS.md` under "Failure
  mode — parent SIGKILL and stale project lock."
- **Configuration drift emits one audit event per service**
  (issue 085). The drift detection in `internal/app/upcase/state.go`
  used to log a `warn` line and stop there; the
  `audit.LogDriftDetected` helper was exported with no live
  call site, so the audit log was missing the historical "when
  did this project start diverging" signal. Now every drifted
  service produces one `drift_detected` audit event with the
  field-level summary, alongside the existing live warning.
- **Legacy state migration failures are now user-visible**
  (issue 073). `MigrateLegacyStateDirs` (ADR-022) used to send
  both success and failure notes to `logging.Debug`, hidden at
  the default log level. Now successes log at info; skipped /
  failed sources escalate to `output.PrintWarning` so the user
  knows their pre-upgrade audit log / workspace state did not
  move. On failure the legacy dir also gets a
  `.raioz-migration-failed` breadcrumb (timestamp + error) so
  the user inspecting `~/.raioz/` later finds an explicit note
  instead of silence.
- **`mkcert` invocations now run under the caller's context**
  (issue 082). `EnsureCerts` used `exec.Command` instead of
  `exec.CommandContext`, so a macOS keychain prompt that never
  got answered would hang the parent indefinitely — Ctrl+C
  killed the raioz process but not the orphan mkcert. Now the
  caller's context kills the child cleanly, consistent with
  ADR-026's signal-handling umbrella. Signature changed:
  `EnsureCerts(ctx, domain)` instead of `EnsureCerts(domain)`.
- **Sibling spawn (ADR-008 mode A) gets a timeout** (issue 072).
  `spawnSibling` used to call `cmd.Wait()` unbounded — a hung
  child raioz held the parent forever. Capped at
  `RAIOZ_SIBLING_TIMEOUT` (default `10m`). Listed in
  `host.KnownDurationEnvs()` so `raioz doctor` surfaces
  overrides and warns on malformed values (ADR-035). The
  deadline error names the sibling dir and the env var, so
  the user knows exactly where to look and what knob to turn.

### Removed

- **Dead `internal/app/dependency_assist.go`** (issue 078). The
  exported `HandleDependencyAssist` had a single caller —
  its own test. The live implementation lives in
  `internal/app/upcase/dependency_assist.go`. Removing the
  duplicate eliminates -556 LoC and the grep-confusion that
  came with it.
- **`internal/resilience/circuitbreaker.go`** and its 7 wrapping
  call sites (issue 078). The breaker opened after 5 failures
  but the retry layer above it capped at 3 attempts, so the
  threshold was unreachable in practice — the two protections
  never composed. Each call site collapses to a plain
  `RetryWithContext` invocation; the retry behavior is
  unchanged in user-visible terms.

## [0.7.0] - 2026-05-14

Trust-boundary release. Three lines closed: `raioz.yaml` no longer
accepts credentials or unsafe paths (ADR-036), private repos clone
without downgrading the public-only hardening (issue 067 fases 1-3),
and the Windows runtime CI gate (ADR-030) actually runs green so the
release pipeline can vouch for windows binaries again. No
user-facing breaking changes.

### Added

- **YAML hygiene gates** (ADR-036). Three pre-flight checks that
  trip at config load:
  - **Secret scanner** — rejects `raioz.yaml` containing a known
    credential pattern (GitHub PATs `ghp_*`/`gho_*`/`ghu_*`/
    `ghs_*`/`ghr_*`, GitLab PATs `glpat-*`, AWS access keys
    `AKIA*`, Slack tokens `xox[bopa]-*`, PEM private keys).
    Error message never echoes the leaked literal — a committed
    secret is a credential-rotation incident, the scanner stops
    it at the boundary.
  - **Path traversal rejection** — every path declared in
    `raioz.yaml` must resolve inside the project dir (sibling
    project paths excepted) and may not target system locations
    like `/etc` or `/root`. Catches typos and traversal before
    they reach disk.
  - **Unpinned image warning** — `dependencies.<n>.image:` with no
    tag or `:latest` emits a warning at load. Digest-pinning
    (`@sha256:…`) is accepted; compose-backed deps without
    `image:` are unaffected.
- **`services.<n>.auth` selector** (issue 067, fases 1-3
  complete). Picks the auth strategy `raioz clone` / `raioz dev`
  apply when reaching a private repo:
  - `auth: inherit` — fully delegates to the developer's git
    config (helper, ssh-agent, Kerberos, OS keychain).
  - `auth: strict` (default, empty) — preserves v0.1 public-only
    hardening bit-for-bit. No credential.helper, no
    GIT_ASKPASS, no GIT_SSH_COMMAND, no terminal prompts.
  - `auth: gh` — scopes `gh auth git-credential` to the single
    git invocation. Token resolution lives inside the gh
    process; nothing on the command line, no global config
    mutation. Validate fails fast when `gh` is missing or
    `gh auth login` hasn't completed.
  - `auth: ssh` — rewrites `https://{github,gitlab,bitbucket}`
    URLs to `git@host:owner/repo.git` and runs git over SSH with
    `StrictHostKeyChecking=accept-new BatchMode=yes
    ConnectTimeout=10`. Trust ssh-agent / `~/.ssh/config` for
    identity selection. Self-hosted hosts pass URLs through
    unchanged.
  - Validation rejects unknown values; declaring `auth:` without
    `git:` warns instead of dropping silently.
- New `internal/git/auth/` package — `Provider` abstraction that
  gives every clone site (`ForceReclone`, `EnsureReadonlyRepo`,
  `EnsureEditableRepo`) one seam to swap hardening for per-service
  auth strategies.

### Changed

- `HostRunner.Stop` on Windows now sends a real graceful signal.
  `KillProcessTree` switched from `taskkill /T` (WM_CLOSE — silent
  no-op for console apps) to `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT)`,
  the Windows analogue of SIGTERM for console processes.
  `SetNewProcessGroup` now wires `CREATE_NEW_PROCESS_GROUP` so the
  signal is deliverable. Stops cutting to the 5s timeout +
  ForceKill loop on every host-service teardown.
- Centralized the clone-hardening helper inside `internal/git/` so
  the three call sites that copy-pasted the
  `credential.helper=`/`GIT_ASKPASS=echo`/`GIT_SSH_COMMAND=false`/
  `GIT_TERMINAL_PROMPT=0` setup verbatim share one source of truth.
- Build-tagged helpers replace skips on Windows runtime CI
  (issue 068): `ports_probe_{unix,windows}.go` map
  `WSAECONNREFUSED` / `WSAEADDRINUSE` so the TCP probes return
  definitive answers; `rename_{unix,windows}.go` wraps
  `os.Rename` with a retry-backoff loop on
  `ERROR_SHARING_VIOLATION` / `ERROR_ACCESS_DENIED` (antivirus /
  indexer / concurrent reader); `dir_unwritable_{unix,windows}_test.go`
  uses `chmod 0555` on Unix and `icacls /deny <USER>:(W)` on
  Windows to set up the same precondition cross-platform.

### Fixed

- 13 OS-specific tests that the Windows CI gate (ADR-030) surfaced
  on the v0.6.0 push now run green on `windows-latest`. Causes
  covered: path-separator assertions (`filepath.ToSlash`),
  `HOME` vs `USERPROFILE` for `os.UserHomeDir`-driven helpers
  (`setHome` helper in cert tests), legacy-migration setup
  (`USERPROFILE` alongside `HOME`), the file-permission assertion
  in `TestSaveAndLoadProcessesState` (now conditional — Windows
  ACL doesn't map to POSIX mode bits), `TestKillProcessTree_RealChild`
  (cross-platform binary choice + CTRL_BREAK_EVENT delivery),
  `TestSaveProjectRoutes_AtomicUnderConcurrency` (rename retry
  loop), `TestAssertProxyDirWritable_DirReadOnly` (icacls deny
  ACE setup). Issue 068 closed at 100%.
- `renameWithRetry` returned `os.Rename`'s error bare; wrapcheck
  flagged it. Wrapped with `fmt.Errorf("rename %s → %s: %w", ...)`
  on both Unix and Windows variants.
- `scripts/check-config-fixtures.sh` no longer fires on
  comment-only edits to schema files (`yaml_types.go` /
  `yaml_aux_types.go`). Now diffs each file and only counts as
  a "real schema change" when at least one non-comment,
  non-blank line moved.

### Notes

- The Windows port keeps existing pre-v0.6.0 skips in
  `internal/infra/exec/` and `internal/host/process_test.go`
  (tests that genuinely require a Unix shell). Those are out of
  scope for issue 068 — a separate sweep.
- ADR-037 (replaceable edge router) was authored during this
  cycle but lands in v0.8 — not implemented yet. See
  `docs/decisions/037-replaceable-edge-router.md` for the
  scoped V1 plan.

## [0.6.0] - 2026-05-14

Minor release. Tightens the proxy port contract, lands meta-orchestrator
opt-in profiles, and closes several quality gaps flagged by the audit
sweep (issues 048-064). One BREAKING change in the internal
`ProxyManager` port; user-facing YAML stays backward compatible.

### Added

- **Meta-orchestrator profiles** (issue 048). `kind: meta` configs gain
  a `profiles:` field per sub-project. Empty list = always-on;
  non-empty = skipped unless the user passes `--meta-profile <name>`
  matching one. `raioz down` deliberately ignores profiles so a sub
  started under a different set can't strand. Distinct from the
  service-level `--profile` flag (no namespace collision).
- **Lifecycle audit events** (ADR-026 / issue 048). `up`/`down`/`restart`
  emit start+complete pairs with a propagated `correlation_id` so
  recursive sibling spawns share IDs across audit logs. `dev`
  promote/revert and sibling-deferred verdicts also emit events.
- **`version:` schema gate** (ADR-031 / issue 054). Missing, newer,
  older, or malformed `version:` values now emit distinct warnings
  instead of silent acceptance. Escalation plan published: v0.7
  hard-errors past-version, v1.0 hard-errors any mismatch.
- **i18n source-discipline lint** (ADR-027 / issue 058). New
  `make check-i18n-source` enforces a shrinking baseline so new
  `output.Print*` calls must route through `i18n.T(...)`.
- **App-layer infra-imports ratchet** (ADR-029 / issue 049). Baseline
  list of files allowed to import `internal/{docker,proxy,orchestrate}`
  directly; new files fail outright. Wired into `make check`.
- **Windows runtime CI** (ADR-030 / issue 050). New
  `Unit tests (Windows)` job runs on push to develop/main against
  OS-sensitive packages. PRs keep the cheaper cross-compile gate.
- **Goreleaser dry-run on every PR** (ADR-033 / issue 056). Catches
  packaging-time regressions (archive templates, changelog regex,
  `verify-stamp.sh` script) before tag.
- **`raioz doctor::checkEnvironment`** (ADR-035 / issue 062). Surfaces
  resolved duration env vars and flags typos like
  `RAIOZ_LAUNCHER_TIMEOUT=60` (missing unit) loudly.
- Documentation: `docs/SECURITY.md`, `docs/STATE.md`, `docs/LOCKS.md`,
  unified env-var reference in `CONFIG_REFERENCE.md` (issues
  051 / 052 / 053 / 063).

### Changed

- **BREAKING (internal)** `ProxyManager` port keeps only
  `Configure(cfg ProxyConfig)` (ADR-013 Phase 2 / ADR-032 /
  issue 055). The eight per-field setters (`SetDomain`, `SetTLSMode`,
  …) are gone. `ProxyConfig.TLSMode` is the typed
  `interfaces.TLSMode` enum (`TLSModeLocal` / `TLSModeACME` /
  `TLSModeManual`), not a string. User YAML still accepts the legacy
  aliases `mkcert` / `letsencrypt` through `ParseTLSMode`.
- Malformed duration env vars now warn once per (process, var) via
  `sync.Map` dedup (ADR-035 / issue 062).
- Shared-map mutexes for `proxy.Manager.routes` and
  `orchestrate.HostRunner.{processes,launchers}` (ADR-028 /
  issue 059).
- Removed transient `issue NNN` references from source comments;
  durable ADR cites stay. Trimmed verbose follow-up/TODO-style intent
  notes (issue 064 + comment audit).

### Fixed

- `HostRunner.Start` now closes the parent's log fd in the wait
  goroutine so long watch-mode sessions stop accumulating handles
  until GC (ADR-034 / issue 061). Linux regression test polls
  `/proc/self/fd`.
- Sibling spawns reap on parent exit via signal context +
  `Pdeathsig = SIGTERM` (ADR-026 / issue 057). macOS/Windows fall
  back to ctx cancel.
- `install.sh` no longer drops into dev mode when piped via stdin
  and cleans its tempdir via a global EXIT trap.
- Coverage for the destructive `down --conflicting` /
  `--all-projects` / `sweepLauncherOrphans` / `downSelectiveServices`
  paths went from 0% to 80-100% (issue 060). Package coverage
  71.3% → 73.8%.

### Notes

- `--meta-profile` and the service-level `--profile` flag are
  separate namespaces and can be combined in the same command.
- The corpus test now routes `kind: meta` fixtures through
  `LoadMetaConfig` so the meta shape is part of the locked contract.
- Internal cleanup: dead `readLogTail` wrapper deleted, orphan
  comments stripped (issue 064).

## [0.5.2] - 2026-05-14

Patch release. Closes the version-stamping gap that left every
official binary from `v0.4.0` onwards reporting `version dev`,
and reworks `install.sh` so the freshly installed binary actually
wins `command -v raioz`.

### Fixed
- Release binaries now report their real version. `.goreleaser.yml`
  injected ldflags into `main.{version,commit,date}`, but the
  variables live in `raioz/internal/cli.{Version,Commit,BuildDate}`
  — every official tarball from v0.4.0 onwards reported
  `version dev`. Fixed by aligning the ldflag paths with the
  Makefile, and adding a smoke-test step in `release.yml` that
  fails the workflow if the produced binary is unstamped.
- `internal/cli/version.go` now falls back to
  `runtime/debug.ReadBuildInfo()` when ldflags didn't inject
  metadata. This makes `go install github.com/ingeniomaps/raioz/cmd/raioz@vX.Y.Z`
  produce a correctly-versioned binary, and defends against future
  ldflag regressions.

### Changed
- `install.sh` is now PATH-aware. The default install directory is
  the first of `~/.local/bin`, `~/bin`, `/usr/local/bin` that is
  already on the user's PATH — so the freshly installed binary
  isn't silently shadowed by an older copy in a higher-priority
  directory. Override with `INSTALL_DIR=...` as before. The
  post-install step now also detects when `command -v raioz`
  resolves to a different copy than the one we just wrote, and
  prints concrete cleanup instructions instead of claiming success.

## [0.5.1] - 2026-05-14

Build-fix release. `v0.5.0` tagged successfully but the goreleaser
build failed at the Windows targets — `internal/proxy/workspace_lock.go`
(ADR-010, shipped in `v0.5.0`) used `syscall.Flock` / `syscall.LOCK_EX`
which only exist on Unix. Linux CI never caught it because it only
cross-builds against itself. No published artifacts were affected;
this release replaces the missing `v0.5.0` binaries.

### Fixed

- **Windows cross-compile.** Split `internal/proxy/workspace_lock.go`
  into a platform-neutral shell plus `workspace_lock_unix.go`
  (`syscall.Flock`) and `workspace_lock_windows.go`
  (`golang.org/x/sys/windows.LockFileEx` — already a transitive
  dep, no new imports). Both implementations honor the ADR-010
  contract: exclusive advisory lock, blocking acquire, idempotent
  release.

### Changed

- **CI cross-compile gate.** `ci.yml`'s test job now runs
  `GOOS=windows GOARCH=amd64 go build ./cmd/raioz` and the matching
  `GOOS=darwin` build after the linux build. Adds ~15s to the test
  job and would have failed the v0.5.0 commit instead of the
  release.

## [0.5.0] - 2026-05-14

The headline is **architecture hardening**: the legacy `.state.json`
snapshot API is gone (ADR-011 phases 1-3), domain model types own
themselves (ADR-009), runner dispatch goes through a package-init
registry instead of a 23-case switch (ADR-019), and snapshot / tunnel
/ proxy lifecycles route through use-case ports (ADR-014/015/016)
behind matching adapters under `internal/infra/`. Eight ADRs that
used to live as prose in `CLAUDE.md` are now standalone documents.
Plus a `preUp:` hook for bootstrap that needs sibling-spawned deps,
a unified `naming.RaiozStateDir()` with XDG-conformant resolution,
audit-log rotation, and a launcher-pattern container wait so
`raioz up` no longer claims "ready" while `docker compose up
-d --build` is still building.

### Added

- **`preUp:` hook** (ADR-024) — runs after infra and sibling-spawn but
  before the project's own services. Use it for bootstrap that needs
  a workspace dep already reachable (`make createdb` against a
  sibling-spawned postgres). `pre:` keeps its pre-everything contract.
  Failure aborts. YAML-mode only. The hook runs on the host — reach
  deps via published `localhost:port`, not container DNS.
- **`naming.RaiozStateDir()`** as the single source of truth for
  runtime state (ADR-022). Resolution: `RAIOZ_HOME` →
  `$XDG_STATE_HOME/raioz` → `~/.local/state/raioz`. `audit`, `ignore`,
  and `workspace` all delegate; `MigrateLegacyStateDirs` runs once
  from `rootCmd.PersistentPreRun` to lift `~/.raioz` and
  `/opt/raioz-proyecto` into the unified root on upgrade.
- **Audit-log rotation** (ADR-020). `internal/audit/audit.Log`
  rotates `audit.log` to `audit.log.1` once it crosses a 10 MiB soft
  cap. Rotation failures don't drop events.
- **Dev-build warning** (ADR-021). A binary built without `-ldflags`
  (plain `go build`, `go install`) prints a one-time stderr warning
  at startup pointing at `make build` / the right ldflags;
  `raioz doctor` surfaces the same signal as a yellow `Build info`
  check. `CONTRIBUTING.md` documents the reproducible build invocation.
- **`raioz yaml lint`** subcommand. Reports each field your
  `raioz.yaml` uses alongside the version that introduced it, and
  warns when `version:` is missing. Powered by `since: vX.Y.Z`
  markers on every yaml-tagged field plus an AST walker.
- **Optional `version:` field** at the top of `raioz.yaml`. Loading
  emits a warning when absent; `raioz init` and `raioz migrate yaml`
  write `version: "1"` on new files. Locks the schema for future
  binaries.
- **Workspace-scoped proxy reload lock** (ADR-010). A second
  `raioz up` in the same workspace could render a different Caddyfile
  and reload Caddy in arbitrary order. A flock now serializes
  `SaveProjectRoutes`, `RemoveProjectRoutes`, and `Reload` within
  and across processes.
- **`ResolveContainer`** in `internal/naming` probes the canonical
  name then falls back to `com.raioz.*` labels, closing the gap
  where compose deps with a custom `container_name:` made the proxy
  and discovery point at the wrong container.
- **Sibling re-probe before dispatch.** A `raioz down` of a sibling
  between `decideSibling` and the consumer's dispatch left env vars
  wired at containers that no longer existed. The re-probe now
  fails fast with `SIBLING_DOWN` and a `cd <sibling> && raioz up`
  hint.
- **Eight new ADRs** under `docs/decisions/` covering wiring, the
  CLI thin-viz exception, runner registry, audit rotation, dev-build
  warning, unified state paths, state-mirrors-reality, the `preUp:`
  hook, and the launcher-pattern container wait. Old prose in
  `CLAUDE.md` collapsed to an index.
- **`docs/OBSERVABILITY.md`** — when to use `logging` vs `audit` vs
  `notify` vs `output`, the event matrix, and a worked example of
  how `raioz up` materializes across all four. Linked from
  `ARCHITECTURE.md`, `CLAUDE.md`, and `CONTRIBUTING.md`.
- `raioz switch` — closes the remaining gap from #24 (camino B).
  Detects active raioz projects (cross-workspace) holding host
  ports declared in the cwd's `raioz.yaml`, prompts with the list
  of offenders + the ports they hold, stops them on confirmation,
  then runs `raioz up`. `--yes` skips the prompt for scripting;
  `--keep proj1,proj2` spares specific projects from teardown.
  Detection shares `docker.ValidatePorts` with `down --conflicting`
  and `ports --conflicting` so the three commands stay aligned.

### Changed

- **Domain model types own themselves** (ADR-009). `Deps`, `Service`,
  `Infra`, `ProjectState`, and the state graph moved to
  `internal/domain/models`; every caller now uses `models.X`.
  `go list -deps raioz/internal/domain/...` no longer shows
  `config`, `state`, `detect`, or `infra` — the inversion holds.
- **`DockerRunner` segregated into six interfaces** (ADR-012 Plan B).
  Six narrow ports (`ContainerManager`, `ComposeRunner`,
  `NetworkManager`, `VolumeManager`, `ImageValidator`,
  `PortValidator`) replace the 46-method aggregate. The aggregate
  keeps embedding all six so existing callers compile unchanged,
  but new tests can mock only the surface they exercise.
- **Snapshot / tunnel / proxy lifecycles** now route through
  use-case ports (ADR-014/015/016): `interfaces.SnapshotManager` +
  `internal/app/snapshotcase`, `interfaces.TunnelManager` +
  `internal/app/tunnelcase`, and `internal/app/proxycase` with
  `Status` / `Stop` use cases plus a preflight composable from
  three probes (mkcert + ports 80/443 gated on `publish:`).
  Adapters under `internal/infra/{snapshot,tunnel,proxy}` wrap the
  concrete packages.
- **Proxy configuration via a single value** (ADR-013 Phase 1).
  `interfaces.ProxyConfig` + `ProxyManager.Configure` replace the
  4-8 setter dance with a struct literal. Eight per-field setters
  marked `Deprecated`; the migration is opportunistic.
- **Adapter wiring moved to `internal/cli/wiring.go`** (ADR-018).
  `internal/app/dependencies.go` now owns only the struct shape and
  `NewDependenciesWithMocks`; production wiring lives next to the
  CLI. Two new thin adapters (`internal/infra/proxy` and
  `internal/infra/discovery`) bring every adapter under
  `internal/infra/`. `wiring_test.go` guards every port is wired.
- **Runner dispatch via package-init registry** (ADR-019). Replaces
  the 23-case switch in `Dispatcher.selectRunner` with a `runtime →
  selector` map populated by each runner file's `init()`.
  `models.AllRuntimes()` returns the canonical list;
  `TestAllRuntimesHaveRunner` enforces exhaustiveness — a new
  `Runtime` constant added without a `register()` call now fails CI
  instead of silently hitting "unsupported runtime" at first dispatch.
- **CLI thin-viz exception codified with a lint gate** (ADR-017).
  Every `internal/cli/*.go` must import `raioz/internal/app` unless
  it appears on the exempt list (scaffolding, parent commands,
  pure-viz commands, plus one tagged tech-debt). The list lives in
  three coordinated places — the lint script, `ARCHITECTURE.md`,
  the ADR — so silent expansions surface in review.
  `make check-cli-layering` wires the gate into `make check`.
- **Naming label filters** (`internal/naming/labels.go`) replace
  three container-filter sites that built the `com.raioz.*` label
  map by string literal. A rename of any key would have silently
  bypassed them; `make check-labels` now prevents the regression.
- **Schema fixture corpus** under `internal/config/testdata/configs/`
  isolates 18 documented `raioz.yaml` combinations.
  `TestConfigCorpus` parses every fixture through `LoadYAML`;
  `check-config-fixtures.sh` fails CI when `yaml_types.go` changes
  without a matching fixture diff.
- **Toolchain minimum raised to Go 1.26.** CI workflows (`ci.yml`,
  `release.yml`) and `go.mod` now require 1.26; 1.24 fell out of
  the Go team's support window. `golangci-lint` pinned to
  **v2.12.2** (built with Go 1.26.2); v2.1.6 refused to lint a
  Go 1.26 project.
- `golang.org/x/sys` bumped 0.38.0 → 0.44.0 — includes a windows
  `uint16` overflow fix in `NewNTUnicodeString` that directly
  affects raioz's proctree code path.

### Fixed

- **Launcher-pattern premature ready** (ADR-025). When a host
  `command:` exits cleanly inside the settle window and the user
  declared `proxy.target:` with a container name, HostRunner now
  polls Docker for that container after the launcher exits (up to
  `RAIOZ_LAUNCHER_TIMEOUT`, default 60s) before reporting ready.
  `raioz down` drains in-progress builds before invoking `stop:`
  (`RAIOZ_LAUNCHER_DRAIN_TIMEOUT`, default 30s) so no orphan
  containers when the build finishes post-stop. Host-shaped targets
  (`host.docker.internal`, IPs, dotted names) skip the wait.
- **Custom `stop:` env inheritance + visible failures.** `cmd.Env`
  ran as nil-or-RAIOZ_ENV_FILE-only, so the child had no `PATH` and
  `make dev-docker-stop` / `docker compose down` wrappers failed
  silently while raioz printed "Project stopped" and left containers
  running. Seed `cmd.Env` from `os.Environ()` before overrides,
  collect per-service failures, render an `[error]` block, and
  return `ErrCodeServiceStopFailed` so `raioz down` exits non-zero
  when a service couldn't be stopped.
- **`raioz.root.json` cleanup on `down`** (ADR-023). `up` wrote a
  per-project snapshot drift detection reads on the next `up`;
  `down` never deleted it, so a project that hasn't run in months
  still surfaced as "drift" against current config. `root.Delete(ws)`
  is now invoked from `downOrchestrated` when no raioz-labeled
  containers survive the teardown. Selective downs (`raioz down svc`)
  leave the file alone.
- **Atomic proxy route persistence** (cef0fa2). Two `raioz up` in
  the same workspace could race on each other's route files: the
  Caddyfile reader would observe a partial write and silently drop
  that project. Writes now use temp + rename, parse failures log
  instead of silencing.
- gosec G702 (command injection via taint) and G703 (path traversal
  via taint) now excluded by design — same class as G204 + G306,
  raioz orchestrates other binaries and reads user-configurable
  paths as its contract.
- **`staticcheck` / `govet` quickfixes** (3106126). 25
  `staticcheck QF1012` (`WriteString` of `fmt.Sprintf` collapsed to
  `fmt.Fprintf`) and one `govet inline` (`reflect.Ptr` →
  `reflect.Pointer`), surfaced by the v2.12.2 bump.

### Removed

- **Legacy `.state.json` snapshot API** (ADR-011 phases 1-3).
  `state.{Save,Load,Exists}` and the matching `StateManager`
  methods plus `CompareDeps` / `FormatChanges` are gone, along with
  the infra adapter wrappers. The workspace-project conflict prompt
  in `upcase/dependency_assist.go` retired with them — without the
  snapshot there is no way to materialize the previous project's
  config for a diff, and reconstructing from labels alone yields an
  unactionable merge prompt. `raioz down --conflicting` covers the
  multi-project workspace collision case via labels.
- `CheckAlignment` now keeps only the branch-drift path; the
  config-vs-state comparison disappeared with the snapshot.
- `scripts/lint-state-legacy.sh` and its make target — there's
  nothing left to guard.

## [0.4.0] - 2026-05-12

The headline is multi-project orchestration: a raioz project can now
declare another raioz project as a dependency (#26), and a top-level
`kind: meta` config delegates up/down/status to a list of sub-projects.
The on-ramp for both: `raioz up` now detaches by default, so siblings
and sub-projects can be brought up in parallel without fighting for
the workspace lock. Plus a round of selective-command UX (`raioz
down`/`restart`/`status` accept names), the lint baseline migrated
from golangci-lint v1 → v2, and a batch of fixes around proxy / status
/ host-launcher edge cases that surfaced in the keycloak and dropi
pilots.

### Added

- Sibling raioz projects as dependencies (#26). A `dependencies.<n>`
  entry can now point at another raioz project in the same workspace
  via `project: ../sibling` (mode A: the sibling *is* the dep, raioz
  brings it up recursively when needed) or `siblingProject: ../sibling`
  combined with `image:` (mode B: fallback to the local image when the
  sibling isn't running). `requiredHostname:` validates that the
  sibling's raioz.yaml actually declares the hostname this consumer
  expects. `raioz down` of the consumer never tumba al hermano — the
  sibling has its own lifecycle, and mode B skips are persisted in
  `.raioz.state.json` so down doesn't try to tear down a container
  that was never created. Cycles (A → B → A) fail fast with the chain
  printed via `RAIOZ_SIBLING_STACK`.
- `kind: meta` at the top of `raioz.yaml` turns the file into a
  meta-orchestrator: `projects: [path, …]` lists sub-projects that
  `raioz up`/`down`/`status` walk in order (down in reverse). Each
  sub runs in its own `raioz` process (`os.Args[0]`) so global state
  stays isolated. Optional subs survive failures; non-optional aborts
  abort the whole up. Lets a single `raioz up` bring up an entire
  multi-repo workspace from one cwd.
- Selective targeting on three commands:
  - `raioz down [name…]` stops only the named services/deps, leaving
    network + proxy + state intact. Unknown names fail loudly with
    the list of valid targets — no more silently tearing down the
    whole project when the user thought they were stopping one
    service.
  - `raioz status [name…]` filters the report to the named entries.
    Unknown names error so typos don't silently widen the report.
  - `raioz restart [name…]` in YAML mode now correctly handles
    `--all` and `--include-infra`, gathers services and infra in
    deterministic order, and restarts host services (with `command:`)
    through the same path `up` uses.
- `raioz ports --conflicting` — read-only report of host ports
  currently held by other raioz projects or unrelated processes,
  with the resolution actions the interactive allocator would have
  offered. No state changes; useful in CI / dry-run contexts.
- Host launcher safety nets:
  - `command:` launchers that exit 0 (foreground process detaches
    docker / spawns a container) get a structured warning when no
    `stop:` is declared — without `stop:`, the next `raioz down`
    has nothing to call.
  - Immediate post-start death now detected: if the host process
    exits within a settle window after launch, `raioz up` reports
    the failure instead of silently claiming the service is up.
- Status: when the canonical container name misses (compose runtime
  picks its own naming, user-supplied `name:` override, etc.), status
  now falls back to `com.raioz.project` + `com.raioz.service` labels.
  `IdentifyPortOccupant` prefers labels over the prefix heuristic so
  preflight stops false-flagging the user's own deps as conflicts.
- Status priority for host services with launcher commands: when the
  service declares `proxy.target:`, that container becomes the source
  of truth — the PID-alive heuristic loses, because the user already
  named the contract.

### Changed

- **`raioz up` now detaches by default.** `--attach` keeps the old
  foreground/stream-logs behavior; `--watch` keeps following files for
  hot-reload. The workspace lock is released as soon as bring-up
  completes instead of being held for the whole foreground session.
  This unblocks running two projects from the same workspace in
  parallel and is a prerequisite for the sibling-spawn flow (#26) and
  `kind: meta` (above). **Breaking** if a script assumed `raioz up`
  blocked until Ctrl+C — add `--attach` to preserve that behavior.
- Lint baseline migrated to golangci-lint **v2** schema. CI bumped to
  `golangci-lint-action@v8`. Functional ruleset (errcheck, gosec,
  revive, wrapcheck, govet, staticcheck, …) unchanged from v0.2.0.
- Network creation is now label-stamped (`com.raioz.managed=true`)
  and `raioz down` sweeps by label rather than by name prefix. Stops
  leaking networks named by compose-managed sub-projects that don't
  match the `raioz-` prefix.

### Fixed

- Compose-runtime services now resolve under the proxy (#43). The
  network overlay publishes `{prefix}-{project}-{service}` as a
  network alias on the workspace network. Without it, the Caddyfile's
  `reverse_proxy <canonical>` line hit NXDOMAIN — every compose-
  backed service returned 502 with no log signal — because docker
  compose names containers `{compose-project}-{service}-{num}`.
- Dependencies declared with `compose:` (no `image:`) no longer
  surface as `:latest` in `raioz status` or trigger a
  `docker pull :latest` (invalid reference format) in error
  suggestions when start fails (#44). `extractTag("")` now returns
  `""` instead of inventing `"latest"`, and `DependencyStartFailed`
  branches on image presence with a compose-oriented suggestion when
  the dep is compose-backed.
- `raioz restart --all` and `raioz restart --all --include-infra` in
  YAML mode now actually iterate the services / infra instead of
  printing "No services specified. Use service names or --all" —
  suggesting the very flag the caller had already passed (#45).
- `Starting infrastructure (N)` no longer counts deps deferred to
  sibling raioz projects (#26 polish). The number now reflects only
  the deps that will actually be dispatched.
- Workspace state moved out of `/tmp` to `$XDG_STATE_HOME`
  (default `~/.local/state/raioz/…`). Previously, `/tmp` getting
  cleared by the OS could leave the Caddy container with a
  bind-mount source that no longer existed, blocking every
  subsequent `raioz up` with a cryptic "is a directory" error
  (issue 015 → GitHub #42). Legacy `/tmp` paths are cleaned by the
  next `down`.
- `raioz up <name>` (selective bring-up) no longer wipes all
  `HostPIDs` from local state — only PIDs for services actually in
  scope are reconciled, so untouched host services keep running
  across selective ups.
- Last project leaving the workspace properly removes the per-project
  Caddyfile and routes directory; previously, residue accumulated
  in `~/.local/state/raioz/…/proxy/routes/`.
- Host launcher cleanup: if process-group kill doesn't reap children
  (some launchers double-fork to detach), raioz now falls back to a
  PID-by-PID kill and a final cwd-scoped sweep before declaring the
  service stopped. Eliminates orphan processes left behind by
  `make dev-docker`-style launchers.
- Sibling-deps polish (all carry `(#26)` in the commit log):
  preflight accepts deps with only `project:` (no image/compose);
  recursive sibling spawn skips the workspace lock (was deadlocking
  on parent-still-holding); cycle detection now logs the full chain
  instead of `A → A`; selective `raioz down <sibling-dep>` is no
  longer a silent no-op; port allocator skips mode A sibling deps
  (the sibling allocates its own ports); sibling deps are excluded
  from health checks, proxy routing, and endpoint reporting because
  the consumer doesn't own those containers.

### Migration

If you had a script doing `raioz up && do_thing_after_services_are_up`
counting on `raioz up` to block, add `--attach`:

```diff
- raioz up
+ raioz up --attach
```

Or, the idiomatic replacement: drop the `--attach` and let `up`
return immediately — `raioz status` / `raioz logs` cover the
follow-up needs that foreground mode was filling.

## [0.3.0] - 2026-04-17

Follow-ups to v0.2.0 surfaced by pilot users running
workspace-shared mode: a proxy-field plumbing gap exposed in
runtime, a second class of silent-field-drop in the
workspace-merge clone path, and a new multi-hostname surface for
admin/user split patterns (Keycloak, gradual domain migrations).
No breaking changes to `raioz.yaml` or CLI flags.

### Added

- `services.<n>.hostnameAliases` and `dependencies.<n>.hostnameAliases`
  — expose the same upstream under extra subdomains without duplicating
  the declaration. Aliases share one Caddy site block (single
  `reverse_proxy`, one TLS directive under mkcert) and each one gets its
  own `--network-alias` so container→container DNS works for every name.
  Unblocks the Keycloak admin/user split (`sso.example.dev` +
  `accounts.example.dev`) and any multi-hostname API pattern. Empty list
  = prior behavior.
- `raioz down --conflicting` and `--all-projects`. The first stops
  sibling projects (cross-workspace) whose published host ports collide
  with the cwd's `raioz.yaml`, freeing the ports so `raioz up` can
  proceed without a manual `cd` to the other repo. The second stops
  every active raioz project except the cwd's. Detection uses the
  `com.raioz.project` container label; teardown uses `docker rm -f`
  per label. Bypasses the other project's `post:` hook and leaves its
  `.raioz.state.json` stale — the next `raioz up` in that repo
  reconciles via state-vs-docker diff. Flags are mutually exclusive
  and never touch the cwd project itself.

### Fixed

- `dependencies.<n>.hostname:` and `dependencies.<n>.proxy.port:` are
  now honored by the proxy. Both fields parsed cleanly but were dropped
  by the YAML→`Infra` bridge before reaching `buildProxyRoute`, so
  deps fell back to the entry name as the subdomain and to the
  detection-picked port as the upstream. Multi-port images (e.g.
  `mailhog/mailhog` exposing 1025 SMTP + 8025 UI) ended up routed to
  the wrong port. Added `Hostname` to `Infra`, propagated both fields
  through `yaml_bridge`, and made `proxy.port` standalone (no
  `proxy.target`) override the detected port. Also stops emitting the
  `legacy ports:` warning when a dep declares `proxy:` or `hostname:`
  — the recommended migration to `publish:` + `expose:` would break
  routing in those cases.
- `dependencies.<n>.hostname:` is now honored in runtime under
  workspace-shared mode. The YAML bridge already populated
  `Infra.Hostname` (the fix above), but `cloneInfraEntry` in the
  workspace-merge path dropped it on every re-up, so the persisted
  route and Caddyfile kept falling back to the entry name. Root cause
  was the same class of silent-field-drop previously fixed for
  `ProxyOverride`: the clone functions reinstantiated the struct field
  by field and new fields weren't listed. `cloneInfraEntry` now copies
  `Hostname`, `HostnameAliases`, and `Seed` (latent bug — the seed file
  list for `/docker-entrypoint-initdb.d/` was also being dropped).
  Regression guarded by a generative test that reflects over every
  exported field on `config.Service` and `config.Infra` and fails if
  the clone returns a zero value — next time someone adds a field, CI
  rejects with a pointer to the clone function to update.

## [0.2.0] - 2026-04-15

Pays back the technical-debt items the v0.1.0 release deferred:
linters, Windows binaries, dependency tracking, image-port detection,
and a coverage push. No breaking changes to `raioz.yaml` or CLI flags.

### Added

- Windows binaries (`windows/amd64`, `windows/arm64`) ship from
  goreleaser. Process-tree management (Setpgid + group kill on Unix)
  and disk-space probes (`syscall.Statfs`) split behind `_unix.go` /
  `_windows.go` build tags. New `internal/host` exports
  `KillProcessTree`, `ForceKillProcessTree`, `SetNewProcessGroup`, and
  `IsProcessAlive` so the three sites that needed Unix-only signals
  (host runner, host lifecycle, down) share a single cross-platform
  abstraction. Windows uses `taskkill /T` for tree kill, `tasklist`
  for liveness, and `golang.org/x/sys/windows.GetDiskFreeSpaceEx` for
  disk space.
- Proxy routes for image-based dependencies now read `EXPOSE` from the
  image manifest. After deps start, raioz runs
  `docker image inspect --format '{{json .Config.ExposedPorts}}'` for
  any dep whose `detection.Port` is still 0, picks the lowest TCP
  port, and writes it back so the proxy reaches `postgres:5432`,
  `pgadmin4:80`, etc. without the user copying the port into `ports:`
  or `expose:`. Results cache per `image:tag` for the process
  lifetime; lookup failure preserves the existing `Port: 0` fallback
  chain.
- Dependabot now tracks GitHub Actions versions
  (`actions/checkout`, `setup-go`, `golangci-lint-action`, etc.)
  alongside Go modules. Weekly Monday schedule, separate commit
  prefix (`ci`), 5-PR cap.

### Changed

- Lint baseline tightened in four atomic PRs:
  - `errcheck` enabled; `_test.go` excluded. 37 production sites
    addressed (best-effort cleanup gets explicit discards with
    why-comments; real errors propagate or log; Cobra flag boilerplate
    discards).
  - `gosec` enabled; G204 (subprocess with variable) and G306
    (WriteFile permissions) excluded globally with rationale — raioz
    orchestrates docker by design and writes user-readable configs.
    G115 suppressed inline at the one safe site (filesystem block
    size cast).
  - `revive` enabled with a curated 17-rule set (default fires
    ~980 issues mostly from `unused-parameter` and `exported`, which
    don't fit this codebase's conventions). Fixed 5 production hits:
    `copy`/`max` builtin shadowing, empty blocks, var-declaration
    redundancy, if-return collapses.
  - `wrapcheck` enabled scoped to errors from outside raioz
    (`ignorePackageGlobs: raioz/internal/**`); `internal/infra/`
    (hexagonal adapter layer) and `_test.go` exempted. 58 stdlib /
    third-party error sites wrapped with `fmt.Errorf("…: %w", err)`.
- Coverage threshold raised from 70% to 73%, with `internal/mocks`
  and `internal/testing` excluded from the metric (test
  infrastructure, not production code). Real total now ~74%. New
  unit tests cover pure helpers in `compose_spec`, `hosts`,
  `update_port`, `infer_deps`, `naming`, `host/proctree`,
  `production`, and `output`. Path to 80% needs integration tests
  under a live Docker daemon — see ROADMAP.

### Fixed

- `host_runner.Restart` no longer ignores the error from `Stop`
  ahead of `Start` — silenced explicitly with a comment so the next
  reader knows the intent.
- `cleanStaleHostProcesses` no longer silently drops the
  `state.SaveLocalState` error after clearing PIDs — the call is
  best-effort but documented.

## [0.1.1] - 2026-04-15

Patch-level fixes for configuration parsing, surfaced by the keycloak
pilot user configuring `raioz.yaml` against v0.1.0 (2026-04-14).

### Added

- `dependencies.<n>.proxy: {target, port}` — mirror of the existing
  `services.<n>.proxy:` escape hatch. Overrides proxy detection for a
  dependency whose runtime raioz can't fully introspect (e.g. a
  `compose:`-backed dep whose target container name or port doesn't
  match the defaults). Bridges into `Infra.ProxyOverride` and is read
  by `buildProxyRoute` via the same `proxyTargetOverride` path used for
  services.
- Advisory warnings for unknown YAML fields at config load. Typos
  (e.g. `whtch:` instead of `watch:`) or fields introduced by a newer
  raioz version now surface as `<file>: line N: field <name> not found
  in type …` on stderr instead of being silently dropped. Warning-only
  by design to preserve forward compatibility; a `--strict` flag for
  hard fail is tracked for a future release.

### Fixed

- `dependencies.<n>.proxy:` was accepted by the YAML parser but
  silently dropped — Caddy then routed the dependency through its
  image's default port (typically 80) regardless of what the user
  declared. The bridge layer now populates `Infra.ProxyOverride` and
  `cloneInfraEntry` propagates it through the workspace-merge path.
- Proxy port fallback for dependencies now consults
  `dependencies.<n>.expose[0]` when detection couldn't resolve a port
  and the legacy `ports:` field is empty. Previously a dep that only
  declared `expose:` would get a proxy route with port 0. `ports:`
  still wins when both are set, preserving existing behavior.

## [0.1.0] - 2026-04-14

First stable release. Complete rewrite from Docker Compose generator
to meta-orchestrator: raioz no longer generates compose files — it
detects runtimes and runs services natively under a shared network
with HTTPS via Caddy.

### Added

#### Core
- `raioz.yaml` as primary config format (services + dependencies).
- Auto-detection of 24 runtimes (Go, Node, Python, Rust, PHP, Java, .NET, Ruby, Elixir, Dart, Swift, Scala, Clojure, Zig, Gleam, Haskell, Deno, Bun, Make, Just, Task, Compose, Dockerfile).
- Zero-config mode: `raioz up` without any config file.
- `raioz init` auto-scans project and generates `raioz.yaml`.
- Host process lifecycle management (PID tracking, cleanup).
- Container runtime abstraction (Docker, Podman, nerdctl).
- Container labels `com.raioz.managed`, `com.raioz.workspace`, `com.raioz.project`, `com.raioz.service`, `com.raioz.kind` stamped on every raioz-created container. Shared deps omit `com.raioz.project` to signal workspace ownership.

#### Proxy & networking
- Caddy reverse proxy with automatic HTTPS via mkcert.
- `https://<service>.<domain>` for every service.
- Custom domain support (`proxy.domain`).
- WebSocket, SSE, and gRPC routing.
- Automatic service discovery with injected env vars.
- Workspace-shared proxy mode — when `workspace:` is declared, a single `{workspace}-proxy` Caddy fronts every project in the workspace. Routes persisted per project at `/tmp/<workspace>/proxy/routes/<project>.json`; Caddyfile is the union of every project's contribution. `raioz down` removes only the current project's routes and reloads; only the last project leaving tumba the proxy.
- `proxy.ip` optional field — pin the Caddy container's IP inside the Docker network. Default: `<subnet-base>.1.1` when `network.subnet` is declared.
- `proxy.publish` optional field (default `true`) — when set to `false`, the proxy does NOT bind host ports 80/443. Reachable only via its container IP, so multiple workspaces can run in parallel without port contention. Requires `network.subnet` or explicit `proxy.ip`. Linux-only.
- `raioz hosts` command — prints an `/etc/hosts` entry for the current project's proxy (container IP + every proxied hostname). Ready for `sudo tee -a /etc/hosts`. Trailing `# raioz:<workspace>` comment makes entries grep-findable.
- Interactive recovery menu when the proxy fails to start on an interactive tty (Retry / Skip / Cancel). Non-interactive stdin still hard-fails.

#### Configuration
- `network.name` and `network.subnet` optional fields in `raioz.yaml` — pin the Docker network name and subnet explicitly. String shorthand (`network: my-net`) also accepted.
- `dependencies.<n>.name` — literal container name override for a dep.
- `dependencies.<n>.routing` — opt-in HTTPS proxy route for a dep whose image matches the DB/broker heuristic.
- `services.<n>.proxy.{target, port}` — override detection when `command:` launches a compose stack raioz can't see.

#### Developer experience
- Multiplexed logs from all services with colored prefixes.
- File watching with debounced auto-restart (`watch: true`).
- `--attach` flag for foreground mode.
- Interactive TUI dashboard (`raioz dashboard`).
- Dependency graph visualization (`raioz graph` — ASCII, DOT, JSON).
- Volume snapshots (`raioz snapshot create/restore/list/delete`).
- Tunnel support (`raioz tunnel` — cloudflared, bore).
- `raioz dev` to hot-swap a dependency from image to local code.
- Package manager auto-detection from lock files (yarn, pnpm, bun).
- Air integration for Go projects with `.air.toml`.
- Workspace naming for Docker resource prefixes.

#### Operations
- Infra health checks with diagnostics.
- `raioz doctor` for system diagnostics.
- `raioz ports` to list port mappings.
- Pre/post hooks (`pre:`, `post:` in config).
- Dependency inference from `.env` files (DATABASE_URL → postgres).

#### Build & CI
- GitHub Actions pipeline with lint, test, and build (consolidated into a single `ci.yml`).
- goreleaser config for cross-platform releases (Linux + macOS; Windows planned).
- Integration test script.

### Changed
- `raioz init` replaced wizard with auto-scan.
- `raioz status` shows runtime type and PID for host services, and reports the correct state for shared/dependency containers (previously always showed `stopped`).
- `raioz list` shows live status for host services.
- Resource naming centralized in `naming` package.
- `.raioz.json` config deprecated with migration warning (`raioz migrate yaml`).
- Install script rewritten for goreleaser compatibility.
- Dependencies in a workspace are now container-shared (`{workspace}-{dep}`), not per-project. First `up` creates; subsequent `up`s reuse. `down` keeps shared deps alive while any sibling project still runs in the workspace; last project out tumba them.
- Certificates are namespaced per domain (`~/.raioz/certs/<domain>/`) and their SAN is verified before reuse. Prevents silent cross-domain cert reuse.
- Caddyfile global block uses `auto_https off` in mkcert mode (was `disable_redirects`). Stops Caddy from falling back to ACME on custom domains without public DNS.
- `raioz.yaml` now fails fast when a name appears in both `services:` and `dependencies:`.
- Proxy startup now pre-flights host ports `80`/`443` and distinguishes `EADDRINUSE` (real conflict) from `EACCES` (privileged port as non-root — not our concern).
- Proxy skips HTTPS route creation for deps whose image matches a well-known non-HTTP list (postgres, redis, mysql, mariadb, mongo, rabbitmq, kafka, etc.). Use `routing: {}` to opt in.
- `.raioz.state.json` is now always written on `up` (even for projects without host services) with project, workspace, `networkName`, and `lastUp` populated. Removed on `down` if project is empty.

### Fixed
- Resolve project name for proxy status and stop.
- `raioz down` no longer sweeps containers belonging to other projects that happen to share a name prefix on the same Docker daemon.
- Service containers with a user-supplied `command:` (e.g. `make start`) are now caught by the down flow via exact-name fallback (`<prefix>-<project>`).
- Caddy proxy no longer gets stuck in `Created` state after a port conflict — stale containers are removed before retry, and the failure is surfaced as an actual error instead of a passable warning.
- `DepComposeProjectName` now uses the active naming prefix instead of hardcoded `raioz-`, so `docker compose ls` matches the real container names.
- Errors from `docker stop` / `rm` during teardown are logged with stderr instead of silently swallowed.
- Proxy port preflight no longer emits false-positive `port in use` for privileged ports when running as non-root.
- Proxy port preflight now uses a TCP dial probe before attempting a bind — unprivileged raioz processes could previously miss privileged ports (e.g. :80) actually held by another process because `net.Listen` returned `EACCES`, which was mistaken for "probe inconclusive".
- `cloneService` / `cloneInfraEntry` in the workspace-conflict merge path now copy ALL orchestration-relevant fields (`ProxyOverride`, `Port`, `HealthEndpoint`, `Name`, `Routing`, `Expose`, `Publish`). Missing fields silently vanished on re-up after a workspace state mismatch.
- `proxy.IsNonHTTPImage` classifier moved to shared `internal/proxy/filter.go` and rewritten to match on the bare image name (last path segment before tag/digest) instead of substrings. `redis/redisinsight`, `dpage/pgadmin4`, `mongo-express`, and similar HTTP UIs that share a substring with their binary-protocol namesake are now correctly proxied.
- Workspace-shared proxy: `Reload` no longer runs `docker cp` (the bind-mount target is read-only and `cp` failed with "device or resource busy"). It writes the Caddyfile on the host and calls `caddy reload` — the bind mount propagates the file into the container automatically.

### Removed
- `raioz workspace` command (replaced by workspace config field).
- `raioz link` command.
- Override system.
- `docker-compose.generated.yml` generation.

---

## Pre-pivot releases

Earlier versions used `.raioz.json` to generate Docker Compose files.
That model is deprecated. Use `raioz migrate yaml` to convert.
