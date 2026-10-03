package upcase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

// dev stops and restarts what up started. Its context has to carry the
// same project, container name and volumes, or the runner addresses
// something else and the swap silently does nothing.
func TestDependencyContext_MatchesWhatUpStarts(t *testing.T) {
	projectDir := t.TempDir()
	deps := &models.Deps{
		Project: models.Project{Name: "bencha"},
		Network: models.NetworkConfig{Name: "bencha-net"},
		Infra: map[string]models.InfraEntry{
			"kv": {Inline: &models.Infra{
				Image: "redis", Tag: "7.4-alpine", Volumes: []string{"kvdata:/data"},
			}},
		},
	}

	got, ok := DependencyContext(context.Background(), deps, "kv", projectDir)
	if !ok {
		t.Fatal("kv not recognised as a dependency")
	}
	if got.ProjectName != "bencha" || got.NetworkName != "bencha-net" {
		t.Errorf("project/network = %q/%q", got.ProjectName, got.NetworkName)
	}
	if got.ContainerName != "raioz-bencha-kv" {
		t.Errorf("container = %q, want raioz-bencha-kv", got.ContainerName)
	}
	if got.EnvVars["RAIOZ_IMAGE"] != "redis:7.4-alpine" {
		t.Errorf("image = %q", got.EnvVars["RAIOZ_IMAGE"])
	}
	if len(got.Volumes) != 1 || got.ProjectDir != projectDir {
		t.Errorf("volumes = %v, projectDir = %q", got.Volumes, got.ProjectDir)
	}

	if _, ok := DependencyContext(context.Background(), deps, "ghost", projectDir); ok {
		t.Error("an unknown name was accepted as a dependency")
	}
}

func TestDevOverrideContext(t *testing.T) {
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "Dockerfile"), []byte("FROM redis:7-alpine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	depCtx, _ := DependencyContext(context.Background(), &models.Deps{
		Project: models.Project{Name: "bencha"},
		Network: models.NetworkConfig{Name: "bencha-net"},
		Infra: map[string]models.InfraEntry{
			"kv": {Inline: &models.Infra{Image: "redis", Tag: "7.4-alpine", Ports: []string{"36380:6379"}}},
		},
	}, "kv", t.TempDir())

	got := DevOverrideContext(depCtx, local)

	if got.Path != local || got.Detection.Runtime != models.RuntimeDockerfile {
		t.Errorf("path/runtime = %q/%q, want the local Dockerfile build", got.Path, got.Detection.Runtime)
	}
	// Everything that reached the image must reach the local build.
	if got.ContainerName != depCtx.ContainerName || got.NetworkName != depCtx.NetworkName ||
		got.ProjectName != depCtx.ProjectName || len(got.Ports) != len(depCtx.Ports) {
		t.Errorf("identity changed: %+v", got)
	}
	if _, kept := got.EnvVars["RAIOZ_IMAGE"]; kept {
		t.Error("the local build still carries the image reference")
	}
	if depCtx.EnvVars["RAIOZ_IMAGE"] == "" {
		t.Error("building the override altered the dependency's own context")
	}
}
