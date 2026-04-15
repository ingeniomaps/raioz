package app

import (
	"context"
	"testing"

	"raioz/internal/config"
)

func TestHandleLocalProjectDown_LoadFails(t *testing.T) {
	initI18nForTest(t)
	handled, err := HandleLocalProjectDown(context.Background(), "/nonexistent/raioz.yaml", "/tmp", nil)
	if handled {
		t.Error("expected handled=false when config cannot load")
	}
	_ = err
}

func TestHandleLocalProjectDown_NotLocal(t *testing.T) {
	initI18nForTest(t)
	// Create a temporary config inside the workspace dir (not local)
	tmpDir := t.TempDir()
	// Simulate baseDir that contains the config path (making it non-local)
	handled, _ := HandleLocalProjectDown(context.Background(), tmpDir+"/workspaces/proj/raioz.yaml", tmpDir, nil)
	if handled {
		t.Error("expected handled=false for non-local project")
	}
}

func TestGetLocalProjectCommand_AllModes(t *testing.T) {
	tests := []struct {
		name     string
		deps     *config.Deps
		cmdType  string
		mode     string
		expected string
	}{
		{
			name:     "nil commands",
			deps:     &config.Deps{Project: config.Project{Commands: nil}},
			cmdType:  "up",
			mode:     "dev",
			expected: "",
		},
		{
			name: "prod up",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{
						Up:   "make dev",
						Prod: &config.EnvironmentCommands{Up: "make prod"},
					},
				},
			},
			cmdType:  "up",
			mode:     "prod",
			expected: "make prod",
		},
		{
			name: "dev down",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{
						Down: "make down",
						Dev:  &config.EnvironmentCommands{Down: "make dev-down"},
					},
				},
			},
			cmdType:  "down",
			mode:     "dev",
			expected: "make dev-down",
		},
		{
			name: "prod down fallback",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{
						Down: "make down",
					},
				},
			},
			cmdType:  "down",
			mode:     "prod",
			expected: "make down",
		},
		{
			name: "health prod",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{
						Health: "curl /health",
						Prod:   &config.EnvironmentCommands{Health: "curl /prod-health"},
					},
				},
			},
			cmdType:  "health",
			mode:     "prod",
			expected: "curl /prod-health",
		},
		{
			name: "health dev fallback",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{
						Health: "curl /health",
					},
				},
			},
			cmdType:  "health",
			mode:     "dev",
			expected: "curl /health",
		},
		{
			name: "unknown command type",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{Up: "make up"},
				},
			},
			cmdType:  "unknown",
			mode:     "dev",
			expected: "",
		},
		{
			name: "empty mode defaults to dev",
			deps: &config.Deps{
				Project: config.Project{
					Commands: &config.ProjectCommands{
						Dev: &config.EnvironmentCommands{Up: "make dev-up"},
					},
				},
			},
			cmdType:  "up",
			mode:     "",
			expected: "make dev-up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLocalProjectCommand(tt.deps, tt.cmdType, tt.mode)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestExecuteLocalProjectCommand_EmptyCommand(t *testing.T) {
	err := ExecuteLocalProjectCommand(context.Background(), "/tmp", "", "dev")
	if err != nil {
		t.Errorf("expected nil for empty command, got %v", err)
	}
}

func TestExecuteLocalProjectCommand_Success(t *testing.T) {
	err := ExecuteLocalProjectCommand(context.Background(), "/tmp", "true", "dev")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteLocalProjectCommand_MultiPart(t *testing.T) {
	err := ExecuteLocalProjectCommand(context.Background(), "/tmp", "echo hello world", "prod")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckLocalProjectHealth_EmptyCommand(t *testing.T) {
	ok, err := CheckLocalProjectHealth(context.Background(), "/tmp", "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for empty health command")
	}
}

func TestCheckLocalProjectHealth_Passing(t *testing.T) {
	ok, err := CheckLocalProjectHealth(context.Background(), "/tmp", "true")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for passing health check")
	}
}

func TestCheckLocalProjectHealth_Failing(t *testing.T) {
	ok, err := CheckLocalProjectHealth(context.Background(), "/tmp", "false")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for failing health check")
	}
}

func TestCheckLocalProjectHealth_MultiPart(t *testing.T) {
	ok, err := CheckLocalProjectHealth(context.Background(), "/tmp", "echo health ok")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for passing multi-part health check")
	}
}
