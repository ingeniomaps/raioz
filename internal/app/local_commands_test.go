package app

import (
	"context"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestIsLocalProject_Local(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "raioz.yaml")
	baseDir := t.TempDir()
	isLocal, projectDir, err := IsLocalProject(configPath, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLocal {
		t.Error("expected isLocal true for path outside baseDir")
	}
	if projectDir != tmpDir {
		t.Errorf("expected projectDir %q, got %q", tmpDir, projectDir)
	}
}

func TestIsLocalProject_InWorkspacesDir(t *testing.T) {
	baseDir := t.TempDir()
	wsDir := filepath.Join(baseDir, "workspaces", "proj")
	configPath := filepath.Join(wsDir, "raioz.yaml")
	isLocal, _, err := IsLocalProject(configPath, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isLocal {
		t.Error("expected isLocal false for path inside workspaces dir")
	}
}

func TestIsLocalProject_InServicesDir(t *testing.T) {
	baseDir := t.TempDir()
	svcDir := filepath.Join(baseDir, "services", "api")
	configPath := filepath.Join(svcDir, "raioz.yaml")
	isLocal, _, err := IsLocalProject(configPath, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isLocal {
		t.Error("expected isLocal false for path inside services dir")
	}
}

func TestGetLocalProjectCommand(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		cmdType string
		mode    string
		want    string
	}{
		{
			name:    "no commands returns empty",
			deps:    &config.Deps{Project: config.Project{Name: "test"}},
			cmdType: "up",
			mode:    "dev",
			want:    "",
		},
		{
			name: "generic up",
			deps: &config.Deps{
				Project: config.Project{
					Name: "test",
					Commands: &config.ProjectCommands{
						Up: "make up",
					},
				},
			},
			cmdType: "up",
			mode:    "dev",
			want:    "make up",
		},
		{
			name: "dev up overrides generic",
			deps: &config.Deps{
				Project: config.Project{
					Name: "test",
					Commands: &config.ProjectCommands{
						Up: "make up",
						Dev: &config.EnvironmentCommands{
							Up: "make dev",
						},
					},
				},
			},
			cmdType: "up",
			mode:    "dev",
			want:    "make dev",
		},
		{
			name: "prod down",
			deps: &config.Deps{
				Project: config.Project{
					Name: "test",
					Commands: &config.ProjectCommands{
						Down: "make down",
						Prod: &config.EnvironmentCommands{
							Down: "make prod-down",
						},
					},
				},
			},
			cmdType: "down",
			mode:    "prod",
			want:    "make prod-down",
		},
		{
			name: "health generic",
			deps: &config.Deps{
				Project: config.Project{
					Name: "test",
					Commands: &config.ProjectCommands{
						Health: "curl ok",
					},
				},
			},
			cmdType: "health",
			mode:    "dev",
			want:    "curl ok",
		},
		{
			name: "unknown command type",
			deps: &config.Deps{
				Project: config.Project{
					Name: "test",
					Commands: &config.ProjectCommands{
						Up: "make up",
					},
				},
			},
			cmdType: "unknown",
			mode:    "dev",
			want:    "",
		},
		{
			name: "empty mode defaults to dev",
			deps: &config.Deps{
				Project: config.Project{
					Name: "test",
					Commands: &config.ProjectCommands{
						Up: "generic",
						Dev: &config.EnvironmentCommands{
							Up: "dev-up",
						},
					},
				},
			},
			cmdType: "up",
			mode:    "",
			want:    "dev-up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLocalProjectCommand(tt.deps, tt.cmdType, tt.mode)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExecuteLocalProjectCommand_Empty(t *testing.T) {
	err := ExecuteLocalProjectCommand(context.Background(), "/tmp", "", "dev")
	if err != nil {
		t.Errorf("expected nil for empty command, got %v", err)
	}
}

func TestExecuteLocalProjectCommand_Simple(t *testing.T) {
	tmpDir := t.TempDir()
	// Use 'true' which always succeeds on unix
	err := ExecuteLocalProjectCommand(context.Background(), tmpDir, "true", "dev")
	if err != nil {
		t.Errorf("expected nil for 'true', got %v", err)
	}
}

func TestCheckLocalProjectHealth_Empty(t *testing.T) {
	healthy, err := CheckLocalProjectHealth(context.Background(), "/tmp", "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if healthy {
		t.Error("expected not healthy for empty command")
	}
}

func TestCheckLocalProjectHealth_Success(t *testing.T) {
	tmpDir := t.TempDir()
	healthy, err := CheckLocalProjectHealth(context.Background(), tmpDir, "true")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !healthy {
		t.Error("expected healthy for 'true'")
	}
}

func TestCheckLocalProjectHealth_Failure(t *testing.T) {
	tmpDir := t.TempDir()
	healthy, _ := CheckLocalProjectHealth(context.Background(), tmpDir, "false")
	if healthy {
		t.Error("expected not healthy for 'false'")
	}
}
