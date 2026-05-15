package host

import (
	"os/exec"
	"runtime"
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
	// Pick a long-running command that exists by default on the OS.
	// `sleep` is in PATH on Unix; on Windows it isn't a built-in, so
	// use `ping` with a high count — it's always present on
	// windows-latest runners and responds to CTRL_BREAK_EVENT.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "30", "127.0.0.1")
	} else {
		cmd = exec.Command("sleep", "30")
	}
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("long-running command not available: %v", err)
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
	case <-time.After(3 * time.Second):
		_ = ForceKillProcessTree(pid)
		<-done
		t.Errorf("child didn't exit within 3s of KillProcessTree")
	}
}
