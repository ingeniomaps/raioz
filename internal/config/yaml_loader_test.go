package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML_BasicConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
project: my-app
workspace: acme

services:
  api:
    path: ./api
    dependsOn: [postgres]
    health: /health
    watch: true

  frontend:
    path: ./frontend
    dependsOn: [api]
    watch: native

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
    env: .env.postgres

  redis:
    image: redis:7
`
	os.WriteFile(configPath, []byte(content), 0644)
	// Create required dirs so path validation doesn't fail
	os.MkdirAll(filepath.Join(dir, "api"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	cfg, err := LoadYAML(configPath)
	if err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	if cfg.Project != "my-app" {
		t.Errorf("expected project 'my-app', got '%s'", cfg.Project)
	}
	if cfg.Workspace != "acme" {
		t.Errorf("expected workspace 'acme', got '%s'", cfg.Workspace)
	}
	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Services))
	}
	if len(cfg.Deps) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(cfg.Deps))
	}

	api := cfg.Services["api"]
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "postgres" {
		t.Errorf("api dependsOn should be [postgres], got %v", api.DependsOn)
	}
	if api.Health != "/health" {
		t.Errorf("api health should be '/health', got '%s'", api.Health)
	}
	if !api.Watch.Enabled {
		t.Error("api watch should be enabled")
	}

	frontend := cfg.Services["frontend"]
	if frontend.Watch.Mode != "native" {
		t.Errorf("frontend watch should be 'native', got '%s'", frontend.Watch.Mode)
	}

	pg := cfg.Deps["postgres"]
	if pg.Image != "postgres:16" {
		t.Errorf("postgres image should be 'postgres:16', got '%s'", pg.Image)
	}
	if len(pg.Ports) != 1 || pg.Ports[0] != "5432" {
		t.Errorf("postgres ports should be ['5432'], got %v", pg.Ports)
	}
}

func TestLoadYAML_ProxyBool(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
project: test
proxy: true
dependencies:
  redis:
    image: redis:7
`
	os.WriteFile(configPath, []byte(content), 0644)

	cfg, err := LoadYAML(configPath)
	if err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}
	if cfg.Proxy == nil || !cfg.Proxy.Enabled {
		t.Error("proxy should be enabled")
	}
}

func TestLoadYAML_PrePostHooks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
project: test
pre:
  - infisical login
  - infisical export > .env
post: rm .env
dependencies:
  redis:
    image: redis:7
`
	os.WriteFile(configPath, []byte(content), 0644)

	cfg, err := LoadYAML(configPath)
	if err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}
	if len(cfg.Pre) != 2 {
		t.Errorf("expected 2 pre hooks, got %d", len(cfg.Pre))
	}
	if len(cfg.Post) != 1 {
		t.Errorf("expected 1 post hook, got %d", len(cfg.Post))
	}
}

func TestLoadYAML_MissingProject(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
services:
  api:
    path: ./api
`
	os.WriteFile(configPath, []byte(content), 0644)

	_, err := LoadYAML(configPath)
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestLoadYAML_MissingImage(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
project: test
dependencies:
  postgres:
    ports: ["5432"]
`
	os.WriteFile(configPath, []byte(content), 0644)

	_, err := LoadYAML(configPath)
	if err == nil {
		t.Error("expected error for dependency without image")
	}
}

func TestLoadYAML_InvalidDependsOnRef(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
project: test
services:
  api:
    path: ./api
    dependsOn: [nonexistent]
`
	os.WriteFile(configPath, []byte(content), 0644)
	os.MkdirAll(filepath.Join(dir, "api"), 0755)

	_, err := LoadYAML(configPath)
	if err == nil {
		t.Error("expected error for invalid dependsOn reference")
	}
}

func TestYAMLToDeps_Bridge(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raioz.yaml")
	content := `
project: e-commerce
workspace: acme-corp
proxy: true
pre: ./fetch-secrets.sh

services:
  api:
    path: ./api
    dependsOn: [postgres]
    ports: ["3000"]

dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
`
	os.WriteFile(configPath, []byte(content), 0644)
	os.MkdirAll(filepath.Join(dir, "api"), 0755)

	cfg, err := LoadYAML(configPath)
	if err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	deps, err := YAMLToDeps(cfg)
	if err != nil {
		t.Fatalf("YAMLToDeps failed: %v", err)
	}

	if deps.Project.Name != "e-commerce" {
		t.Errorf("expected project 'e-commerce', got '%s'", deps.Project.Name)
	}
	if deps.Workspace != "acme-corp" {
		t.Errorf("expected workspace 'acme-corp', got '%s'", deps.Workspace)
	}
	if !deps.Proxy {
		t.Error("proxy should be enabled")
	}
	if deps.Network.Name != "acme-corp-net" {
		t.Errorf("expected network 'acme-corp-net', got '%s'", deps.Network.Name)
	}

	api, ok := deps.Services["api"]
	if !ok {
		t.Fatal("api service not found")
	}
	if api.Source.Kind != "local" {
		t.Errorf("expected source kind 'local', got '%s'", api.Source.Kind)
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "postgres" {
		t.Errorf("expected dependsOn [postgres], got %v", api.DependsOn)
	}

	pg, ok := deps.Infra["postgres"]
	if !ok {
		t.Fatal("postgres infra not found")
	}
	if pg.Inline == nil {
		t.Fatal("postgres should be inline infra")
	}
	if pg.Inline.Image != "postgres" {
		t.Errorf("expected image 'postgres', got '%s'", pg.Inline.Image)
	}
	if pg.Inline.Tag != "16" {
		t.Errorf("expected tag '16', got '%s'", pg.Inline.Tag)
	}
}

func TestIsYAMLConfig(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"raioz.yaml", true},
		{"raioz.yml", true},
		{".raioz.json", false},
		{"config.YAML", true},
		{"something.txt", false},
	}
	for _, tt := range tests {
		if got := IsYAMLConfig(tt.path); got != tt.want {
			t.Errorf("IsYAMLConfig(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
