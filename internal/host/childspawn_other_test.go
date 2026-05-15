//go:build !linux

package host

import (
	"os/exec"
	"testing"
)

// On non-Linux the call must initialize SysProcAttr (so subsequent
// platform-specific fields can be set) without panicking.
func TestAttachPdeathsig_NoopSafe(t *testing.T) {
	cmd := exec.Command("true")
	AttachPdeathsig(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("AttachPdeathsig left SysProcAttr nil")
	}
}
