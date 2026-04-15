package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetAllActivePorts_WithStateFiles(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesDir := filepath.Join(tmpDir, "workspaces")
	proj1 := filepath.Join(workspacesDir, "project1")
	if err := os.MkdirAll(proj1, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// State file with service and infra ports
	state := map[string]any{
		"project": map[string]any{"name": "project1"},
		"services": map[string]any{
			"api": map[string]any{
				"docker": map[string]any{
					"ports": []string{"3000:8080", "9000:9000"},
				},
			},
			"no-ports": map[string]any{
				"docker": map[string]any{
					"ports": []string{},
				},
			},
		},
		"infra": map[string]any{
			"pg": map[string]any{
				"ports": []string{"5432:5432"},
			},
		},
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(
		filepath.Join(proj1, ".state.json"), data, 0644,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Extra: one file that is invalid JSON
	proj2 := filepath.Join(workspacesDir, "project2")
	_ = os.MkdirAll(proj2, 0755)
	_ = os.WriteFile(
		filepath.Join(proj2, ".state.json"), []byte("not-json"), 0644,
	)

	ports, err := GetAllActivePorts(tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should have 3 ports from project1 (2 service + 1 infra)
	if len(ports) != 3 {
		t.Errorf("expected 3 ports, got %d: %v", len(ports), ports)
	}

	var foundApi, foundPg bool
	for _, p := range ports {
		if p.HostPort == 3000 && p.ContainerPort == 8080 && p.Service == "api" {
			foundApi = true
		}
		if p.HostPort == 5432 && p.Service == "pg" {
			foundPg = true
		}
	}
	if !foundApi {
		t.Error("api port not found")
	}
	if !foundPg {
		t.Error("pg port not found")
	}
}

func TestGetProjectUsingPort_Found(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesDir := filepath.Join(tmpDir, "workspaces")
	proj := filepath.Join(workspacesDir, "proj")
	_ = os.MkdirAll(proj, 0755)

	state := map[string]any{
		"services": map[string]any{
			"api": map[string]any{
				"docker": map[string]any{
					"ports": []string{"8080:80"},
				},
			},
		},
	}
	data, _ := json.Marshal(state)
	_ = os.WriteFile(
		filepath.Join(proj, ".state.json"), data, 0644,
	)

	got, err := GetProjectUsingPort("8080", tmpDir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Project != "proj" || got.Service != "api" {
		t.Errorf("got %+v", got)
	}
}
