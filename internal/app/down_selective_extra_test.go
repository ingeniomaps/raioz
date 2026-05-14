package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// writeProjectState seeds .raioz.state.json under dir so downSelectiveServices
// has something to read. Only HostPIDs are interesting for these tests.
func writeProjectState(t *testing.T, dir string, pids map[string]int) {
	t.Helper()
	st := &models.LocalState{HostPIDs: pids}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".raioz.state.json"), b, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func readProjectState(t *testing.T, dir string) *models.LocalState {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".raioz.state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var st models.LocalState
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &st
}

func TestDownSelectiveServices_UnknownTargetReturnsError(t *testing.T) {
	initI18nForTest(t)
	uc := &DownUseCase{deps: newFullMockDeps()}
	deps := &models.Deps{
		Project:  models.Project{Name: "myproj"},
		Services: map[string]models.Service{"api": {}},
		Infra:    map[string]models.InfraEntry{"postgres": {}},
	}

	err := uc.downSelectiveServices(
		context.Background(), deps, t.TempDir(), "myproj",
		[]string{"api", "wat"},
	)
	if err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"unknown service or dependency", "wat"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q: %s", want, msg)
		}
	}
}

func TestDownSelectiveServices_TouchesOnlyRequestedServices(t *testing.T) {
	initI18nForTest(t)

	// Stub destructive functions so the test never delivers real signals
	// or talks to a Docker daemon.
	var killedPIDs []int
	prevKill := killProcessGroup
	killProcessGroup = func(pid int) { killedPIDs = append(killedPIDs, pid) }
	t.Cleanup(func() { killProcessGroup = prevKill })

	var sweepPaths []string
	prevSweep := killOrphansByCwdFn
	killOrphansByCwdFn = func(p string) []int {
		sweepPaths = append(sweepPaths, p)
		return nil
	}
	t.Cleanup(func() { killOrphansByCwdFn = prevSweep })

	// listContainersByLabelsFn returning empty avoids stopAndRemoveContainer
	// firing real `docker stop` subprocesses for this test.
	var labelQueries []map[string]string
	prevList := listContainersByLabelsFn
	listContainersByLabelsFn = func(_ context.Context, labels map[string]string) []string {
		labelQueries = append(labelQueries, labels)
		return nil
	}
	t.Cleanup(func() { listContainersByLabelsFn = prevList })

	projectDir := t.TempDir()
	writeProjectState(t, projectDir, map[string]int{
		"api":   12345,
		"web":   67890, // untouched — not in request list
		"batch": 11111,
	})

	deps := &models.Deps{
		Project: models.Project{Name: "myproj"},
		Services: map[string]models.Service{
			"api":   {Source: models.SourceConfig{Path: "api"}},
			"web":   {Source: models.SourceConfig{Path: "web"}},
			"batch": {Source: models.SourceConfig{Path: "batch"}},
		},
	}

	uc := &DownUseCase{deps: newFullMockDeps()}
	if err := uc.downSelectiveServices(
		context.Background(), deps, projectDir, "myproj",
		[]string{"api", "batch"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Ints(killedPIDs)
	if !reflect.DeepEqual(killedPIDs, []int{11111, 12345}) {
		t.Errorf("killed PIDs = %v, want [11111 12345] only (67890 must be untouched)", killedPIDs)
	}

	// State on disk must keep "web" and drop the requested two.
	after := readProjectState(t, projectDir)
	if _, gone := after.HostPIDs["api"]; gone {
		t.Errorf("api PID should have been removed from state, got %v", after.HostPIDs)
	}
	if _, gone := after.HostPIDs["batch"]; gone {
		t.Errorf("batch PID should have been removed from state, got %v", after.HostPIDs)
	}
	if pid, ok := after.HostPIDs["web"]; !ok || pid != 67890 {
		t.Errorf("web must remain at 67890, got %v", after.HostPIDs)
	}

	// listContainersByLabelsFn must have been asked about the targeted
	// services only — never "web".
	gotServices := map[string]bool{}
	for _, q := range labelQueries {
		if svc := q["com.raioz.service"]; svc != "" {
			gotServices[svc] = true
		}
	}
	if !gotServices["api"] || !gotServices["batch"] {
		t.Errorf("expected label queries for api+batch, got %v", gotServices)
	}
	if gotServices["web"] {
		t.Errorf("must never label-query a service not in the request: %v", gotServices)
	}

	// Orphan sweep must also be scoped to the requested services only.
	sort.Strings(sweepPaths)
	for _, p := range sweepPaths {
		if strings.HasSuffix(p, "/web") {
			t.Errorf("orphan sweep must not target /web; paths = %v", sweepPaths)
		}
	}
}

func TestDownSelectiveServices_EmptyListNoop(t *testing.T) {
	initI18nForTest(t)

	prevKill := killProcessGroup
	killProcessGroup = func(int) { t.Error("must not kill anything when request is empty") }
	t.Cleanup(func() { killProcessGroup = prevKill })

	uc := &DownUseCase{deps: newFullMockDeps()}
	deps := &models.Deps{
		Project:  models.Project{Name: "myproj"},
		Services: map[string]models.Service{"api": {}},
	}
	if err := uc.downSelectiveServices(
		context.Background(), deps, t.TempDir(), "myproj", nil,
	); err != nil {
		t.Errorf("empty request must not error: %v", err)
	}
}
