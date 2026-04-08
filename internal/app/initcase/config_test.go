package initcase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

// Test helpers are in helpers_test.go

func TestCreateConfig(t *testing.T) {
	initI18n(t)

	tests := []struct {
		name    string
		project string
		network string
	}{
		{name: "valid names", project: "my-project", network: "my-project-network"},
		{name: "custom names", project: "billing-dashboard", network: "billing-net"},
		{name: "single word", project: "api", network: "api-network"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := newTestUseCase()
			deps, err := uc.createConfig(tt.project, tt.network, nil, nil)
			if err != nil {
				t.Fatalf("createConfig() error: %v", err)
			}

			if deps.SchemaVersion != "1.0" {
				t.Errorf("SchemaVersion = %s, want 1.0", deps.SchemaVersion)
			}
			if deps.Project.Name != tt.project {
				t.Errorf("Project.Name = %s, want %s", deps.Project.Name, tt.project)
			}
			if deps.Network.Name != tt.network {
				t.Errorf("Network.Name = %s, want %s", deps.Network.Name, tt.network)
			}
			if deps.Services == nil {
				t.Error("Services should be initialized")
			}
			if deps.Infra == nil {
				t.Error("Infra should be initialized")
			}
			if !deps.Env.UseGlobal {
				t.Error("Env.UseGlobal should be true")
			}
		})
	}
}

func TestCreateConfigEnvFiles(t *testing.T) {
	initI18n(t)

	uc := newTestUseCase()
	deps, err := uc.createConfig("billing", "billing-net", nil, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(deps.Env.Files) != 2 {
		t.Fatalf("Env.Files = %v, want 2 entries", deps.Env.Files)
	}
	if deps.Env.Files[0] != "global" {
		t.Errorf("Env.Files[0] = %s, want global", deps.Env.Files[0])
	}
	if deps.Env.Files[1] != "projects/billing" {
		t.Errorf("Env.Files[1] = %s, want projects/billing", deps.Env.Files[1])
	}
}

func TestCreateConfigWithService(t *testing.T) {
	initI18n(t)

	uc := newTestUseCase()
	svcs := []serviceResult{
		{
			Name:   "api",
			Source: config.SourceConfig{Kind: "git", Repo: "git@github.com:org/api.git", Branch: "main", Path: "services/api"},
			Docker: &config.DockerConfig{Mode: "dev", Ports: []string{"3000:3000"}},
		},
	}

	deps, err := uc.createConfig("my-project", "my-network", svcs, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(deps.Services) != 1 {
		t.Fatalf("Services count = %d, want 1", len(deps.Services))
	}

	s, ok := deps.Services["api"]
	if !ok {
		t.Fatal("service 'api' not found")
	}
	if s.Source.Kind != "git" {
		t.Errorf("Source.Kind = %s, want git", s.Source.Kind)
	}
	if s.Docker == nil || s.Docker.Mode != "dev" {
		t.Error("Docker config missing or wrong mode")
	}
}

func TestCreateConfigWithMultipleServices(t *testing.T) {
	initI18n(t)

	uc := newTestUseCase()
	svcs := []serviceResult{
		{Name: "api", Source: config.SourceConfig{Kind: "git", Repo: "git@github.com:org/api.git", Branch: "main", Path: "services/api"}, Docker: &config.DockerConfig{Mode: "dev", Ports: []string{"3000:3000"}}},
		{Name: "web", Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"}, Docker: &config.DockerConfig{Mode: "prod", Ports: []string{"80:80"}}},
	}

	deps, err := uc.createConfig("my-project", "my-network", svcs, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(deps.Services) != 2 {
		t.Fatalf("Services count = %d, want 2", len(deps.Services))
	}
	if _, ok := deps.Services["api"]; !ok {
		t.Error("service 'api' not found")
	}
	if _, ok := deps.Services["web"]; !ok {
		t.Error("service 'web' not found")
	}
}

func TestCreateConfigWithInfra(t *testing.T) {
	initI18n(t)

	uc := newTestUseCase()
	infra := map[string]config.InfraEntry{
		"postgres": {Inline: &config.Infra{Image: "postgres", Tag: "15", Ports: []string{"5432:5432"}}},
	}

	deps, err := uc.createConfig("my-project", "my-network", nil, infra)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(deps.Infra) != 1 {
		t.Fatalf("Infra count = %d, want 1", len(deps.Infra))
	}
}

func TestCreateConfigProducesValidJSON(t *testing.T) {
	initI18n(t)

	uc := newTestUseCase()
	deps, err := uc.createConfig("test-project", "test-network", nil, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var parsed config.Deps
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if parsed.Project.Name != "test-project" {
		t.Errorf("round-trip Project.Name = %s, want test-project", parsed.Project.Name)
	}
}

func TestWriteConfigFile(t *testing.T) {
	initI18n(t)

	uc := newTestUseCase()
	deps, _ := uc.createConfig("my-project", "my-network", nil, nil)

	t.Run("writes valid file", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, ".raioz.json")

		if err := uc.writeConfigFile(outputPath, deps); err != nil {
			t.Fatalf("error: %v", err)
		}

		data, _ := os.ReadFile(outputPath)
		var parsed config.Deps
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("written file is not valid JSON: %v", err)
		}
		if parsed.Project.Name != "my-project" {
			t.Errorf("written Project.Name = %s, want my-project", parsed.Project.Name)
		}
	})

	t.Run("error on invalid path", func(t *testing.T) {
		err := uc.writeConfigFile("/nonexistent/dir/config.json", deps)
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}
