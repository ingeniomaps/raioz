package docker

import (
	"context"
	"encoding/json"
	"os/exec"

	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
)

// batchInspect runs a single `docker inspect` for multiple containers and
// returns parsed data keyed by container name.
func batchInspect(ctx context.Context, names []string) map[string]ContainerInspect {
	result := make(map[string]ContainerInspect, len(names))
	if len(names) == 0 {
		return result
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	args := append([]string{"inspect"}, names...)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		return result
	}

	var inspectData []ContainerInspect
	if err := json.Unmarshal(out, &inspectData); err != nil {
		return result
	}

	for i, data := range inspectData {
		if i < len(names) {
			result[names[i]] = data
		}
	}

	return result
}
