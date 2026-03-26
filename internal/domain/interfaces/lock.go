package interfaces

// LockManager defines operations for managing locks
type LockManager interface {
	// Acquire acquires a lock for the workspace
	Acquire(ws *Workspace) (Lock, error)
}

// Lock represents a lock instance
type Lock interface {
	// Release releases the lock
	Release() error
}
