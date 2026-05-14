package upcase

import (
	"context"
	"time"

	"raioz/internal/audit"
	"raioz/internal/domain/models"
	"raioz/internal/logging"
)

// emitLifecycleStart audits the moment `raioz up` enters real work
// (after bootstrap resolved deps + workspace). Best-effort: an audit
// error must never abort the up — logged at debug only.
func emitLifecycleStart(ctx context.Context, deps *models.Deps) {
	if deps == nil {
		return
	}
	if err := audit.LogLifecycleStart(
		ctx, "up", deps.Project.Name, deps.Workspace,
	); err != nil {
		logging.DebugWithContext(ctx, "audit LogLifecycleStart failed",
			"error", err.Error())
	}
}

// emitLifecycleComplete pairs with emitLifecycleStart and runs in a
// deferred close so every return path of Execute closes the audit
// record. err can be nil on success.
func emitLifecycleComplete(
	ctx context.Context, deps *models.Deps, start time.Time, err error,
) {
	if deps == nil {
		return
	}
	status := "success"
	if err != nil {
		status = "failure"
	}
	if auditErr := audit.LogLifecycleComplete(
		ctx, "up", deps.Project.Name, deps.Workspace,
		status, time.Since(start), err,
	); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogLifecycleComplete failed",
			"error", auditErr.Error())
	}
}
