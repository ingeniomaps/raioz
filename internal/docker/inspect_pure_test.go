package docker

import (
	"context"
	"encoding/json"
	"testing"

	"raioz/internal/domain/models"
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

func TestGetContainerNameWithContext_EmptyComposePath(t *testing.T) {
	_, err := GetContainerNameWithContext(
		context.Background(), "", "svc",
	)
	if err == nil {
		t.Error("expected error for empty compose path")
	}
}

func TestGetServicesInfoWithContext_InvalidPath(t *testing.T) {
	result, err := GetServicesInfoWithContext(
		context.Background(), "/nonexistent/path.yml",
		[]string{"svc1", "svc2"}, "proj",
		map[string]models.Service{}, nil,
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
		map[string]models.Service{}, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}
