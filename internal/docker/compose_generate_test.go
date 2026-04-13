package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
)

// TestGenerateCompose_FullFlow builds a minimal deps structure and walks the
// full GenerateCompose flow. Since GenerateCompose calls EnsureVolume which
// invokes the Docker CLI, we skip the test when no Docker is available.
func TestGenerateCompose_NoVolumes(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())

	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Network:  config.NetworkConfig{Name: "raioz-net"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
		Env:      config.EnvConfig{},
	}

	path, externalNames, err := GenerateCompose(deps, ws, projectDir)
	if err != nil {
		// EnsureVolume requires Docker; skip when unavailable.
		if strings.Contains(err.Error(), "failed to ensure volume") ||
			strings.Contains(err.Error(), "volume inspect") {
			t.Skipf("skipping: docker required: %v", err)
		}
		t.Fatalf("GenerateCompose error = %v", err)
	}
	if len(externalNames) != 0 {
		t.Errorf("expected no external names, got %v", externalNames)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("compose file missing: %v", err)
	}
}

func TestGenerateCompose_CycleError(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Network: config.NetworkConfig{Name: "raioz-net"},
		Services: map[string]config.Service{
			"a": {
				Source:    config.SourceConfig{Kind: "image", Image: "nginx"},
				Docker:    &config.DockerConfig{},
				DependsOn: []string{"b"},
			},
			"b": {
				Source:    config.SourceConfig{Kind: "image", Image: "nginx"},
				Docker:    &config.DockerConfig{},
				DependsOn: []string{"a"},
			},
		},
		Infra: map[string]config.InfraEntry{},
	}

	_, _, err := GenerateCompose(deps, ws, projectDir)
	if err == nil {
		t.Error("expected cycle error")
	}
}

// TestAddInfraToCompose exercises the public entry point used by GenerateCompose.
func TestAddInfraToCompose(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Network: config.NetworkConfig{Name: "raioz-net"},
		Infra: map[string]config.InfraEntry{
			"pg": {Inline: &config.Infra{
				Image: "postgres",
				Tag:   "15",
				Ports: []string{"5432:5432"},
			}},
			"skip-empty": {}, // no Path, no Inline — skipped
		},
	}

	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{
			"raioz-net": map[string]any{"external": true},
		},
	}

	_, err := addInfraToCompose(
		compose, deps, ws, projectDir, "raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	services := compose["services"].(map[string]any)
	if _, ok := services["pg"]; !ok {
		t.Error("pg missing after addInfraToCompose")
	}
}

func TestAddInfraToCompose_WithExternalPath(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())

	yamlContent := `services:
  extdb:
    image: mysql:8
`
	if err := os.WriteFile(
		filepath.Join(projectDir, "ext.yml"), []byte(yamlContent), 0644,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Network: config.NetworkConfig{Name: "raioz-net"},
		Infra: map[string]config.InfraEntry{
			"db": {Path: "ext.yml"},
		},
	}
	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{
			"raioz-net": map[string]any{"external": true},
		},
	}
	names, err := addInfraToCompose(
		compose, deps, ws, projectDir, "raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(names) != 1 || names[0] != "extdb" {
		t.Errorf("names = %v, want [extdb]", names)
	}
}

// --- writeCombinedEnvFile ---

func TestWriteCombinedEnvFile_Empty(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Env:     config.EnvConfig{},
	}

	if err := writeCombinedEnvFile(deps, ws, projectDir); err != nil {
		t.Fatalf("error: %v", err)
	}
	// No .env should be written
	if _, err := os.Stat(filepath.Join(ws.Root, ".env")); err == nil {
		t.Error(".env should not exist when no vars")
	}
}

func TestWriteCombinedEnvFile_DirectVars(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Env: config.EnvConfig{
			Variables: map[string]string{
				"SIMPLE":      "value",
				"WITH_SPACE":  "hello world",
				"WITH_DOLLAR": "$VAR",
			},
		},
	}

	if err := writeCombinedEnvFile(deps, ws, projectDir); err != nil {
		t.Fatalf("error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ws.Root, ".env"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "SIMPLE=value") {
		t.Errorf("SIMPLE missing: %s", content)
	}
	if !strings.Contains(content, `WITH_SPACE="hello world"`) {
		t.Errorf("quoted value missing: %s", content)
	}
	if !strings.Contains(content, `WITH_DOLLAR="$VAR"`) {
		t.Errorf("dollar-escape missing: %s", content)
	}
}

func TestWriteCombinedEnvFile_InfraObjectEnv(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Env:     config.EnvConfig{},
		Infra: map[string]config.InfraEntry{
			"pg": {Inline: &config.Infra{
				Image: "postgres",
				Env: &config.EnvValue{
					IsObject: true,
					Variables: map[string]string{
						"POSTGRES_USER": "admin",
					},
				},
			}},
		},
	}
	if err := writeCombinedEnvFile(deps, ws, projectDir); err != nil {
		t.Fatalf("error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws.Root, ".env"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "POSTGRES_USER=admin") {
		t.Errorf("missing infra var: %s", data)
	}
}

func TestWriteCombinedEnvFile_ProjectEnvFile(t *testing.T) {
	projectDir := t.TempDir()
	// Create a project-relative env file
	envContent := "API_KEY=secret\n"
	if err := os.WriteFile(
		filepath.Join(projectDir, ".env.shared"), []byte(envContent), 0644,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Env: config.EnvConfig{
			Files: []string{".env.shared"},
		},
	}
	if err := writeCombinedEnvFile(deps, ws, projectDir); err != nil {
		t.Fatalf("error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(ws.Root, ".env"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "API_KEY=secret") {
		t.Errorf("missing API_KEY: %s", data)
	}
}
