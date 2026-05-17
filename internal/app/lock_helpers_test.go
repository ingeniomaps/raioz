package app

import (
	"context"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/protocol"
	"raioz/internal/workspace"
)

// Regression: when RAIOZ_SIBLING_STACK is set (we are running as a
// mode-A recursive spawn, ADR-008), the parent already holds the
// workspace lock. acquireWorkspaceMutatorLock MUST skip acquisition
// or it deadlocks against the parent. The first cut of this helper
// missed the guard (see the third architecture-review pass on the
// v0.9 resolution arc); this test pins the fix and locks in
// agreement with upcase.acquireLock's behaviour.
func TestAcquireWorkspaceMutatorLock_SkipsUnderSiblingSpawn(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "/parent/project")

	called := false
	deps := &Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(string) (*workspace.Workspace, error) {
				return &workspace.Workspace{Root: t.TempDir()}, nil
			},
		},
		LockManager: &mocks.MockLockManager{
			AcquireFunc: func(*workspace.Workspace) (interfaces.Lock, error) {
				called = true
				return &mocks.MockLock{}, nil
			},
		},
	}

	release, err := acquireWorkspaceMutatorLock(
		context.Background(), deps, "any-project", "test",
	)
	if err != nil {
		t.Fatalf("acquireWorkspaceMutatorLock: %v", err)
	}
	if called {
		t.Error("LockManager.Acquire MUST NOT be called under " +
			"RAIOZ_SIBLING_STACK (would deadlock against parent)")
	}
	// Release must be a safe no-op too.
	release()
}

// Sanity: outside a sibling spawn, the helper does call Acquire.
func TestAcquireWorkspaceMutatorLock_AcquiresWhenNotSiblingSpawn(t *testing.T) {
	// Ensure clean env (the previous test might have leaked via parallel runs).
	t.Setenv(protocol.SiblingStack, "")

	called := false
	deps := &Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(string) (*workspace.Workspace, error) {
				return &workspace.Workspace{Root: t.TempDir()}, nil
			},
		},
		LockManager: &mocks.MockLockManager{
			AcquireFunc: func(*workspace.Workspace) (interfaces.Lock, error) {
				called = true
				return &mocks.MockLock{}, nil
			},
		},
	}

	release, err := acquireWorkspaceMutatorLock(
		context.Background(), deps, "any-project", "test",
	)
	if err != nil {
		t.Fatalf("acquireWorkspaceMutatorLock: %v", err)
	}
	if !called {
		t.Error("LockManager.Acquire must be called in the normal path")
	}
	release()
}
