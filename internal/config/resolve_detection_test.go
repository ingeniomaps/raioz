package config

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

func TestResolveServiceDetection_ComposeOverride(t *testing.T) {
	svc := Service{
		Source: SourceConfig{
			ComposeFiles: []string{"docker-compose.yml", "docker-compose.dev.yml"},
		},
	}

	result := ResolveServiceDetection(svc, "/some/path")

	if result.Runtime != models.RuntimeCompose {
		t.Errorf("runtime = %q, want %q", result.Runtime, models.RuntimeCompose)
	}
	if result.ComposeFile != "docker-compose.yml" {
		t.Errorf("ComposeFile = %q, want %q", result.ComposeFile, "docker-compose.yml")
	}
	if len(result.ComposeFiles) != 2 {
		t.Errorf("ComposeFiles len = %d, want 2", len(result.ComposeFiles))
	}
}

func TestResolveServiceDetection_CommandOverride(t *testing.T) {
	svc := Service{
		Source: SourceConfig{
			Command: "make dev",
		},
	}

	result := ResolveServiceDetection(svc, "/some/path")

	if result.Runtime != models.RuntimeMake {
		t.Errorf("runtime = %q, want %q", result.Runtime, models.RuntimeMake)
	}
	if result.StartCommand != "make dev" {
		t.Errorf("StartCommand = %q, want %q", result.StartCommand, "make dev")
	}
	if result.DevCommand != "make dev" {
		t.Errorf("DevCommand = %q, want %q", result.DevCommand, "make dev")
	}
}

func TestResolveServiceDetection_EmptyPath(t *testing.T) {
	svc := Service{}
	result := ResolveServiceDetection(svc, "")
	if result.Runtime != models.RuntimeUnknown {
		t.Errorf("runtime = %q, want %q", result.Runtime, models.RuntimeUnknown)
	}
}

func TestResolveServiceDetection_FallbackDetect(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a Makefile so detect.Detect returns RuntimeMake
	os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("all:\n\techo ok\n"), 0o644)

	svc := Service{}
	result := ResolveServiceDetection(svc, tmpDir)

	if result.Runtime != models.RuntimeMake {
		t.Errorf("runtime = %q, want %q (Makefile should be detected)", result.Runtime, models.RuntimeMake)
	}
}

func TestResolveServiceDetection_ComposeOverrideWinsOverCommand(t *testing.T) {
	// ComposeFiles takes precedence over Command
	svc := Service{
		Source: SourceConfig{
			ComposeFiles: []string{"compose.yml"},
			Command:      "make dev",
		},
	}

	result := ResolveServiceDetection(svc, "/some/path")
	if result.Runtime != models.RuntimeCompose {
		t.Errorf("runtime = %q, want %q (compose should win over command)", result.Runtime, models.RuntimeCompose)
	}
}

func TestResolveServiceDetection_ComposeFilesAreCopied(t *testing.T) {
	original := []string{"a.yml", "b.yml"}
	svc := Service{
		Source: SourceConfig{ComposeFiles: original},
	}

	result := ResolveServiceDetection(svc, "")

	// Modify original to verify it was copied
	original[0] = "modified.yml"
	if result.ComposeFiles[0] == "modified.yml" {
		t.Error("ComposeFiles should be a copy, not a reference to the original")
	}
}
