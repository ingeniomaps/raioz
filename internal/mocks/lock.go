package mocks

import (
	"raioz/internal/domain/interfaces"
	"raioz/internal/workspace"
)

// Compile-time checks
var _ interfaces.LockManager = (*MockLockManager)(nil)
var _ interfaces.Lock = (*MockLock)(nil)

// MockLockManager is a mock implementation of interfaces.LockManager
type MockLockManager struct {
	AcquireFunc func(ws *workspace.Workspace) (interfaces.Lock, error)
}

func (m *MockLockManager) Acquire(ws *workspace.Workspace) (interfaces.Lock, error) {
	if m.AcquireFunc != nil {
		return m.AcquireFunc(ws)
	}
	return &MockLock{}, nil
}

// MockLock is a mock implementation of interfaces.Lock
type MockLock struct {
	ReleaseFunc func() error
}

func (m *MockLock) Release() error {
	if m.ReleaseFunc != nil {
		return m.ReleaseFunc()
	}
	return nil
}
