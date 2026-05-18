//go:build windows

package exec

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// Windows variant of the OSCommandExecutor suite. The wrapper itself
// is OS-agnostic (it forwards every method to os/exec.Cmd), so the
// tests just substitute equivalent commands available via cmd.exe and
// run the same assertions as the Unix file. ADR-030 wires the Windows
// runtime CI gate that exercises this package on real Windows.

func TestOSCommandExecutor_Command_Run(t *testing.T) {
	exec := NewOSCommandExecutor()
	cmd := exec.Command("cmd", "/c", "exit", "0")
	if err := cmd.Run(); err != nil {
		t.Errorf("exit 0 should succeed, got %v", err)
	}
}

func TestOSCommandExecutor_Command_RunFailure(t *testing.T) {
	exec := NewOSCommandExecutor()
	cmd := exec.Command("cmd", "/c", "exit", "1")
	if err := cmd.Run(); err == nil {
		t.Error("exit 1 should fail")
	}
}

func TestOSCommandExecutor_Output(t *testing.T) {
	exec := NewOSCommandExecutor()
	cmd := exec.Command("cmd", "/c", "echo", "hello")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("expected 'hello' in output, got %q", out)
	}
}

func TestOSCommandExecutor_CombinedOutput(t *testing.T) {
	exec := NewOSCommandExecutor()
	// `&` chains commands in cmd.exe; `1>&2` redirects to stderr.
	cmd := exec.Command("cmd", "/c", "echo stdout & echo stderr 1>&2")
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
	exec := NewOSCommandExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// `timeout` honors /nobreak but still respects ctx cancel via
	// the kill signal; the standard portable sleep on Windows.
	cmd := exec.CommandContext(ctx, "cmd", "/c", "timeout", "/t", "10", "/nobreak")
	err := cmd.Run()
	if err == nil {
		t.Error("expected context to kill timeout")
	}
}

func TestOSCommandExecutor_StartAndWait(t *testing.T) {
	exec := NewOSCommandExecutor()
	cmd := exec.Command("cmd", "/c", "exit", "0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("Wait failed: %v", err)
	}
}

func TestOSCommandExecutor_SetDir(t *testing.T) {
	exec := NewOSCommandExecutor()
	dir := t.TempDir()
	cmd := exec.Command("cmd", "/c", "cd")
	cmd.SetDir(dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	// Compare case-insensitively — Windows tempdir paths normalize the
	// drive letter case differently across versions.
	if !strings.EqualFold(got, dir) {
		t.Errorf("expected pwd %q, got %q", dir, got)
	}
}

func TestOSCommandExecutor_SetStdoutStderr(t *testing.T) {
	exec := NewOSCommandExecutor()
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("cmd", "/c", "echo out & echo err 1>&2")
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
	exec := NewOSCommandExecutor()
	// `findstr "x*"` matches any line (the * makes x optional), so
	// every stdin line is echoed back — Windows' cat-equivalent for
	// stdin testing.
	cmd := exec.Command("cmd", "/c", "findstr", `"x*"`)
	cmd.SetStdin(strings.NewReader("stdin-content"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if !strings.Contains(string(out), "stdin-content") {
		t.Errorf("expected stdin echoed, got %q", out)
	}
}
