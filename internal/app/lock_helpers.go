package app

import (
	"context"
	"fmt"

	"raioz/internal/logging"
	"raioz/internal/protocol"
)

// acquireRestartLock acquires the workspace lock for `raioz restart`.
// State mutators (restart, dev, down --service) must serialize against
// each other and against `raioz up --watch` save-state ticks. Issue
// 038 / CLAUDE.md "State writers acquire the workspace lock".
//
// Returns a release func the caller must defer; release logs on
// failure but never returns the error since the caller is already
// returning from a different path.
func acquireRestartLock(
	ctx context.Context, deps *Dependencies, projectName string,
) (func(), error) {
	return acquireWorkspaceMutatorLock(ctx, deps, projectName, "restart")
}

// acquireDownSelectiveLock — same as acquireRestartLock for the
// selective-down path (`raioz down <service>`).
func acquireDownSelectiveLock(
	ctx context.Context, deps *Dependencies, projectName string,
) (func(), error) {
	return acquireWorkspaceMutatorLock(ctx, deps, projectName, "down-selective")
}

// acquireWorkspaceMutatorLock is the shared body. The `label` is only
// used in the debug log line so concurrent lock acquisitions are
// distinguishable in audit forensics.
func acquireWorkspaceMutatorLock(
	ctx context.Context, deps *Dependencies, projectName, label string,
) (func(), error) {
	// Recursive sibling spawn: the parent already holds the workspace
	// lock; re-acquiring deadlocks. Mirrors upcase.acquireLock — the
	// two acquirer surfaces MUST agree, see protocol.IsRecursiveSiblingSpawn.
	if protocol.IsRecursiveSiblingSpawn() {
		logging.DebugWithContext(ctx,
			label+": skipping workspace lock (recursive sibling spawn)")
		return func() {}, nil
	}
	ws, err := deps.Workspace.Resolve(projectName)
	if err != nil {
		return func() {}, fmt.Errorf("%s: resolve workspace: %w", label, err)
	}
	if ws == nil {
		// Test / no-workspace path — skip the lock.
		return func() {}, nil
	}
	lock, err := deps.LockManager.Acquire(ws)
	if err != nil {
		return func() {}, fmt.Errorf("%s: acquire workspace lock: %w", label, err)
	}
	logging.DebugWithContext(ctx, label+": workspace lock acquired",
		"workspace", ws.Root)
	return func() {
		if err := lock.Release(); err != nil {
			logging.WarnWithContext(ctx,
				label+": failed to release workspace lock",
				"error", err.Error())
		}
	}, nil
}
