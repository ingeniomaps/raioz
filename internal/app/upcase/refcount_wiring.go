package upcase

import (
	"context"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/refcount"
)

// registerSharedDepRefs records this project's reference to each shared
// dependency it just dispatched, so `raioz down` can decide whether it is
// the last consumer (issue 069). Only shared deps are tracked — a
// per-project dep has exactly one owner and is always torn down by its own
// `down`, so a refcount would be noise.
//
// Failures are logged, not fatal: a refcount write that fails must not
// abort a successful `up`. The down path reconciles against live
// containers, so a missing ref degrades to the old best-effort behavior
// rather than corrupting state.
func registerSharedDepRefs(ctx context.Context, deps *models.Deps, dispatched []string) {
	for _, name := range dispatched {
		var override string
		if entry, ok := deps.Infra[name]; ok && entry.Inline != nil {
			override = entry.Inline.Name
		}
		if !naming.IsSharedDep(override) {
			continue
		}
		if err := refcount.AddRef(deps.Workspace, name, deps.Project.Name); err != nil {
			logging.WarnWithContext(ctx, "Failed to record shared dep reference",
				"dep", name, "workspace", deps.Workspace,
				"project", deps.Project.Name, "error", err.Error())
		}
	}
}
