package upcase

import (
	"context"

	"raioz/internal/domain/models"
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

	if conflicts := checkPortBindConflicts(portAllocs); len(conflicts) > 0 {
		if err := resolvePortBindConflicts(
			ctx, conflicts, portAllocs, configPath, deps.Project.Name,
		); err != nil {
			return nil, err
		}
	}
	return portAllocs, nil
}
