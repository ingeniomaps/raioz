package app

import (
	"context"
	"path/filepath"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/logging"
)

// sweepLauncherOrphans is the post-kill safety net for the launcher pattern.
// Tools like `yarn nx serve` spawn long-lived daemons (nx daemon, vite,
// esbuild) that detach into a new session via setsid before raioz ever
// records a PID — so killing the recorded process group leaves them alive
// and re-parented to init. They keep their original cwd, which is the
// service path raioz launched them from. We resolve that path the same
// way host.StartService does and let host.KillOrphansByCwd send SIGTERM
// to anything still rooted there. Linux-only sweep; macOS/Windows return
// nil and this is a no-op there.
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
	if killed := host.KillOrphansByCwd(abs); len(killed) > 0 {
		logging.InfoWithContext(ctx, "Killed launcher-pattern orphans",
			"service", service, "path", abs, "pids", killed)
	}
}

// absoluteServicePath resolves a service's source.path to an absolute,
// cleaned path. Empty input or a path that doesn't resolve returns "".
// Mirrors host.StartService's resolution rules so the sweep targets the
// same directory the launcher was started from.
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
