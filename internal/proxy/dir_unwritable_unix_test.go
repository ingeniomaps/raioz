//go:build !windows

package proxy

import (
	"os"
	"testing"
)

// makeDirUnwritable removes write permission from dir so the next
// CreateTemp / WriteFile under it fails. Unix-flavored — drops POSIX
// mode bits. Cleanup restores 0o755 so t.TempDir's recursive delete
// can still nuke the tree.
func makeDirUnwritable(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// skipIfPrivilegedWriter skips when the current user can write anywhere
// regardless of POSIX mode (root). On Windows there's no equivalent
// blanket bypass for icacls deny ACEs, so the variant is a no-op there.
func skipIfPrivilegedWriter(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root can write everywhere — skip on root runners")
	}
}
