package upcase

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/logging"
	"raioz/internal/protocol"
)

// --- stack helpers --------------------------------------------------------

func TestReadSiblingStack_Empty(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "")
	if got := readSiblingStack(); got != nil {
		t.Errorf("expected nil for empty env, got %v", got)
	}
}

func TestReadSiblingStack_Populated(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "/a"+string(os.PathListSeparator)+"/b")
	got := readSiblingStack()
	if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Errorf("got %v, want [/a /b]", got)
	}
}

func TestPushSiblingStack_AppendsToExisting(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "/a")
	got := pushSiblingStack("/b", "/c")
	sep := string(os.PathListSeparator)
	want := protocol.SiblingStack + "=/a" + sep + "/b" + sep + "/c"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPushSiblingStack_FirstEntry(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "")
	got := pushSiblingStack("/parent", "/child")
	want := protocol.SiblingStack + "=/parent" + string(os.PathListSeparator) + "/child"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- checkSiblingCycle ----------------------------------------------------

func TestCheckSiblingCycle_NoStack(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "")
	sib := &config.SiblingInfo{Dir: "/some/path", Project: "x"}
	if err := checkSiblingCycle("x", sib); err != nil {
		t.Errorf("expected nil for empty stack, got %v", err)
	}
}

func TestCheckSiblingCycle_DetectsLoop(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "/a"+string(os.PathListSeparator)+"/b")
	sib := &config.SiblingInfo{Dir: "/a", Project: "a"}
	err := checkSiblingCycle("a-dep", sib)
	if err == nil || !strings.Contains(err.Error(), "sibling cycle") {
		t.Errorf("expected cycle error, got %v", err)
	}
	// Chain should be visible in the message for diagnostics.
	if err != nil && !strings.Contains(err.Error(), "/a → /b → /a") {
		t.Errorf("error should include the chain, got %v", err)
	}
}

func TestCheckSiblingCycle_PathNotInStack(t *testing.T) {
	t.Setenv(protocol.SiblingStack, "/a"+string(os.PathListSeparator)+"/b")
	sib := &config.SiblingInfo{Dir: "/c", Project: "c"}
	if err := checkSiblingCycle("c-dep", sib); err != nil {
		t.Errorf("expected nil when target not in stack, got %v", err)
	}
}

// --- spawnSibling ---------------------------------------------------------

// withFakeRaiozBinary swaps spawnRaiozBinary to point at a script that
// echoes `stdout` and exits with `code`. Restores on cleanup.
func withFakeRaiozBinary(t *testing.T, stdout string, code int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	dir := t.TempDir()
	// The fake binary ignores its args and just writes a known message.
	body := "#!/bin/sh\necho '" + stdout + "'\nexit " + itoaCode(code) + "\n"
	p := filepath.Join(dir, "fakeraioz")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake raioz: %v", err)
	}
	prev := spawnRaiozBinary
	spawnRaiozBinary = func() (string, error) { return p, nil }
	t.Cleanup(func() { spawnRaiozBinary = prev })
}

func itoaCode(c int) string {
	switch c {
	case 0:
		return "0"
	default:
		return "1"
	}
}

func TestSpawnSibling_Success(t *testing.T) {
	withFakeRaiozBinary(t, "started ok", 0)
	sib := &config.SiblingInfo{Dir: t.TempDir(), Project: "kc"}
	err := spawnSibling(context.Background(), "/consumer", "kc", sib)
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestSpawnSibling_FailureIncludesDiagnosticHint(t *testing.T) {
	withFakeRaiozBinary(t, "boom", 1)
	siblingDir := t.TempDir()
	sib := &config.SiblingInfo{Dir: siblingDir, Project: "kc"}
	err := spawnSibling(context.Background(), "/consumer", "kc", sib)
	if err == nil {
		t.Fatal("expected error from non-zero exit")
	}
	if !strings.Contains(err.Error(), siblingDir) {
		t.Errorf("error should include sibling dir for copy-paste, got %v", err)
	}
	if !strings.Contains(err.Error(), "raioz up") {
		t.Errorf("error should suggest the diagnostic command, got %v", err)
	}
}

// TestSpawnSibling_PdeathsigKillsOrphans guards ADR-026
// end-to-end on Linux: when the parent process exits without
// reaping the spawn, the child must die via Pdeathsig. We simulate
// that path by spawning a long-running fake binary, then cancelling
// the parent ctx (mirrors a Ctrl+C). The actual Pdeathsig signal
// comes from the kernel when this test process exits — but the
// ctx-cancellation half (which is what cmd.Process.Kill rides on)
// fires immediately and is what we can assert here.
func TestSpawnSibling_PdeathsigKillsOrphans(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Pdeathsig is Linux-only")
	}
	// Fake binary sleeps long enough that ctx cancellation must be
	// what stops it.
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fakeraioz")
	body := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(binPath, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake raioz: %v", err)
	}
	prev := spawnRaiozBinary
	spawnRaiozBinary = func() (string, error) { return binPath, nil }
	t.Cleanup(func() { spawnRaiozBinary = prev })

	ctx, cancel := context.WithCancel(context.Background())
	sib := &config.SiblingInfo{Dir: t.TempDir(), Project: "kc"}

	done := make(chan error, 1)
	go func() {
		done <- spawnSibling(ctx, "/consumer", "kc", sib)
	}()

	// Let the spawn start, then cancel — emulates Ctrl+C on parent.
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// spawnSibling returned: cmd.Wait observed ctx cancellation
		// and the process is reaped. Test passes.
	case <-time.After(5 * time.Second):
		t.Fatal("spawnSibling did not return within 5s after ctx cancel — " +
			"child likely orphaned")
	}
}

// TestSpawnSibling_PropagatesCorrelationID asserts ADR-024:
// the parent ctx's request ID is propagated to the spawned child via
// the RAIOZ_CORRELATION_ID env var so audit/log records share the
// value across the spawn tree.
func TestSpawnSibling_PropagatesCorrelationID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	// Fake raioz binary writes its inherited correlation env var to a
	// known file so the parent test can verify propagation.
	dir := t.TempDir()
	probe := filepath.Join(dir, "captured-cid")
	body := "#!/bin/sh\nprintf '%s' \"$RAIOZ_CORRELATION_ID\" > " + probe + "\n"
	binPath := filepath.Join(dir, "fakeraioz")
	if err := os.WriteFile(binPath, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake raioz: %v", err)
	}
	prev := spawnRaiozBinary
	spawnRaiozBinary = func() (string, error) { return binPath, nil }
	t.Cleanup(func() { spawnRaiozBinary = prev })

	ctx := logging.WithRequestID(context.Background())
	parentID := logging.GetRequestID(ctx)

	sib := &config.SiblingInfo{Dir: t.TempDir(), Project: "kc"}
	if err := spawnSibling(ctx, "/consumer", "kc", sib); err != nil {
		t.Fatalf("spawnSibling: %v", err)
	}

	captured, err := os.ReadFile(probe)
	if err != nil {
		t.Fatalf("read probe file: %v", err)
	}
	if string(captured) != parentID {
		t.Errorf("child saw RAIOZ_CORRELATION_ID=%q, want parent's %q",
			string(captured), parentID)
	}
}

// TestSpawnSibling_LongLineDoesNotTruncateOutput asserts issue 027:
// the streamer survives a child line longer than bufio.Scanner's
// default 64 KiB buffer. Previously the scanner would silently stop
// reading; now the buffer is raised to 16 MiB.
func TestSpawnSibling_LongLineDoesNotTruncateOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "fakeraioz")
	// 70 KiB of 'x' followed by a newline + final "done" marker. If
	// the scanner truncates at 64 KiB, the spawn would fail; we just
	// verify spawn succeeds end-to-end without orphan errors.
	body := "#!/bin/sh\n" +
		"perl -e 'print q(x) x 70000, \"\\n\"'\n" +
		"echo done\n" +
		"exit 0\n"
	if err := os.WriteFile(binPath, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake raioz: %v", err)
	}
	prev := spawnRaiozBinary
	spawnRaiozBinary = func() (string, error) { return binPath, nil }
	t.Cleanup(func() { spawnRaiozBinary = prev })

	sib := &config.SiblingInfo{Dir: t.TempDir(), Project: "kc"}
	if err := spawnSibling(context.Background(), "/consumer", "kc", sib); err != nil {
		t.Errorf("expected spawn to survive >64KiB line, got %v", err)
	}
}

// Issue 028 — gap 3: when RAIOZ_SIBLING_TIMEOUT fires, the error must
// name the env var so the operator knows which knob to turn. Previously
// only the parser of the env var was tested; the end-to-end deadline
// branch had no coverage.
func TestSpawnSibling_TimeoutSurfacesEnvVarHint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	t.Setenv("RAIOZ_SIBLING_TIMEOUT", "200ms")

	// Fake binary sleeps for 5s — well past the 200ms deadline.
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fakeraioz")
	if err := os.WriteFile(binPath,
		[]byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatalf("write fake raioz: %v", err)
	}
	prev := spawnRaiozBinary
	spawnRaiozBinary = func() (string, error) { return binPath, nil }
	t.Cleanup(func() { spawnRaiozBinary = prev })

	sib := &config.SiblingInfo{Dir: t.TempDir(), Project: "kc"}
	err := spawnSibling(context.Background(), "/consumer", "kc", sib)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "RAIOZ_SIBLING_TIMEOUT") {
		t.Errorf("timeout error must name the env var for actionability: %v", err)
	}
}

func TestSpawnSibling_BinaryLookupFailure(t *testing.T) {
	prev := spawnRaiozBinary
	spawnRaiozBinary = func() (string, error) {
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { spawnRaiozBinary = prev })

	sib := &config.SiblingInfo{Dir: t.TempDir(), Project: "kc"}
	err := spawnSibling(context.Background(), "/consumer", "kc", sib)
	if err == nil || !strings.Contains(err.Error(), "locate raioz binary") {
		t.Errorf("expected binary-lookup error, got %v", err)
	}
}
