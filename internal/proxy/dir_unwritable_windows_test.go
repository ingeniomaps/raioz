//go:build windows

package proxy

import (
	"os"
	"os/exec"
	"testing"
)

// makeDirUnwritable denies the current user the Write right on dir via
// icacls. POSIX chmod bits don't gate the owner on NTFS, so we have to
// punch a real deny ACE instead. Cleanup removes the deny ACE so
// t.TempDir's recursive delete works afterwards.
func makeDirUnwritable(t *testing.T, dir string) {
	t.Helper()
	user := os.Getenv("USERNAME")
	if user == "" {
		t.Skip("USERNAME env unavailable; can't author deny ACE")
	}
	cmd := exec.Command("icacls", dir, "/deny", user+":(W)")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("icacls deny: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		// /remove:d wipes any deny ACE for the user. Best-effort —
		// tempdir cleanup proceeds regardless.
		_ = exec.Command("icacls", dir, "/remove:d", user).Run()
	})
}

// skipIfPrivilegedWriter is a no-op on Windows: there's no equivalent
// of root that bypasses an icacls deny ACE for the very user the ACE
// is bound to.
func skipIfPrivilegedWriter(_ *testing.T) {}
