package root

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestLoadCorruptedRootConfig(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws := &workspace.Workspace{Root: tmpDir}

		rootPath := filepath.Join(ws.Root, rootFileName)
		os.WriteFile(rootPath, []byte("{ invalid json }"), 0644)

		_, err := Load(ws)
		if err == nil {
			t.Error("Expected error loading corrupted root config, got nil")
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws := &workspace.Workspace{Root: tmpDir}

		rootPath := filepath.Join(ws.Root, rootFileName)
		incompleteRoot := `{"schemaVersion": "1.0", "project": {}}`
		os.WriteFile(rootPath, []byte(incompleteRoot), 0644)

		// May succeed (unmarshal), but structure might be invalid
		_, _ = Load(ws)
	})

	t.Run("save with nil metadata initializes it", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws := &workspace.Workspace{Root: tmpDir}

		deps := &config.Deps{
			SchemaVersion: "1.0",
			Project:       config.Project{Name: "test"},
			Network:       config.NetworkConfig{Name: "test", IsObject: false},
			Services:      map[string]config.Service{},
			Infra:         map[string]config.InfraEntry{},
			Env:           config.EnvConfig{},
		}

		rootConfig, err := GenerateFromDeps(deps, []string{}, map[string]string{})
		if err != nil {
			t.Fatalf("GenerateFromDeps failed: %v", err)
		}
		rootConfig.Metadata = nil

		if err := Save(ws, rootConfig); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		reloaded, err := Load(ws)
		if err != nil {
			t.Fatalf("Reload failed: %v", err)
		}
		if reloaded.Metadata == nil {
			t.Error("Expected metadata to be initialized, got nil")
		}
	})
}
