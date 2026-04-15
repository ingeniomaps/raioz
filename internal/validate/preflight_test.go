package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWorkspacePermissions_WritableTempDir(t *testing.T) {
	// t.TempDir() is writable — should succeed, and path may or may not exist
	dir := t.TempDir()
	target := filepath.Join(dir, "ws")
	if err := CheckWorkspacePermissions(target); err != nil {
		t.Errorf("CheckWorkspacePermissions(%s) unexpected error: %v", target, err)
	}
	// Confirm directory exists after call
	if _, err := os.Stat(target); err != nil {
		t.Errorf("expected workspace dir to exist, stat error: %v", err)
	}
}

func TestCheckWorkspacePermissions_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	if err := CheckWorkspacePermissions(dir); err != nil {
		t.Errorf("CheckWorkspacePermissions(existing dir) unexpected error: %v", err)
	}
}

func TestCheckWorkspacePermissions_NonWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, cannot test unwritable dir")
	}
	// Create a read-only parent so MkdirAll fails for child
	parent := t.TempDir()
	readonly := filepath.Join(parent, "ro")
	if err := os.MkdirAll(readonly, 0500); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	defer func() { _ = os.Chmod(readonly, 0700) }()

	target := filepath.Join(readonly, "child", "ws")
	err := CheckWorkspacePermissions(target)
	if err == nil {
		t.Error("expected error for non-writable parent, got nil")
	}
}

func TestCheckDiskSpace_NoPanic(t *testing.T) {
	// Should not panic; in practice returns nil unless disk is extremely full.
	_ = checkDiskSpace()
}

func TestCheckGitInstalled_NoPanic(t *testing.T) {
	// May fail if git not installed, but must not panic.
	_ = checkGitInstalled()
	_ = checkGitInstalledWithContext(context.Background())
}

func TestCheckDockerInstalled_NoPanic(t *testing.T) {
	_ = checkDockerInstalled()
	_ = checkDockerRunning()
	_ = checkDockerInstalledWithContext(context.Background())
	_ = checkDockerRunningWithContext(context.Background())
}

func TestCheckNetworkConnectivity_NoPanic(t *testing.T) {
	_ = checkNetworkConnectivity()
	_ = checkNetworkConnectivityWithContext(context.Background())
}

func TestPreflightCheck_NoPanic(t *testing.T) {
	// Preflight aggregates many checks; it may fail but should not panic.
	_ = PreflightCheck()
	_ = PreflightCheckWithContext(context.Background())
}

func TestPreflightCheck_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Cancelled context should cause errors, not panic
	_ = PreflightCheckWithContext(ctx)
}
