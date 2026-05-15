//go:build linux

package host

import (
	"os/exec"
	"syscall"
	"testing"
)

// AttachPdeathsig must wire Pdeathsig = SIGTERM on Linux so the
// kernel reaps the spawn when the parent dies. ADR-026.
func TestAttachPdeathsig_Linux(t *testing.T) {
	cmd := exec.Command("true")
	AttachPdeathsig(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("AttachPdeathsig left SysProcAttr nil")
	}
	if cmd.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Errorf("Pdeathsig = %v, want SIGTERM", cmd.SysProcAttr.Pdeathsig)
	}
}

// Pre-existing SysProcAttr must be preserved, not stomped — callers
// may have set unrelated fields before calling.
func TestAttachPdeathsig_PreservesExisting(t *testing.T) {
	cmd := exec.Command("true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	AttachPdeathsig(cmd)
	if !cmd.SysProcAttr.Setpgid {
		t.Error("AttachPdeathsig clobbered Setpgid")
	}
	if cmd.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Errorf("Pdeathsig = %v, want SIGTERM", cmd.SysProcAttr.Pdeathsig)
	}
}
