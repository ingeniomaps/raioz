package upcase

import (
	"context"

	"raioz/internal/domain/models"
)

// allocatePortsLocked wraps port allocation + bind-conflict resolution
// in the global ports flock and releases the lock immediately afterward.
// Splitting this out of processOrchestration keeps the lock scope tight
// — `defer release()` in processOrchestration would hold it across the
// sibling dispatch phase, deadlocking a `project:`-spawned recursive
// `raioz up` that tries to acquire the same flock (issue 020-meta).
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

	if conflicts := checkPortBindConflicts(portAllocs); len(conflicts) > 0 {
		if err := resolvePortBindConflicts(
			ctx, conflicts, portAllocs, configPath, deps.Project.Name,
		); err != nil {
			return nil, err
		}
	}
	return portAllocs, nil
}
