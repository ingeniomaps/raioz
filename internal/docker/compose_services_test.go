package docker

import (
	"testing"

	"raioz/internal/config"
)

func TestAddServiceToCompose_ImageService(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	services := map[string]any{}
	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "image", Image: "nginx", Tag: "alpine",
		},
		Docker: &config.DockerConfig{
			Ports:   []string{"80:80"},
			Volumes: []string{"data:/data"},
		},
	}

	err := addServiceToCompose(
		services, "web", svc, deps, ws, projectDir,
		"raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	cfg, ok := services["web"].(map[string]any)
	if !ok {
		t.Fatal("service not added")
	}
	if cfg["image"] != "nginx:alpine" {
		t.Errorf("image = %v, want nginx:alpine", cfg["image"])
	}
	if _, ok := cfg["volumes"]; !ok {
		t.Error("volumes missing")
	}
	if cfg["container_name"] == nil {
		t.Error("container_name missing")
	}
}

func TestAddServiceToCompose_SkipDisabled(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	disabled := false
	services := map[string]any{}
	svc := config.Service{
		Enabled: &disabled,
		Source:  config.SourceConfig{Kind: "image", Image: "nginx"},
		Docker:  &config.DockerConfig{},
	}

	err := addServiceToCompose(
		services, "web", svc, deps, ws, projectDir,
		"raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if _, ok := services["web"]; ok {
		t.Error("disabled service should not be added")
	}
}

func TestAddServiceToCompose_SkipHostCommand(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	services := map[string]any{}
	svc := config.Service{
		Source: config.SourceConfig{
			Kind: "git", Command: "npm start",
		},
	}

	err := addServiceToCompose(
		services, "web", svc, deps, ws, projectDir,
		"raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if _, ok := services["web"]; ok {
		t.Error("host-command service should not be added")
	}
}

func TestAddServiceToCompose_ImageWithStaticIP(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	services := map[string]any{}
	svc := config.Service{
		Source: config.SourceConfig{Kind: "image", Image: "nginx"},
		Docker: &config.DockerConfig{
			IP:    "150.150.0.10",
			Ports: []string{"80:80"},
		},
	}

	err := addServiceToCompose(
		services, "web", svc, deps, ws, projectDir,
		"raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	cfg := services["web"].(map[string]any)
	nets, ok := cfg["networks"].(map[string]any)
	if !ok {
		t.Fatalf("networks should be map, got %T", cfg["networks"])
	}
	entry, _ := nets["raioz-net"].(map[string]any)
	if entry["ipv4_address"] != "150.150.0.10" {
		t.Errorf("ipv4_address = %v", entry["ipv4_address"])
	}
}

func TestAddServiceToCompose_WithDependsOn(t *testing.T) {
	projectDir := t.TempDir()
	ws := mkWorkspace(t.TempDir())
	deps := &config.Deps{Project: config.Project{Name: "proj"}}

	services := map[string]any{}
	svc := config.Service{
		Source:    config.SourceConfig{Kind: "image", Image: "nginx"},
		DependsOn: []string{"db", "cache"},
		Docker:    &config.DockerConfig{},
	}

	err := addServiceToCompose(
		services, "web", svc, deps, ws, projectDir,
		"raioz-net", map[string]string{},
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	cfg := services["web"].(map[string]any)
	dep, ok := cfg["depends_on"].([]string)
	if !ok || len(dep) != 2 {
		t.Errorf("depends_on = %v", cfg["depends_on"])
	}
}
