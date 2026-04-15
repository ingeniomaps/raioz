package host

import (
	"os/exec"
	"testing"
	"time"
)

func TestKillProcessTree_BadPID(t *testing.T) {
	if err := KillProcessTree(0); err != nil {
		t.Errorf("pid=0 should be a no-op, got %v", err)
	}
	if err := KillProcessTree(-1); err != nil {
		t.Errorf("pid=-1 should be a no-op, got %v", err)
	}
}

func TestForceKillProcessTree_BadPID(t *testing.T) {
	if err := ForceKillProcessTree(0); err != nil {
		t.Errorf("pid=0 should be a no-op, got %v", err)
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	if IsProcessAlive(0) {
		t.Error("pid=0 should never be alive")
	}
	if IsProcessAlive(-1) {
		t.Error("negative pid should never be alive")
	}
}

func TestSetNewProcessGroup_NoPanicOnExecCmd(t *testing.T) {
	cmd := exec.Command("true")
	SetNewProcessGroup(cmd)
	// On unix this sets SysProcAttr; on windows it's a no-op. Both must
	// leave cmd usable.
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = cmd.Wait()
}

func TestKillProcessTree_RealChild(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("sleep not available: %v", err)
	}
	pid := cmd.Process.Pid

	if err := KillProcessTree(pid); err != nil {
		t.Fatalf("KillProcessTree: %v", err)
	}

	// Reap so a zombie doesn't leave Signal(0) reporting alive forever.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = ForceKillProcessTree(pid)
		<-done
		t.Errorf("sleep didn't exit within 2s of KillProcessTree")
	}
}
