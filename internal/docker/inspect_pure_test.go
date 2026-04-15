package docker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"raioz/internal/config"
)

func TestContainerInspect_JSONParsing(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		status string
		health string
		image  string
	}{
		{
			"running with health",
			`[{"State":{"Status":"running","Health":{"Status":"healthy"},` +
				`"StartedAt":"2025-01-01T00:00:00Z"},` +
				`"Config":{"Image":"postgres:16","Env":["FOO=bar"]},` +
				`"Image":"sha256:abc123def456789"}]`,
			"running",
			"healthy",
			"postgres:16",
		},
		{
			"running without health",
			`[{"State":{"Status":"running","StartedAt":"2025-01-01T00:00:00Z"},` +
				`"Config":{"Image":"redis:7","Env":[]},"Image":"sha256:xyz789"}]`,
			"running",
			"",
			"redis:7",
		},
		{
			"exited",
			`[{"State":{"Status":"exited","StartedAt":"2025-01-01T00:00:00Z"},` +
				`"Config":{"Image":"alpine","Env":[]},"Image":"sha256:short"}]`,
			"exited",
			"",
			"alpine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inspectData []ContainerInspect
			if err := json.Unmarshal([]byte(tt.json), &inspectData); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(inspectData) == 0 {
				t.Fatal("empty inspect data")
			}

			inspect := inspectData[0]
			if inspect.State.Status != tt.status {
				t.Errorf("status = %q, want %q", inspect.State.Status, tt.status)
			}
			if tt.health != "" {
				if inspect.State.Health == nil {
					t.Fatal("expected health info")
				}
				if inspect.State.Health.Status != tt.health {
					t.Errorf("health = %q, want %q",
						inspect.State.Health.Status, tt.health)
				}
			}
			if inspect.Config.Image != tt.image {
				t.Errorf("image = %q, want %q", inspect.Config.Image, tt.image)
			}
		})
	}
}

func TestFormatUptime_Comprehensive(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0m"},
		{"under a minute", 30 * time.Second, "0m"},
		{"1 minute", time.Minute, "1m"},
		{"59 minutes", 59 * time.Minute, "59m"},
		{"1 hour", time.Hour, "1h 0m"},
		{"1 hour 1 min", time.Hour + time.Minute, "1h 1m"},
		{"23 hours 59 min", 23*time.Hour + 59*time.Minute, "23h 59m"},
		{"1 day", 24 * time.Hour, "1d 0h 0m"},
		{"1 day 2 hours 3 min", 26*time.Hour + 3*time.Minute, "1d 2h 3m"},
		{"7 days", 7 * 24 * time.Hour, "7d 0h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUptime(tt.d)
			if got != tt.want {
				t.Errorf("formatUptime(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestBatchInspect_EmptyNames(t *testing.T) {
	result := batchInspect(context.Background(), []string{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestBatchInspect_NonexistentContainers(t *testing.T) {
	requireDocker(t)
	result := batchInspect(context.Background(), []string{
		"raioz-test-nonexistent-1",
		"raioz-test-nonexistent-2",
	})
	// docker inspect will fail, so result should be empty
	if len(result) != 0 {
		t.Errorf("expected empty for nonexistent, got %d", len(result))
	}
}

func TestBatchResourceUsage_EmptyNames(t *testing.T) {
	result := batchResourceUsage(context.Background(), []string{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestBatchResourceUsage_NonexistentContainers(t *testing.T) {
	requireDocker(t)
	result := batchResourceUsage(context.Background(), []string{
		"raioz-test-nonexistent-1",
	})
	if len(result) != 0 {
		t.Errorf("expected empty for nonexistent, got %d", len(result))
	}
}

func TestServiceInfo_Fields(t *testing.T) {
	info := &ServiceInfo{
		Name:        "api",
		Status:      "running",
		Health:      "healthy",
		Uptime:      "2h 30m",
		Memory:      "256MB/1GB",
		CPU:         "12.5%",
		Image:       "myapp:latest",
		Version:     "abc123def456",
		LastUpdated: "2025-01-01 12:00:00",
		Linked:      true,
		LinkTarget:  "/home/user/projects/api",
	}

	if info.Name != "api" {
		t.Errorf("Name = %q", info.Name)
	}
	if !info.Linked {
		t.Error("Linked should be true")
	}
	if info.LinkTarget != "/home/user/projects/api" {
		t.Errorf("LinkTarget = %q", info.LinkTarget)
	}
}

func TestGetContainerNameWithContext_EmptyComposePath(t *testing.T) {
	_, err := GetContainerNameWithContext(
		context.Background(), "", "svc",
	)
	if err == nil {
		t.Error("expected error for empty compose path")
	}
}

func TestGetServiceInfoWithContext_InvalidPath(t *testing.T) {
	// Invalid path => GetContainerNameWithContext fails => returns stopped
	info, err := GetServiceInfoWithContext(
		context.Background(), "/nonexistent/path.yml",
		"svc", "proj", nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("info should not be nil")
	}
	if info.Status != "stopped" {
		t.Errorf("status = %q, want stopped", info.Status)
	}
	if info.Health != "none" {
		t.Errorf("health = %q, want none", info.Health)
	}
}

func TestGetServicesInfoWithContext_InvalidPath(t *testing.T) {
	result, err := GetServicesInfoWithContext(
		context.Background(), "/nonexistent/path.yml",
		[]string{"svc1", "svc2"}, "proj",
		map[string]config.Service{}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All services should be "stopped"
	for _, name := range []string{"svc1", "svc2"} {
		info, ok := result[name]
		if !ok {
			t.Errorf("missing entry for %s", name)
			continue
		}
		if info.Status != "stopped" {
			t.Errorf("%s.Status = %q, want stopped", name, info.Status)
		}
	}
}

func TestGetServicesInfoWithContext_EmptyList(t *testing.T) {
	result, err := GetServicesInfoWithContext(
		context.Background(), "/tmp/fake.yml",
		[]string{}, "proj",
		map[string]config.Service{}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestGetResourceUsage_Wrapper(t *testing.T) {
	// Exercises the wrapper function (calls WithContext)
	_, _, err := getResourceUsage("raioz-nonexistent-container-xyz")
	if err == nil {
		// Docker might not be available, which is fine
		t.Log("no error (docker may not be available)")
	}
}

func TestGetServiceInfo_Wrapper(t *testing.T) {
	// Exercises the non-context wrapper
	info, err := GetServiceInfo(
		"/nonexistent/path.yml", "svc", "proj", nil, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Status != "stopped" {
		t.Errorf("status = %q, want stopped", info.Status)
	}
}

func TestGetServicesInfo_Wrapper(t *testing.T) {
	result, err := GetServicesInfo(
		"/nonexistent/path.yml",
		[]string{"svc1"},
		"proj",
		map[string]config.Service{},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info, ok := result["svc1"]; ok {
		if info.Status != "stopped" {
			t.Errorf("svc1.Status = %q, want stopped", info.Status)
		}
	}
}
