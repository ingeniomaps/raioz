package app

import (
	"context"
	"path/filepath"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/logging"
)

// killOrphansByCwdFn is a package-level hook so tests can simulate the
// launcher-orphan sweep without touching the host's /proc tree.
// Production points at host.KillOrphansByCwd directly.
var killOrphansByCwdFn = host.KillOrphansByCwd

// sweepLauncherOrphans is the post-kill safety net for the launcher pattern.
// Tools like `yarn nx serve` spawn long-lived daemons (nx daemon, vite,
// esbuild) that detach into a new session via setsid before raioz ever
// records a PID — so killing the recorded process group leaves them alive
// and re-parented to init. They keep the cwd raioz launched them from and
// the service marker raioz put in their environment; host.KillOrphansByCwd
// stops whatever matches both, and nothing else that lives in that
// directory. Linux-only sweep; macOS/Windows return nil and this is a
// no-op there.
func sweepLauncherOrphans(ctx context.Context, deps *models.Deps, projectDir, service string) {
	if deps == nil {
		return
	}
	svc, ok := deps.Services[service]
	if !ok {
		return
	}
	abs := absoluteServicePath(projectDir, svc.Source.Path)
	if abs == "" {
		return
	}
	marker := host.ServiceMarker(deps.Project.Name, service)
	if killed := killOrphansByCwdFn(abs, marker); len(killed) > 0 {
		logging.InfoWithContext(ctx, "Killed launcher-pattern orphans",
			"service", service, "path", abs, "pids", killed)
	}
}

// absoluteServicePath resolves a service's source.path to an absolute,
// cleaned path. Empty input or a path that doesn't resolve returns "".
// The same directory the host runner starts the service in.
func absoluteServicePath(projectDir, raw string) string {
	if raw == "" || projectDir == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	if raw == "." {
		return filepath.Clean(projectDir)
	}
	return filepath.Clean(filepath.Join(projectDir, raw))
}
