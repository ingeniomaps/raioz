package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/state"
)

func TestRemoveIfHoldsNoFile(t *testing.T) {
	t.Run("a tree of empty directories goes", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "ws")
		if err := os.MkdirAll(filepath.Join(dir, "env", "services"), 0o755); err != nil {
			t.Fatal(err)
		}
		removeIfHoldsNoFile(dir)
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("empty tree should be removed, stat err = %v", err)
		}
	})

	t.Run("one file keeps the whole tree", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "ws")
		nested := filepath.Join(dir, "env", "services")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(nested, "global.env")
		if err := os.WriteFile(file, []byte("A=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		removeIfHoldsNoFile(dir)
		if _, err := os.Stat(file); err != nil {
			t.Errorf("a tree holding a file must be left alone: %v", err)
		}
	})

	t.Run("empty and missing paths are ignored", func(_ *testing.T) {
		removeIfHoldsNoFile("")
		removeIfHoldsNoFile(filepath.Join(os.TempDir(), "raioz-does-not-exist-tidy"))
	})
}

func TestTidyAfterDown_LocalState(t *testing.T) {
	tests := []struct {
		name     string
		state    *models.LocalState
		wantKept bool
	}{
		{"nothing worth keeping", &models.LocalState{Project: "p", NetworkName: "p-net"}, false},
		{
			"a dev override is the user's choice",
			&models.LocalState{Project: "p", DevOverrides: map[string]models.DevOverride{"db": {LocalPath: "./db"}}},
			true,
		},
		{"so is an ignored service", &models.LocalState{Project: "p", Ignored: []string{"web"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			if err := state.SaveLocalState(projectDir, tt.state); err != nil {
				t.Fatal(err)
			}
			tidyAfterDown(context.Background(), "", projectDir, "", tt.state)

			_, err := os.Stat(filepath.Join(projectDir, ".raioz.state.json"))
			if kept := err == nil; kept != tt.wantKept {
				t.Errorf("state file kept = %v, want %v", kept, tt.wantKept)
			}
		})
	}
}

func TestTidyAfterDown_DepFiles(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	project := "tidyproj"
	depFile := naming.DepComposePath(project, "kv")
	if err := os.MkdirAll(filepath.Dir(depFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(depFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tidyAfterDown(context.Background(), project, t.TempDir(), "", nil)

	if _, err := os.Stat(naming.TempDir(project)); !os.IsNotExist(err) {
		t.Errorf("generated dependency files must go with the project, stat err = %v", err)
	}
}
