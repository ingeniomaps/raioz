package upcase

import (
	"context"

	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
)

// LockInstance wraps a lock instance for deferred release
type LockInstance struct {
	lock interfaces.Lock
	ctx  context.Context
	ws   *interfaces.Workspace
}

// Release releases the lock
func (l *LockInstance) Release() error {
	if l.lock != nil {
		if err := l.lock.Release(); err != nil {
			logging.ErrorWithContext(l.ctx, "Failed to release lock", "error", err.Error())
			return err
		}
		logging.DebugWithContext(l.ctx, "Lock released")
	}
	return nil
}

// acquireLock acquires a lock on the workspace
func (uc *UseCase) acquireLock(ctx context.Context, ws *interfaces.Workspace) (*LockInstance, error) {
	logging.DebugWithContext(ctx, "Acquiring lock", "workspace", ws.Root)
	lockInstance, err := uc.deps.LockManager.Acquire(ws)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to acquire lock", "workspace", ws.Root, "error", err.Error())
		return nil, errors.New(errors.ErrCodeLockError, i18n.T("error.lock_failed")).WithSuggestion(i18n.T("error.lock_suggestion")).WithContext("workspace", ws.Root).WithError(err)
	}
	logging.DebugWithContext(ctx, "Lock acquired successfully")
	return &LockInstance{lock: lockInstance, ctx: ctx, ws: ws}, nil
}
