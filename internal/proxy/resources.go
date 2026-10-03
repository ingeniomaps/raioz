package proxy

import (
	"context"
	"os/exec"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/runtime"
)

// resourceArgs turns a `resources:` block into the docker flags that cap a
// container. `--memory-swap` equal to `--memory` is what makes the memory
// figure a ceiling: without it Docker lets the container use as much swap
// again.
func resourceArgs(res *models.Resources) []string {
	if res.IsZero() {
		return nil
	}
	var args []string
	if res.Memory != "" {
		args = append(args, "--memory", res.Memory, "--memory-swap", res.Memory)
	}
	if res.CPUs > 0 {
		args = append(args, "--cpus", res.CPUsString())
	}
	return args
}

// updateResourceLimits applies the declared cap to a proxy that is already
// running. Without it a `resources:` block added to raioz.yaml would wait
// for the proxy's next recreate, which for a workspace-shared proxy can be
// days away. Best-effort: a proxy without the cap still routes.
func (m *Manager) updateResourceLimits(ctx context.Context, containerName string) {
	limits := resourceArgs(m.resources)
	if len(limits) == 0 {
		return
	}
	args := append([]string{"update"}, limits...)
	args = append(args, containerName)
	if out, err := exec.CommandContext(ctx, runtime.Binary(), args...).CombinedOutput(); err != nil {
		logging.WarnWithContext(ctx, "Could not apply resource limits to the running proxy",
			"container", containerName, "error", err.Error(), "output", string(out))
	}
}
