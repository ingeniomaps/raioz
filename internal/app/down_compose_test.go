package app

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/docker"
	"raioz/internal/domain/models"
)

func TestResolveComposeInvocation(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("host command service is not a compose stack", func(t *testing.T) {
		svc := models.Service{Source: models.SourceConfig{
			Path:    dir,
			Command: "npm run dev",
		}}
		if _, ok := resolveComposeInvocation(svc, "proj", "web"); ok {
			t.Fatal("command service must not resolve to a compose invocation")
		}
	})

	t.Run("no path and no compose files", func(t *testing.T) {
		if _, ok := resolveComposeInvocation(models.Service{}, "proj", "x"); ok {
			t.Fatal("empty service must not resolve")
		}
	})

	t.Run("explicit compose files", func(t *testing.T) {
		svc := models.Service{Source: models.SourceConfig{
			Path:         dir,
			ComposeFiles: []string{composeFile},
		}}
		inv, ok := resolveComposeInvocation(svc, "proj", "api")
		if !ok {
			t.Fatal("explicit compose files must resolve")
		}
		if want := "raioz-proj-api"; inv.projectName != want {
			t.Errorf("projectName = %q, want %q", inv.projectName, want)
		}
		if got := docker.SplitComposePaths(inv.spec); len(got) != 1 || got[0] != composeFile {
			t.Errorf("spec = %q, want single file %q", inv.spec, composeFile)
		}
	})

	t.Run("overlay appended when present", func(t *testing.T) {
		overlay := filepath.Join(dir, ".raioz-overlay.yml")
		if err := os.WriteFile(overlay, []byte("services: {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(overlay) })

		svc := models.Service{Source: models.SourceConfig{
			Path:         dir,
			ComposeFiles: []string{composeFile},
		}}
		inv, ok := resolveComposeInvocation(svc, "proj", "api")
		if !ok {
			t.Fatal("must resolve")
		}
		files := docker.SplitComposePaths(inv.spec)
		if len(files) != 2 || files[1] != overlay {
			t.Errorf("expected overlay appended, got spec %q", inv.spec)
		}
	})
}
