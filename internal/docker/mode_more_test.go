package docker

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestAddDevBindMount_NonDev(t *testing.T) {
	ws := &workspace.Workspace{Root: "/tmp"}
	cfg := map[string]any{}
	svc := config.Service{
		Source: config.SourceConfig{Kind: "git"},
		Docker: &config.DockerConfig{Mode: "prod"},
	}
	AddDevBindMount(cfg, "api", svc, ws)
	if _, ok := cfg["volumes"]; ok {
		t.Error("prod mode should not add bind mount")
	}
}

func TestAddDevBindMount_NonGit(t *testing.T) {
	ws := &workspace.Workspace{Root: "/tmp"}
	cfg := map[string]any{}
	svc := config.Service{
		Source: config.SourceConfig{Kind: "image", Image: "nginx"},
		Docker: &config.DockerConfig{Mode: "dev"},
	}
	AddDevBindMount(cfg, "api", svc, ws)
	if _, ok := cfg["volumes"]; ok {
		t.Error("image service should not add bind mount")
	}
}

func TestAddDevBindMount_MissingServicePath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:                tmpDir,
		ServicesDir:         filepath.Join(tmpDir, "services"),
		LocalServicesDir:    filepath.Join(tmpDir, "services", "local"),
		ReadonlyServicesDir: filepath.Join(tmpDir, "services", "ro"),
	}
	cfg := map[string]any{}
	// Git service whose path doesn't exist
	svc := config.Service{
		Source: config.SourceConfig{Kind: "git", Path: "missing"},
		Docker: &config.DockerConfig{Mode: "dev", Runtime: "node"},
	}
	AddDevBindMount(cfg, "api", svc, ws)
	if _, ok := cfg["volumes"]; ok {
		t.Error("missing service path should not add bind mount")
	}
}

func TestAddDevBindMount_DevNodeRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services", "local")
	svcPath := filepath.Join(servicesDir, "api")
	if err := os.MkdirAll(svcPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := &workspace.Workspace{
		Root:                tmpDir,
		ServicesDir:         filepath.Join(tmpDir, "services"),
		LocalServicesDir:    servicesDir,
		ReadonlyServicesDir: filepath.Join(tmpDir, "services", "ro"),
	}
	cfg := map[string]any{}
	svc := config.Service{
		Source: config.SourceConfig{Kind: "git", Path: "api"},
		Docker: &config.DockerConfig{Mode: "dev", Runtime: "node"},
	}
	AddDevBindMount(cfg, "api", svc, ws)
	vols, ok := cfg["volumes"].([]string)
	if !ok || len(vols) == 0 {
		t.Fatalf("expected volumes, got %v", cfg["volumes"])
	}
	// Calling again should not duplicate
	AddDevBindMount(cfg, "api", svc, ws)
	vols2, _ := cfg["volumes"].([]string)
	if len(vols2) != len(vols) {
		t.Errorf("duplicate bind mount added: %v", vols2)
	}
}
