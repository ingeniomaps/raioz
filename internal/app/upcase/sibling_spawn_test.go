package upcase

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/config"
)

// --- stack helpers --------------------------------------------------------

func TestReadSiblingStack_Empty(t *testing.T) {
	t.Setenv(siblingStackEnv, "")
	if got := readSiblingStack(); got != nil {
		t.Errorf("expected nil for empty env, got %v", got)
	}
}

func TestReadSiblingStack_Populated(t *testing.T) {
	t.Setenv(siblingStackEnv, "/a"+string(os.PathListSeparator)+"/b")
	got := readSiblingStack()
	if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Errorf("got %v, want [/a /b]", got)
	}
}

func TestPushSiblingStack_AppendsToExisting(t *testing.T) {
	t.Setenv(siblingStackEnv, "/a")
	got := pushSiblingStack("/b", "/c")
	sep := string(os.PathListSeparator)
	want := siblingStackEnv + "=/a" + sep + "/b" + sep + "/c"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPushSiblingStack_FirstEntry(t *testing.T) {
	t.Setenv(siblingStackEnv, "")
	got := pushSiblingStack("/parent", "/child")
	want := siblingStackEnv + "=/parent" + string(os.PathListSeparator) + "/child"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- checkSiblingCycle ----------------------------------------------------

func TestCheckSiblingCycle_NoStack(t *testing.T) {
	t.Setenv(siblingStackEnv, "")
	sib := &config.SiblingInfo{Dir: "/some/path", Project: "x"}
	if err := checkSiblingCycle("x", sib); err != nil {
		t.Errorf("expected nil for empty stack, got %v", err)
	}
}

func TestCheckSiblingCycle_DetectsLoop(t *testing.T) {
	t.Setenv(siblingStackEnv, "/a"+string(os.PathListSeparator)+"/b")
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
	t.Setenv(siblingStackEnv, "/a"+string(os.PathListSeparator)+"/b")
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
