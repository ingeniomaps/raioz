package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"raioz/internal/app"
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

	handled, err := tryHandleMeta(context.Background(), path, "up", nil, nil, app.MetaUpOptions{})
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
	handled, err := tryHandleMeta(
		context.Background(), AutoDetectMarker, "up", nil, nil, app.MetaUpOptions{},
	)
	if err != nil || handled {
		t.Errorf("auto-detect must not engage meta dispatch (handled=%v, err=%v)",
			handled, err)
	}
}

// Empty path is also a no-op — early returns must guard the loader call.
func TestTryHandleMeta_EmptyPathSkipped(t *testing.T) {
	handled, err := tryHandleMeta(
		context.Background(), "", "up", nil, nil, app.MetaUpOptions{},
	)
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

	// Stage a passing fake binary and inject it via the newMetaRunner
	// override. Doing this through MetaRunner.Binary (rather than
	// os.Args[0]) is required because resolveBinary prefers
	// os.Executable() — under `go test` that's the test runner itself,
	// which would re-enter the suite recursively.
	binDir := t.TempDir()
	fake := filepath.Join(binDir, "raioz")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	prev := newMetaRunner
	newMetaRunner = func() *app.MetaRunner { return &app.MetaRunner{Binary: fake} }
	t.Cleanup(func() { newMetaRunner = prev })

	handled, err := tryHandleMeta(
		context.Background(), metaPath, "up", nil, nil, app.MetaUpOptions{},
	)
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

	handled, err := tryHandleMeta(context.Background(), path, "up", nil, nil, app.MetaUpOptions{})
	if !handled {
		t.Errorf("invalid meta must still be handled (no fall-through)")
	}
	if err == nil {
		t.Errorf("expected error from loader, got nil")
	}
}
