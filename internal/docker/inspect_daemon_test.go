package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/runtime"
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

func TestGetServicesInfoWithContext_NotRunning(t *testing.T) {
	requireDocker(t)
	tmp := t.TempDir()
	compose := mkValidCompose(t, tmp)
	result, err := GetServicesInfoWithContext(
		context.Background(), compose, []string{"svc1"}, "proj",
		map[string]models.Service{}, nil,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = result
}

// --- PullImage wrapper (calls WithContext): just exercise wrapper ---

// --- GetAvailableServices for a valid compose file ---

// --- GetServiceNames for a valid compose file ---

// --- GetServicesStatus for a valid compose file (no running services) ---

func TestGetServicesStatus_Valid(t *testing.T) {
	requireDocker(t)
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
	requireDocker(t)
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

// A container that exists but is not running used to be reported as
// "running" because the status was inferred from the container name alone.
// Paused is the cheapest state to stage deterministically; the same code
// path carries restarting, exited and dead.
func TestGetServicesInfoWithContext_HonorsContainerState(t *testing.T) {
	requireDocker(t)
	tmp := t.TempDir()
	compose := filepath.Join(tmp, "docker-compose.yml")
	content := `services:
  svc1:
    image: alpine:latest
    command: ["sleep", "300"]
`
	if err := os.WriteFile(compose, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx := context.Background()
	if out, err := exec.CommandContext(ctx, runtime.Binary(),
		"compose", "-f", compose, "up", "-d").CombinedOutput(); err != nil {
		t.Skipf("compose up unavailable: %v (%s)", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command(runtime.Binary(), "compose", "-f", compose, "down", "-t", "1").Run()
	})
	if out, err := exec.CommandContext(ctx, runtime.Binary(),
		"compose", "-f", compose, "pause", "svc1").CombinedOutput(); err != nil {
		t.Skipf("compose pause unavailable: %v (%s)", err, out)
	}

	result, err := GetServicesInfoWithContext(ctx, compose, []string{"svc1"},
		"testproj", map[string]models.Service{}, nil)
	if err != nil {
		t.Fatalf("GetServicesInfoWithContext: %v", err)
	}
	info, ok := result["svc1"]
	if !ok {
		t.Fatalf("no info for svc1: %+v", result)
	}
	if info.Status != "paused" {
		t.Errorf("Status = %q, want %q", info.Status, "paused")
	}
}
