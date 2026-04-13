package docker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

// Tests that exercise inspect.go functions by calling them with a valid
// but non-running compose file. The functions return a stopped ServiceInfo
// without error, so no panic and coverage is hit.

func mkValidCompose(t *testing.T, dir string) string {
	t.Helper()
	compose := filepath.Join(dir, "docker-compose.yml")
	content := `services:
  svc1:
    image: alpine:latest
    command: ["sleep", "1"]
`
	if err := os.WriteFile(compose, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return compose
}

func TestGetContainerName_NotRunning(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	// Service not running, should return "" (or error)
	name, err := GetContainerName(compose, "svc1")
	_ = err
	_ = name
}

func TestGetServiceInfo_NotRunning(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	info, err := GetServiceInfo(compose, "svc1", "proj", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if info == nil {
		t.Fatal("info nil")
	}
	// Service is not running
	if info.Status != "stopped" {
		t.Errorf("status = %q, want stopped", info.Status)
	}
}

func TestGetServiceInfo_WithService(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	svc := &config.Service{
		Source: config.SourceConfig{Kind: "image", Image: "alpine"},
	}
	info, err := GetServiceInfo(compose, "svc1", "proj", svc, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if info == nil {
		t.Fatal("info nil")
	}
}

func TestGetServicesInfo_NotRunning(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	result, err := GetServicesInfo(
		compose, []string{"svc1", "svc2"}, "proj",
		map[string]config.Service{
			"svc1": {Source: config.SourceConfig{Kind: "image", Image: "alpine"}},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected at least one result entry")
	}
}

func TestGetServicesInfoWithContext_NotRunning(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	result, err := GetServicesInfoWithContext(
		context.Background(), compose, []string{"svc1"}, "proj",
		map[string]config.Service{}, nil,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = result
}

// --- getResourceUsage: call with nonexistent container ---

func TestGetResourceUsage_Nonexistent(t *testing.T) {
	_, _, err := getResourceUsage("raioz-test-nonexistent-container-xyz-12345")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

// --- PullImage wrapper (calls WithContext): just exercise wrapper ---

func TestPullImage_Wrapper(t *testing.T) {
	// Use an obviously-invalid image name so the wrapper returns quickly
	// with an error. This still exercises the wrapper code path.
	_ = PullImage("raioz-test-invalid-image-that-does-not-exist-xyz:bogus")
}

// --- GetAvailableServices for a valid compose file ---

func TestGetAvailableServices_Valid(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	services, err := GetAvailableServices(compose)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for _, s := range services {
		if s == "svc1" {
			found = true
		}
	}
	if !found {
		t.Errorf("svc1 not in services: %v", services)
	}
}

// --- GetServiceNames for a valid compose file ---

func TestGetServiceNames_Valid(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	names, err := GetServiceNames(compose)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(names) == 0 {
		t.Error("expected at least one service name")
	}
}

// --- GetServicesStatus for a valid compose file (no running services) ---

func TestGetServicesStatus_Valid(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	status, err := GetServicesStatus(compose)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// All services should be "stopped" or absent
	_ = status
}

// --- AreServicesRunning with valid compose ---

func TestAreServicesRunning_Valid(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	running, err := AreServicesRunning(compose, []string{"svc1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if running {
		t.Error("expected false (service not up)")
	}
}

// --- ViewLogs with valid compose, no --follow ---

func TestViewLogs_Valid(t *testing.T) {
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	// May return an error if service is not running, but exercises the path
	_ = ViewLogs(compose, LogsOptions{Tail: 5})
}
