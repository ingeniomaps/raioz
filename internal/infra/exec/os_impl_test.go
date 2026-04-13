package exec

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOSCommandExecutor_Command_Run(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Errorf("true should succeed, got %v", err)
	}
}

func TestOSCommandExecutor_Command_RunFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	cmd := exec.Command("false")
	if err := cmd.Run(); err == nil {
		t.Error("false should fail")
	}
}

func TestOSCommandExecutor_Output(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	cmd := exec.Command("echo", "hello")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("expected 'hello' in output, got %q", out)
	}
}

func TestOSCommandExecutor_CombinedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	cmd := exec.Command("sh", "-c", "echo stdout; echo stderr >&2")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CombinedOutput failed: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "stdout") || !strings.Contains(s, "stderr") {
		t.Errorf("expected both streams in output, got %q", s)
	}
}

func TestOSCommandExecutor_CommandContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sleep", "10")
	err := cmd.Run()
	if err == nil {
		t.Error("expected context to kill sleep")
	}
}

func TestOSCommandExecutor_StartAndWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("Wait failed: %v", err)
	}
}

func TestOSCommandExecutor_SetDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	dir := t.TempDir()
	cmd := exec.Command("pwd")
	cmd.SetDir(dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	// macOS tends to prefix /private to temp paths — tolerate that
	if !strings.HasSuffix(got, dir) {
		t.Errorf("expected suffix %q in pwd, got %q", dir, got)
	}
}

func TestOSCommandExecutor_SetStdoutStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("sh", "-c", "echo out; echo err >&2")
	cmd.SetStdout(&stdout)
	cmd.SetStderr(&stderr)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "out") {
		t.Errorf("stdout missing: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "err") {
		t.Errorf("stderr missing: %q", stderr.String())
	}
}

func TestOSCommandExecutor_SetStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}
	exec := NewOSCommandExecutor()
	cmd := exec.Command("cat")
	cmd.SetStdin(strings.NewReader("stdin-content"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if !strings.Contains(string(out), "stdin-content") {
		t.Errorf("expected stdin echoed, got %q", out)
	}
}
