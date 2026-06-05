package refcount

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// isolate points RaiozStateDir at a fresh temp dir for the duration of the
// test via RAIOZ_HOME. t.Setenv forbids t.Parallel, which is fine — these
// tests touch a process-global file + mutex.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	return dir
}

func TestAddRef_Idempotent(t *testing.T) {
	isolate(t)
	for range 3 {
		if err := AddRef("conorbi", "loki", "observability"); err != nil {
			t.Fatalf("AddRef: %v", err)
		}
	}
	refs, err := Refs("conorbi", "loki")
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}
	if !slices.Equal(refs, []string{"observability"}) {
		t.Errorf("refs = %v, want [observability]", refs)
	}
}

func TestDropRef_LastConsumerEmpties(t *testing.T) {
	isolate(t)
	if err := AddRef("conorbi", "loki", "observability"); err != nil {
		t.Fatalf("AddRef: %v", err)
	}
	remaining, err := DropRef("conorbi", "loki", "observability")
	if err != nil {
		t.Fatalf("DropRef: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("remaining = %v, want empty (last consumer)", remaining)
	}
}

// TestTwoConsumers walks the exact scenario from issue 069: two projects
// share loki; the first down keeps it alive, the second tears it down.
func TestTwoConsumers_Issue069(t *testing.T) {
	isolate(t)
	if err := AddRef("conorbi", "loki", "observability"); err != nil {
		t.Fatalf("AddRef A: %v", err)
	}
	if err := AddRef("conorbi", "loki", "api"); err != nil {
		t.Fatalf("AddRef B: %v", err)
	}

	remaining, err := DropRef("conorbi", "loki", "observability")
	if err != nil {
		t.Fatalf("DropRef A: %v", err)
	}
	if !slices.Equal(remaining, []string{"api"}) {
		t.Fatalf("after first down: remaining = %v, want [api] (keep alive)", remaining)
	}

	remaining, err = DropRef("conorbi", "loki", "api")
	if err != nil {
		t.Fatalf("DropRef B: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("after last down: remaining = %v, want empty (tear down)", remaining)
	}
}

func TestReconcile_PurgesDeadProjects(t *testing.T) {
	isolate(t)
	_ = AddRef("conorbi", "loki", "observability")
	_ = AddRef("conorbi", "loki", "ghost") // project that died without DropRef
	_ = AddRef("conorbi", "jaeger", "ghost")

	// Only observability is actually live now.
	if err := Reconcile("conorbi", []string{"observability"}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	loki, _ := Refs("conorbi", "loki")
	if !slices.Equal(loki, []string{"observability"}) {
		t.Errorf("loki refs = %v, want [observability]", loki)
	}
	jaeger, _ := Refs("conorbi", "jaeger")
	if len(jaeger) != 0 {
		t.Errorf("jaeger refs = %v, want empty (only ghost referenced it)", jaeger)
	}
}

// TestEmptyStateRemovesFile asserts a clean teardown leaves no artifact
// (ADR-023: state mirrors reality).
func TestEmptyStateRemovesFile(t *testing.T) {
	dir := isolate(t)
	_ = AddRef("conorbi", "loki", "observability")
	if _, err := os.Stat(filepath.Join(dir, stateFileName)); err != nil {
		t.Fatalf("state file should exist after AddRef: %v", err)
	}
	if _, err := DropRef("conorbi", "loki", "observability"); err != nil {
		t.Fatalf("DropRef: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, stateFileName)); !os.IsNotExist(err) {
		t.Errorf("state file should be gone after last DropRef, stat err = %v", err)
	}
}

func TestRefs_UnknownWorkspaceOrDep(t *testing.T) {
	isolate(t)
	refs, err := Refs("nope", "loki")
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("refs = %v, want empty for unknown workspace", refs)
	}
}
