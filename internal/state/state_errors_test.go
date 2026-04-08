package state

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/workspace"
)

func TestLoadCorruptedState(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws := &workspace.Workspace{
			Root:        tmpDir,
			ServicesDir: filepath.Join(tmpDir, "services"),
		}

		statePath := filepath.Join(ws.Root, stateFileName)
		os.WriteFile(statePath, []byte("{ invalid json }"), 0600)

		_, err := Load(ws)
		if err == nil {
			t.Error("Expected error loading corrupted state, got nil")
		}
	})

	t.Run("valid JSON but invalid structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		ws := &workspace.Workspace{
			Root:        tmpDir,
			ServicesDir: filepath.Join(tmpDir, "services"),
		}

		statePath := filepath.Join(ws.Root, stateFileName)
		os.WriteFile(statePath, []byte(`{"invalid": "structure"}`), 0600)

		// May succeed (unmarshal to Deps), but structure might be invalid
		_, _ = Load(ws)
	})
}
