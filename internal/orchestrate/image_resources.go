package orchestrate

import (
	"context"
	"os/exec"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/runtime"
)

// applyResourceLimits writes a dependency's `resources:` block into its
// generated compose service. `memswap_limit` equal to `mem_limit` is what
// makes the memory figure a ceiling: without it the container may use as
// much swap again.
func applyResourceLimits(service map[string]any, res *models.Resources) {
	if res.IsZero() {
		return
	}
	if res.Memory != "" {
		service["mem_limit"] = res.Memory
		service["memswap_limit"] = res.Memory
	}
	if res.CPUs > 0 {
		service["cpus"] = res.CPUs
	}
}

// limitUpdateArgs is the `docker update` argument list that applies res to
// a running container, or nil when there is nothing to apply.
func limitUpdateArgs(container string, res *models.Resources) []string {
	if res.IsZero() {
		return nil
	}
	args := []string{"update"}
	if res.Memory != "" {
		args = append(args, "--memory", res.Memory, "--memory-swap", res.Memory)
	}
	if res.CPUs > 0 {
		args = append(args, "--cpus", res.CPUsString())
	}
	return append(args, container)
}

// updateRunningLimits applies a dependency's cap to its container when
// `up` finds it already running and leaves it as is. Without this, a
// `resources:` block added to raioz.yaml would not take effect until the
// dependency was next recreated. Best-effort: the dependency keeps
// serving either way.
func updateRunningLimits(ctx context.Context, container string, res *models.Resources) {
	args := limitUpdateArgs(container, res)
	if args == nil {
		return
	}
	if out, err := exec.CommandContext(ctx, runtime.Binary(), args...).CombinedOutput(); err != nil {
		logging.WarnWithContext(ctx, "Could not apply resource limits to the running dependency",
			"container", container, "error", err.Error(), "output", string(out))
	}
}
