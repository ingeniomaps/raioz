package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rerrors "raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/naming"
)

// withPrefix temporarily aligns naming.prefix with the test's workspace
// name. Production code does this via naming.SetPrefix at config-load
// time; tests have to mirror it because naming.WorkspaceProxyDir reads
// the package-level prefix, not the manager's workspaceName field.
func withPrefix(t *testing.T, p string) {
	t.Helper()
	naming.SetPrefix(p)
	t.Cleanup(func() { naming.SetPrefix("") })
}

// TestMain initializes i18n once for the whole package — assertProxyDirWritable
// formats its error via i18n.T, and an uninitialized catalog returns the key
// literally, breaking suggestion-content assertions.
func TestMain(m *testing.M) {
	i18n.Init("en")
	os.Exit(m.Run())
}

// TestAssertProxyDirWritable_HappyPath sets up a workspace-shared manager
// pointing at a fresh tempdir and expects no error. This is the baseline
// — the issue 015 trap only fires when the dir already exists in a
// poisoned state.
func TestAssertProxyDirWritable_HappyPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	withPrefix(t, "acme")

	m := &Manager{workspaceName: "acme"}
	if err := m.assertProxyDirWritable(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestAssertProxyDirWritable_CaddyfileIsDirectory simulates the exact
// failure mode from issue 015: Docker auto-created the bind-mount source
// as a directory. The check must surface a structured error pointing at
// the corrupt path.
func TestAssertProxyDirWritable_CaddyfileIsDirectory(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	withPrefix(t, "acme")

	// Reproduce the trap: Caddyfile path exists but is a directory.
	corruptPath := filepath.Join(xdg, "acme", "proxy", "Caddyfile")
	if err := os.MkdirAll(corruptPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	m := &Manager{workspaceName: "acme"}
	err := m.assertProxyDirWritable()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	rerr, ok := err.(*rerrors.RaiozError)
	if !ok {
		t.Fatalf("expected *RaiozError, got %T", err)
	}
	if rerr.Code != rerrors.ErrCodePermissionDenied {
		t.Errorf("got code %q, want %q", rerr.Code, rerrors.ErrCodePermissionDenied)
	}
	if !strings.Contains(rerr.Suggestion, "sudo rm -rf") {
		t.Errorf("suggestion should mention sudo rm -rf, got: %s", rerr.Suggestion)
	}
	if !strings.Contains(rerr.Suggestion, filepath.Join(xdg, "acme", "proxy")) {
		t.Errorf("suggestion should include the actual dir path, got: %s", rerr.Suggestion)
	}
}

// TestAssertProxyDirWritable_DirReadOnly covers the second half of the
// trap: even when Caddyfile doesn't exist, a parent dir without write
// permission blocks the WriteFile we'd issue. The probe-based check
// catches this whereas a plain Stat would not.
func TestAssertProxyDirWritable_DirReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write everywhere — skip on root runners")
	}
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	withPrefix(t, "acme")

	dir := filepath.Join(xdg, "acme", "proxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("setup chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	m := &Manager{workspaceName: "acme"}
	if err := m.assertProxyDirWritable(); err == nil {
		t.Fatal("expected error on read-only dir, got nil")
	}
}

// TestAssertProxyDirWritable_LegacyMode confirms per-project (non
// workspace-shared) managers exercise the same check via
// naming.ProxyDir(networkName).
func TestAssertProxyDirWritable_LegacyMode(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	withPrefix(t, "raioz")

	m := &Manager{networkName: "billing"} // workspaceName empty → legacy
	if err := m.assertProxyDirWritable(); err != nil {
		t.Fatalf("legacy happy-path: expected nil, got %v", err)
	}
}
