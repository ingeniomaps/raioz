package app

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

// A git service has nothing to detect until `up` clones it; check must not
// call that a problem.
func TestAwaitingClone(t *testing.T) {
	empty := t.TempDir()
	cloned := t.TempDir()
	if err := os.WriteFile(filepath.Join(cloned, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(path string) models.Service {
		return models.Service{Source: models.SourceConfig{Kind: "git", Repo: "https://example.com/x.git", Path: path}}
	}
	tests := []struct {
		name string
		svc  models.Service
		want bool
	}{
		{"path does not exist yet", git(filepath.Join(empty, "missing")), true},
		{"path exists but is empty", git(empty), true},
		{"already cloned", git(cloned), false},
		{"a local service is never awaiting a clone",
			models.Service{Source: models.SourceConfig{Kind: "local", Path: filepath.Join(empty, "missing")}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := awaitingClone(tt.svc); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
