# ADR-022: One state-path helper for audit, ignore, and workspace

- **Status:** Accepted — implemented 2026-05-13
- **Date:** 2026-05-13

## Context

Three packages each owned a private copy of the "where do I write
this file" logic:

- `internal/audit/audit.go::getBaseDirForAuditLog`
- `internal/ignore/ignore.go::getBaseDirForIgnore`
- `internal/workspace/workspace.go::GetBaseDir` (+ `getFallbackBaseDir`)

All three implemented the same algorithm: honor `RAIOZ_HOME` first,
fall back to `/opt/raioz-proyecto`, then to `~/.raioz`. Each had
slightly different error messages, slightly different mkdir
permissions (0755 vs 0700), and — until somebody happened to fix one
— different bugs.

This was load-bearing duplication: a contributor changing the
location policy in one site silently created divergence with the
others, and the audit log could end up in `/opt/raioz-proyecto` while
ignored services landed in `~/.raioz` on the same machine.

Issue 042 also flagged a second problem: `/opt/raioz-proyecto` is a
Linux distro convention that doesn't transfer to macOS or Windows and
requires root to create. The fallback (`~/.raioz`) works but doesn't
follow XDG, so contributors who export `XDG_STATE_HOME` to keep their
home dir tidy see raioz pollute `~/` anyway.

## Decision

One helper in `internal/naming/paths.go`:

```go
func RaiozStateDir() string {
    if home := os.Getenv("RAIOZ_HOME"); home != "" { return home }
    if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
        return filepath.Join(xdg, "raioz")
    }
    if home, err := os.UserHomeDir(); err == nil && home != "" {
        return filepath.Join(home, ".local", "state", "raioz")
    }
    return filepath.Join(os.TempDir(), "raioz")
}
```

Plus a sibling `RaiozConfigDir()` for user configuration (XDG
separates state from config — state is "rebuildable", config is
"user-authored"). The three callers each delegate to
`RaiozStateDir()` and only keep their own filename constants:

```go
func GetAuditLogPath() (string, error) {
    base := naming.RaiozStateDir()
    if err := os.MkdirAll(base, 0o755); err != nil {
        return "", fmt.Errorf("create audit state dir %q: %w", base, err)
    }
    return filepath.Join(base, auditLogFileName), nil
}
```

The legacy `/opt/raioz-proyecto` path is no longer probed.
`RAIOZ_HOME` is kept for backwards compatibility (it has been the
recommended override for years), and the XDG layer is new behavior
that takes precedence over the home-fallback layer.

### Migration

`internal/naming/migrate.go::MigrateLegacyStateDirs()` runs once per
process from `rootCmd.PersistentPreRun`:

1. Read the destination (`RaiozStateDir()`). If it already has
   contents, skip — either nothing to migrate, or a previous run
   already did this.
2. For each entry returned by `LegacyStateDirs()`
   (`/opt/raioz-proyecto`, `~/.raioz`, `~/.raioz-data`), copy every
   file/subdir into the destination preserving structure.
3. Existing files at the destination are NOT overwritten — the new
   location wins.
4. Write a breadcrumb file (`.raioz-migrated-to-xdg`) into the legacy
   dir so subsequent runs skip it without rescanning.

Migration is best-effort: any failure is logged at debug level and
does not block the command. Worst case the user re-runs and the
remaining files migrate on the next attempt; the breadcrumb file
ensures progress.

## Implementation status

Landed in this commit:

- `internal/naming/paths.go`: `RaiozStateDir`, `RaiozConfigDir`,
  `LegacyStateDirs`.
- `internal/naming/migrate.go`: `MigrateLegacyStateDirs`, `copyTree`,
  `copyFile`, breadcrumb constant.
- `internal/naming/paths_test.go`: env-var precedence + migration
  happy path + dst-wins guard.
- `internal/audit/audit.go`: `getBaseDirForAuditLog` deleted;
  `GetAuditLogPath` uses `naming.RaiozStateDir()`.
- `internal/ignore/ignore.go`: `getBaseDirForIgnore` deleted;
  `GetIgnorePath` uses `naming.RaiozStateDir()`.
- `internal/workspace/workspace.go`: `getFallbackBaseDir` deleted,
  `GetBaseDir` reduced to a `naming.RaiozStateDir()` call.
- `internal/workspace/workspace_test.go`: refactored tests against
  the new precedence model (RAIOZ_HOME > XDG > home fallback).
- `internal/cli/root.go`: `PersistentPreRun` calls
  `naming.MigrateLegacyStateDirs()` and logs notes at debug level.

## Consequences

### Positive

- One place to read the location policy. Future changes (deprecating
  `RAIOZ_HOME`, adding a `--state-dir` flag, splitting state into
  per-version subdirs) happen in `naming/paths.go` and cascade.
- Audit/ignore/workspace can no longer drift.
- XDG-conformant default. Tidy-home contributors get the right
  behavior out of the box without exporting `RAIOZ_HOME`.
- Existing users with data in `~/.raioz` or `/opt/raioz-proyecto`
  upgrade transparently — the migrator runs on the first command.
- Removed three near-identical functions (~120 LoC) and the
  `runtime`/`os/user` imports that each one carried.

### Negative

- The legacy `/opt/raioz-proyecto` path is no longer the preferred
  location. Users who relied on `/opt` for cross-user state on a
  shared box must export `RAIOZ_HOME=/opt/raioz-proyecto` explicitly.
  Documented in the migration note printed at debug level; an entry
  in CHANGELOG.md captures the visible behavior change.
- Migration walks the legacy directories on every cold start. The
  fast path (destination already populated) is one `os.ReadDir`, so
  the cost is negligible. Slow path runs once per machine.

### Neutral

- `RaiozConfigDir()` is introduced but no existing caller migrates to
  it in this commit — the helper is there for a future ADR that
  separates user preferences (language, defaults) from runtime state.

## Alternatives considered

- **Push the helper into `internal/path/`.** `path` already exists
  and arguably owns "where does this thing live." Rejected: `path`
  is for project-local path resolution (compose overlays, workspace
  trees). The state helper is a process-global concern; it belongs
  next to other process-global naming policies in `internal/naming/`.
- **Keep the three implementations and just unify the algorithm.**
  Cheaper to write but doesn't prevent the next drift. The whole
  failure mode is "future contributor changes one of three."
- **Stick with `/opt/raioz-proyecto` as the preferred root.**
  Convenient for the single-Linux-developer case. Rejected: cross
  platform, requires root or a one-time chmod, and contributors who
  set XDG paths see raioz ignore them.
- **Run the migration only on `raioz init`.** Less invasive but
  leaves users who upgrade and run `raioz up` first with a fresh
  empty state dir, losing access to their old audit log and ignore
  list until they happen to run init. The bug report would be
  "raioz forgot my settings after upgrade."

## References

- Code: `internal/naming/paths.go`, `internal/naming/migrate.go`,
  `internal/audit/audit.go`, `internal/ignore/ignore.go`,
  `internal/workspace/workspace.go`, `internal/cli/root.go`.
- Tests: `internal/naming/paths_test.go`,
  `internal/workspace/workspace_test.go`.
- State-file map (what lives under `RaiozStateDir`):
  [docs/STATE.md](../STATE.md).
- Issue: 042.
