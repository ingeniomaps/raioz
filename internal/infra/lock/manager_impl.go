package lock

import (
	"raioz/internal/domain/interfaces"
	lockpkg "raioz/internal/lock"
	workspacepkg "raioz/internal/workspace"
)

// Ensure LockManagerImpl implements interfaces.LockManager
var _ interfaces.LockManager = (*LockManagerImpl)(nil)

// Ensure LockImpl implements interfaces.Lock
var _ interfaces.Lock = (*LockImpl)(nil)

// LockManagerImpl is the concrete implementation of LockManager
type LockManagerImpl struct{}

// NewLockManager creates a new LockManager implementation
func NewLockManager() interfaces.LockManager {
	return &LockManagerImpl{}
}

// Acquire acquires a lock for the workspace
func (m *LockManagerImpl) Acquire(ws *interfaces.Workspace) (interfaces.Lock, error) {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	lockInstance, err := lockpkg.Acquire(wsConcrete)
	if err != nil {
		return nil, err
	}
	return &LockImpl{lock: lockInstance}, nil
}

// LockImpl wraps the concrete lock implementation
type LockImpl struct {
	lock *lockpkg.Lock
}

// Release releases the lock
func (l *LockImpl) Release() error {
	return l.lock.Release()
}
