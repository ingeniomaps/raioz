package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/state"
)

// With the project down there is nothing to swap: dev must refuse before
// touching anything, not leave a half-created container behind.
func TestDevPromote_RefusesWhenDependencyIsNotRunning(t *testing.T) {
	initI18nForTest(t)
	stubContainerLookup(t, &projectLabelLookup{})

	projectDir := t.TempDir()
	local := filepath.Join(projectDir, "localkv")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "Dockerfile"), []byte("FROM redis:7-alpine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &models.Deps{
		Project: models.Project{Name: "bencha"},
		Infra:   map[string]models.InfraEntry{"kv": {Inline: &models.Infra{Image: "redis", Tag: "7.4-alpine"}}},
	}
	localState := &models.LocalState{Project: "bencha"}

	err := NewDevUseCase(&Dependencies{}).promote(context.Background(), "kv", local, cfg, projectDir, localState)
	if err == nil {
		t.Fatal("expected dev to refuse a dependency that is not running")
	}
	if localState.IsDevOverridden("kv") {
		t.Error("override recorded for a promotion that did not happen")
	}
	if saved, _ := state.LoadLocalState(projectDir); saved != nil && saved.IsDevOverridden("kv") {
		t.Error("override persisted for a promotion that did not happen")
	}
}
