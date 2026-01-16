package upcase

import (
	"context"

	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/lock"
	"raioz/internal/logging"
)

// LockInstance wraps a lock instance for deferred release
type LockInstance struct {
	lock *lock.Lock
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
		// Only log at debug level - not useful for end users
		logging.DebugWithContext(l.ctx, "Lock released")
	}
	return nil
}

// acquireLock acquires a lock on the workspace
func (uc *UseCase) acquireLock(ctx context.Context, ws *interfaces.Workspace) (*LockInstance, error) {
	// Only log at debug level - not useful for end users
	logging.DebugWithContext(ctx, "Acquiring lock", "workspace", ws.Root)
	lockInstance, err := lock.Acquire(ws)
	if err != nil {
		logging.ErrorWithContext(ctx, "Failed to acquire lock", "workspace", ws.Root, "error", err.Error())
		return nil, errors.New(errors.ErrCodeLockError, "Failed to acquire lock (another raioz process may be running)").WithSuggestion("Wait for the other process to finish, "+"or remove the lock file manually if the process crashed.").WithContext("workspace", ws.Root).WithError(err)
	}
	// Only log at debug level - not useful for end users
	logging.DebugWithContext(ctx, "Lock acquired successfully")
	return &LockInstance{lock: lockInstance, ctx: ctx, ws: ws}, nil
}
