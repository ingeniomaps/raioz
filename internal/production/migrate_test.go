package production

import (
	"strings"
	"testing"

	"raioz/internal/config"
)

func TestMigrateComposeToDeps(t *testing.T) {
	prod := &ProductionConfig{
		Services: map[string]ProductionService{
			"web": {
				Image: "nginx:1.25",
				Ports: []string{"80:80"},
			},
			"postgres": {
				Image:   "postgres:16",
				Ports:   []string{"5432:5432"},
				Volumes: []string{"./data:/var/lib/postgresql/data"},
			},
			"api": {
				Image:     "myorg/api:v1",
				DependsOn: []interface{}{"postgres"},
			},
		},
	}

	deps, err := MigrateComposeToDeps(prod, "demo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps.Project.Name != "demo" {
		t.Errorf("project = %q, want demo", deps.Project.Name)
	}
	if deps.Network.GetName() != "demo-network" {
		t.Errorf("network = %q, want demo-network (auto)", deps.Network.GetName())
	}
	if _, ok := deps.Infra["postgres"]; !ok {
		t.Error("postgres should be classified as infra")
	}
	if _, ok := deps.Services["web"]; !ok {
		t.Error("web should be a service")
	}
	if svc, ok := deps.Services["api"]; !ok {
		t.Error("api should be a service")
	} else if len(svc.Docker.DependsOn) == 0 {
		t.Error("api dependsOn lost during migration")
	}
}

func TestMigrateComposeToDeps_RequiresProjectName(t *testing.T) {
	if _, err := MigrateComposeToDeps(&ProductionConfig{}, "", ""); err == nil {
		t.Error("expected error when project name missing")
	}
}

func TestValidateMigratedDeps(t *testing.T) {
	t.Run("no warnings on minimal valid", func(t *testing.T) {
		deps := &config.Deps{
			SchemaVersion: "1.0",
			Network:       config.NetworkConfig{Name: "n"},
			Project:       config.Project{Name: "p"},
			Services: map[string]config.Service{
				"api": {
					Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.25"},
					Docker: &config.DockerConfig{Mode: "prod"},
				},
			},
		}
		warnings := ValidateMigratedDeps(deps)
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}
	})

	t.Run("warns when fields missing", func(t *testing.T) {
		deps := &config.Deps{
			SchemaVersion: "0.1",
			Services: map[string]config.Service{
				"api": {
					Source: config.SourceConfig{Kind: "image"},
					Docker: &config.DockerConfig{},
				},
			},
		}
		warnings := ValidateMigratedDeps(deps)
		if len(warnings) == 0 {
			t.Error("expected warnings for missing project, network, image")
		}
	})

	t.Run("warns on missing dependency reference", func(t *testing.T) {
		deps := &config.Deps{
			SchemaVersion: "1.0",
			Network:       config.NetworkConfig{Name: "n"},
			Project:       config.Project{Name: "p"},
			Services: map[string]config.Service{
				"api": {
					Source: config.SourceConfig{Kind: "image", Image: "x", Tag: "1"},
					Docker: &config.DockerConfig{Mode: "prod", DependsOn: []string{"ghost"}},
				},
			},
		}
		warnings := ValidateMigratedDeps(deps)
		joined := strings.Join(warnings, " ")
		if !strings.Contains(joined, "ghost") {
			t.Errorf("expected warning about missing 'ghost' dependency, got: %v", warnings)
		}
	})
}

func TestSuggestGitSource(t *testing.T) {
	t.Run("returns suggestion for gcr image", func(t *testing.T) {
		got := SuggestGitSource("api", "gcr.io/project/api:v1")
		if got == nil {
			t.Fatal("expected suggestion for gcr image")
		}
		if got.Kind != "git" {
			t.Errorf("kind = %q, want git", got.Kind)
		}
	})

	t.Run("returns nil for arbitrary image", func(t *testing.T) {
		got := SuggestGitSource("api", "redis")
		if got != nil {
			t.Errorf("expected nil for unknown image source, got %+v", got)
		}
	})
}

func TestEnhanceMigratedDeps_NoPanic(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image", Image: "gcr.io/p/api"}},
		},
	}
	EnhanceMigratedDeps(deps) // best-effort, no return value
}
