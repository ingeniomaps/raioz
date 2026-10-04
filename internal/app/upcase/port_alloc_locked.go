package upcase

import (
	"context"
	"path/filepath"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
)

// allocatePortsLocked wraps port allocation + bind-conflict resolution
// in the global ports flock and releases it before returning. A wider
// scope (`defer` at processOrchestration level) would deadlock a
// `project:`-spawned recursive `raioz up`: flock is per-fd and the
// child process inherits none of the parent's locks.
func allocatePortsLocked(
	ctx context.Context,
	deps *models.Deps,
	detections DetectionMap,
	configPath string,
) (*PortAllocResult, error) {
	release, err := acquirePortsLock()
	if err != nil {
		return nil, err
	}
	defer release()

	portAllocs, err := AllocateHostPortsOwned(deps, detections, runningHostPorts(projectDirOf(configPath)))
	if err != nil {
		return nil, err
	}

	reuseRunningDepHostPorts(ctx, deps, portAllocs)

	if conflicts := checkPortBindConflicts(portAllocs); len(conflicts) > 0 {
		if err := resolvePortBindConflicts(
			ctx, conflicts, portAllocs, configPath, deps, naming.WorkspaceName(),
		); err != nil {
			return nil, err
		}
	}
	return portAllocs, nil
}

// reuseRunningDepHostPorts keeps an auto-published dependency on the host
// port its running container already publishes. With `publish: true` the
// pure allocator picks the first free host port — and finds the one the
// dependency itself holds busy, so it bumps (6379 → 6380) and raioz then
// injects a <DEP>_URL pointing at a port nobody serves. That happened to
// the 2nd project of a workspace sharing the dep, to a repeated up, and to
// every env recompute (restart, watch, `raioz env`).
//
// Here we look up the port the container actually publishes and pin the
// allocation to it. Only auto-published deps are touched: explicit pins
// stay sacred, and a dep that is not running falls back to the normal
// allocation untouched (GetPublishedHostPort returns 0). The downstream
// bind check sees the port as busy but resolvePortBindConflicts recognizes
// it as our own container and reuses it.
func reuseRunningDepHostPorts(ctx context.Context, deps *models.Deps, result *PortAllocResult) {
	for name, alloc := range result.Deps {
		if alloc.Explicit {
			continue // user pinned the host port — never rewrite it
		}
		entry, ok := deps.Infra[name]
		if !ok || entry.Inline == nil {
			continue
		}
		container := naming.DepContainer(deps.Project.Name, name, entry.Inline.Name)

		changed := false
		for i, m := range alloc.Mappings {
			live, err := publishedHostPortFn(ctx, container, m.ContainerPort)
			if err != nil || live <= 0 || live == m.HostPort {
				continue
			}
			logging.Debug("reusing running dep host port",
				"dep", name, "container", container,
				"containerPort", m.ContainerPort, "from", m.HostPort, "to", live)
			alloc.Mappings[i].HostPort = live
			changed = true
		}
		if changed {
			result.Deps[name] = alloc
		}
	}
}

// projectDirOf returns the directory holding the config, "" when unknown.
func projectDirOf(configPath string) string {
	if configPath == "" {
		return ""
	}
	dir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return ""
	}
	return dir
}
