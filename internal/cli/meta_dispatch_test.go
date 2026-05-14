package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
}

// A regular project config returns handled=false so the caller continues
// with the normal use case.
func TestTryHandleMeta_RegularProjectFallsThrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	if err := os.WriteFile(path, []byte("project: regular\n"), 0644); err != nil {
		t.Fatal(err)
	}

	handled, err := tryHandleMeta(context.Background(), path, "up", nil, nil)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if handled {
		t.Errorf("regular project must not be handled by meta dispatch")
	}
}

// AutoDetectMarker means the user invoked raioz outside any config — no
// meta dispatch should kick in.
func TestTryHandleMeta_AutoDetectMarkerSkipped(t *testing.T) {
	handled, err := tryHandleMeta(context.Background(), AutoDetectMarker, "up", nil, nil)
	if err != nil || handled {
		t.Errorf("auto-detect must not engage meta dispatch (handled=%v, err=%v)",
			handled, err)
	}
}

// Empty path is also a no-op — early returns must guard the loader call.
func TestTryHandleMeta_EmptyPathSkipped(t *testing.T) {
	handled, err := tryHandleMeta(context.Background(), "", "up", nil, nil)
	if err != nil || handled {
		t.Errorf("empty path must not engage meta dispatch")
	}
}

// End-to-end-ish: a meta config with two sub-projects, dispatched against
// a fake `raioz` binary that exits 0 — handled=true, err=nil.
func TestTryHandleMeta_DispatchesToFakeBinary(t *testing.T) {
	skipOnWindows(t)

	// Stage two sub-project dirs.
	base := t.TempDir()
	for _, n := range []string{"sub-a", "sub-b"} {
		if err := os.MkdirAll(filepath.Join(base, n), 0755); err != nil {
			t.Fatal(err)
		}
	}

	metaPath := filepath.Join(base, "raioz.yaml")
	body := "kind: meta\nprojects:\n  - path: sub-a\n  - path: sub-b\n"
	if err := os.WriteFile(metaPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	// Stage a passing fake binary and prepend its dir to PATH so os/exec
	// finds it as `raioz`. We don't go through tryHandleMeta directly
	// because it uses os.Args[0]; instead we exercise the MetaRunner-via
	// dispatch by setting up PATH and asserting the loader recognized
	// the file as meta. The detailed behavior is covered by
	// TestMetaRunner_*; this test just validates the parse + handed=true
	// signal coming out of tryHandleMeta.
	//
	// We swap os.Args[0] for our fake so tryHandleMeta's MetaRunner picks
	// it up via the default-binary path.
	binDir := t.TempDir()
	fake := filepath.Join(binDir, "raioz")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	prevArg0 := os.Args[0]
	os.Args[0] = fake
	t.Cleanup(func() { os.Args[0] = prevArg0 })

	handled, err := tryHandleMeta(context.Background(), metaPath, "up", nil, nil)
	if !handled {
		t.Fatalf("expected handled=true, got false (err=%v)", err)
	}
	if err != nil {
		t.Errorf("expected nil err on all-passing meta, got %v", err)
	}
}

// kind: meta with no projects must surface the loader error AND mark the
// run as handled (so the caller doesn't fall back to project-mode parsing
// of the same file).
func TestTryHandleMeta_InvalidMetaIsHandled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	if err := os.WriteFile(path, []byte("kind: meta\n"), 0644); err != nil {
		t.Fatal(err)
	}

	handled, err := tryHandleMeta(context.Background(), path, "up", nil, nil)
	if !handled {
		t.Errorf("invalid meta must still be handled (no fall-through)")
	}
	if err == nil {
		t.Errorf("expected error from loader, got nil")
	}
}
