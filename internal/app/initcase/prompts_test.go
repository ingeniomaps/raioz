package initcase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test helpers are in helpers_test.go

func TestPromptString(t *testing.T) {
	initI18n(t)

	tests := []struct {
		name         string
		input        string
		defaultValue string
		want         string
	}{
		{
			name:         "uses default on empty input",
			input:        "\n",
			defaultValue: "my-project",
			want:         "my-project",
		},
		{
			name:         "uses custom value",
			input:        "custom-name\n",
			defaultValue: "my-project",
			want:         "custom-name",
		},
		{
			name:         "trims whitespace",
			input:        "  spaced  \n",
			defaultValue: "default",
			want:         "spaced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, _ := newTestUseCaseWithIO(tt.input)

			got, err := uc.promptString("Test prompt", tt.defaultValue)
			if err != nil {
				t.Fatalf("promptString() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("promptString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPromptStringShowsPrompt(t *testing.T) {
	initI18n(t)

	uc, out := newTestUseCaseWithIO("value\n")

	uc.promptString("Project name", "default-val")

	output := out.String()
	if !strings.Contains(output, "Project name") {
		t.Errorf("output should contain prompt text\ngot: %s", output)
	}
	if !strings.Contains(output, "default-val") {
		t.Errorf("output should contain default value\ngot: %s", output)
	}
}

func TestCheckFileExists(t *testing.T) {
	initI18n(t)

	t.Run("file does not exist returns true", func(t *testing.T) {
		uc, _ := newTestUseCaseWithIO("")
		result, err := uc.checkFileExists("/nonexistent/file.json")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !result {
			t.Error("should return true when file does not exist")
		}
	})

	t.Run("file exists user says yes", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "existing.json")
		os.WriteFile(existingFile, []byte("{}"), 0644)

		uc, _ := newTestUseCaseWithIO("y\n")
		result, err := uc.checkFileExists(existingFile)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !result {
			t.Error("should return true when user confirms")
		}
	})

	t.Run("file exists user says yes full word", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "existing.json")
		os.WriteFile(existingFile, []byte("{}"), 0644)

		uc, _ := newTestUseCaseWithIO("yes\n")
		result, err := uc.checkFileExists(existingFile)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !result {
			t.Error("should return true when user says 'yes'")
		}
	})

	t.Run("file exists user says no", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "existing.json")
		os.WriteFile(existingFile, []byte("{}"), 0644)

		uc, _ := newTestUseCaseWithIO("n\n")
		result, err := uc.checkFileExists(existingFile)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result {
			t.Error("should return false when user declines")
		}
	})

	t.Run("file exists user presses enter", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "existing.json")
		os.WriteFile(existingFile, []byte("{}"), 0644)

		uc, _ := newTestUseCaseWithIO("\n")
		result, err := uc.checkFileExists(existingFile)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result {
			t.Error("should return false on empty input (default no)")
		}
	})

	t.Run("file exists shows overwrite prompt", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "existing.json")
		os.WriteFile(existingFile, []byte("{}"), 0644)

		uc, out := newTestUseCaseWithIO("n\n")
		uc.checkFileExists(existingFile)

		output := out.String()
		if !strings.Contains(output, "existing.json") {
			t.Errorf("prompt should mention filename\ngot: %s", output)
		}
	})
}

func TestPromptProjectInfo(t *testing.T) {
	initI18n(t)

	t.Run("default values", func(t *testing.T) {
		uc, _ := newTestUseCaseWithIO("\n\n")
		project, network, err := uc.promptProjectInfo()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if project != "my-project" {
			t.Errorf("project = %q, want my-project", project)
		}
		if network != "my-project-network" {
			t.Errorf("network = %q, want my-project-network", network)
		}
	})

	t.Run("custom project derives network default", func(t *testing.T) {
		uc, _ := newTestUseCaseWithIO("billing\n\n")
		project, network, err := uc.promptProjectInfo()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if project != "billing" {
			t.Errorf("project = %q, want billing", project)
		}
		if network != "billing-network" {
			t.Errorf("network = %q, want billing-network", network)
		}
	})

	t.Run("fully custom values", func(t *testing.T) {
		uc, _ := newTestUseCaseWithIO("my-app\ncustom-net\n")
		project, network, err := uc.promptProjectInfo()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if project != "my-app" {
			t.Errorf("project = %q, want my-app", project)
		}
		if network != "custom-net" {
			t.Errorf("network = %q, want custom-net", network)
		}
	})
}

func TestShowWelcomeMessage(t *testing.T) {
	initI18n(t)

	uc, out := newTestUseCaseWithIO("")
	uc.showWelcomeMessage()

	output := out.String()
	if !strings.Contains(output, "Welcome") {
		t.Errorf("welcome message missing\ngot: %s", output)
	}
	if !strings.Contains(output, "wizard") || !strings.Contains(output, "help") {
		t.Errorf("wizard help message missing\ngot: %s", output)
	}
}

func TestShowSuccessMessage(t *testing.T) {
	initI18n(t)

	uc, out := newTestUseCaseWithIO("")
	uc.showSuccessMessage("test-output.json")

	output := out.String()
	if !strings.Contains(output, "test-output.json") {
		t.Errorf("output path missing\ngot: %s", output)
	}
	if !strings.Contains(output, "raioz up") {
		t.Errorf("next steps missing 'raioz up'\ngot: %s", output)
	}
	if !strings.Contains(output, "raioz --help") {
		t.Errorf("next steps missing 'raioz --help'\ngot: %s", output)
	}
}
