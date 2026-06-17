package upcase

import (
	"context"

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

	portAllocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		return nil, err
	}

	reuseSharedDepHostPorts(ctx, deps, portAllocs)

	if conflicts := checkPortBindConflicts(portAllocs); len(conflicts) > 0 {
		if err := resolvePortBindConflicts(
			ctx, conflicts, portAllocs, configPath, deps, naming.WorkspaceName(),
		); err != nil {
			return nil, err
		}
	}
	return portAllocs, nil
}

// reuseSharedDepHostPorts keeps a workspace-shared dependency on a single host
// port across every project. With `publish: true`, the pure allocator picks a
// free host port per project — so the 2nd project to come up sees the shared
// container's port as "busy elsewhere" and bumps it (6379 → 6380), then injects
// a divergent <DEP>_URL that points at a port nobody serves (issue 020 b).
//
// Here we look up the port the shared container actually publishes and pin the
// allocation to it. Only auto-published shared deps are touched: explicit pins
// stay sacred, and a non-workspace / not-yet-running dep falls back to the
// normal allocation untouched (GetPublishedHostPort returns 0). The downstream
// bind check sees the port as busy but resolvePortBindConflicts recognizes it
// as our own shared dep and reuses it (fix a).
func reuseSharedDepHostPorts(ctx context.Context, deps *models.Deps, result *PortAllocResult) {
	for name, alloc := range result.Deps {
		if alloc.Explicit {
			continue // user pinned the host port — never rewrite it
		}
		entry, ok := deps.Infra[name]
		if !ok || entry.Inline == nil {
			continue
		}
		override := entry.Inline.Name
		if !naming.IsSharedDep(override) {
			continue // per-project dep — no cross-project sharing to honor
		}
		container := naming.DepContainer(deps.Project.Name, name, override)

		changed := false
		for i, m := range alloc.Mappings {
			live, err := publishedHostPortFn(ctx, container, m.ContainerPort)
			if err != nil || live <= 0 || live == m.HostPort {
				continue
			}
			logging.Debug("reusing shared dep host port",
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
