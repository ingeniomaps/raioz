package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

// ADR-023 regression guard: a clean down must remove raioz.root.json.
func TestDownUseCase_downOrchestrated_DeletesRootConfig(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "raioz.yaml")
	_ = os.WriteFile(configPath, []byte("project: test"), 0o644)

	// Seed a workspace root with a populated raioz.root.json. The
	// mock workspace manager returns this same workspace so the use
	// case operates on it directly.
	wsRoot := filepath.Join(tmpDir, "ws-root")
	if err := os.MkdirAll(wsRoot, 0o755); err != nil {
		t.Fatalf("mkdir ws root: %v", err)
	}
	rootPath := filepath.Join(wsRoot, "raioz.root.json")
	if err := os.WriteFile(rootPath, []byte(`{"project":"test"}`), 0o644); err != nil {
		t.Fatalf("seed root.json: %v", err)
	}

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(cp string) (*models.Deps, []string, error) {
			return &models.Deps{
				SchemaVersion: "2.0",
				SourceFormat:  models.SourceFormatYAML,
				Project:       models.Project{Name: "test"},
				Network:       models.NetworkConfig{Name: "testnet"},
				Services:      map[string]models.Service{},
				Infra:         map[string]models.InfraEntry{},
			}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*interfaces.Workspace, error) {
			return &interfaces.Workspace{Root: wsRoot}, nil
		},
	}

	uc := NewDownUseCase(deps)
	if err := uc.downOrchestrated(context.Background(), DownOptions{ConfigPath: configPath}); err != nil {
		t.Fatalf("downOrchestrated() error = %v", err)
	}

	if _, err := os.Stat(rootPath); !os.IsNotExist(err) {
		t.Errorf("raioz.root.json still present after down: %v", err)
	}
}

func TestDownUseCase_downOrchestrated_NoConfig(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return nil, nil, nil
		},
	}
	uc := NewDownUseCase(deps)
	err := uc.downOrchestrated(context.Background(), DownOptions{})
	if err != nil {
		t.Fatalf("expected nil when config cannot load, got %v", err)
	}
}

func TestDownUseCase_downOrchestrated_LegacySchema(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
			return &models.Deps{
				SchemaVersion: "1.0",
				Project:       models.Project{Name: "test"},
			}, nil, nil
		},
	}
	uc := NewDownUseCase(deps)
	err := uc.downOrchestrated(context.Background(), DownOptions{ConfigPath: "raioz.yaml"})
	if err != nil {
		t.Fatalf("expected nil for legacy schema, got %v", err)
	}
}

func TestDownUseCase_downOrchestrated_YAMLProject(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "raioz.yaml")
	_ = os.WriteFile(configPath, []byte("project: test"), 0644)

	// Create a local state with a PID
	ls := &models.LocalState{
		HostPIDs: map[string]int{"api": 99999999}, // non-existent PID
	}
	_ = state.SaveLocalState(tmpDir, ls)

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(cp string) (*models.Deps, []string, error) {
			return &models.Deps{
				SchemaVersion: "2.0",
				SourceFormat:  models.SourceFormatYAML,
				Project:       models.Project{Name: "test"},
				Network:       models.NetworkConfig{Name: "testnet"},
				Services: map[string]models.Service{
					"api": {
						Source: models.SourceConfig{Path: tmpDir},
						Commands: &models.ServiceCommands{
							Down: "echo stopping",
						},
					},
				},
				Infra: map[string]models.InfraEntry{
					"redis": {Inline: &models.Infra{Image: "redis:7"}},
				},
			}, nil, nil
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		IsNetworkInUseWithContextFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil // network in use, so skip removal
		},
	}

	uc := NewDownUseCase(deps)
	err := uc.downOrchestrated(context.Background(), DownOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownUseCase_downOrchestrated_NetworkNotInUse(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "raioz.yaml")
	_ = os.WriteFile(configPath, []byte("project: test"), 0644)

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(cp string) (*models.Deps, []string, error) {
			return &models.Deps{
				SchemaVersion: "2.0",
				SourceFormat:  models.SourceFormatYAML,
				Project:       models.Project{Name: "test"},
				Network:       models.NetworkConfig{Name: "testnet"},
				Services:      map[string]models.Service{},
				Infra:         map[string]models.InfraEntry{},
			}, nil, nil
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		IsNetworkInUseWithContextFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
	}

	uc := NewDownUseCase(deps)
	err := uc.downOrchestrated(context.Background(), DownOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownUseCase_downOrchestrated_ServiceWithStopCommand(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "raioz.yaml")
	_ = os.WriteFile(configPath, []byte("project: test"), 0644)

	// Create state with PID for a service that has a stop command
	ls := &models.LocalState{
		HostPIDs: map[string]int{"api": 99999999},
	}
	_ = state.SaveLocalState(tmpDir, ls)

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(cp string) (*models.Deps, []string, error) {
			return &models.Deps{
				SchemaVersion: "2.0",
				SourceFormat:  models.SourceFormatYAML,
				Project:       models.Project{Name: "test"},
				Network:       models.NetworkConfig{},
				Services: map[string]models.Service{
					"api": {
						Commands: &models.ServiceCommands{
							Down: "echo stop", // has stop command
						},
					},
				},
				Infra: map[string]models.InfraEntry{},
			}, nil, nil
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		IsNetworkInUseWithContextFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}

	uc := NewDownUseCase(deps)
	// The service has a stop command, so the PID-based killProcessGroup should be skipped
	err := uc.downOrchestrated(context.Background(), DownOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCustomStopCommands_NoCommands(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {}, // no commands
		},
	}
	// Should not panic
	runCustomStopCommands(context.Background(), deps, "/tmp")
}

func TestRunCustomStopCommands_WithCommand(t *testing.T) {
	tmpDir := t.TempDir()
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {
				Commands: &models.ServiceCommands{
					Down: "echo stopped",
				},
			},
		},
	}
	runCustomStopCommands(context.Background(), deps, tmpDir)
}

func TestRunCustomStopCommands_WithEnvFiles(t *testing.T) {
	tmpDir := t.TempDir()
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {
				Commands: &models.ServiceCommands{
					Down: "echo stopped",
				},
				Env: &models.EnvValue{
					Files: []string{".env"},
				},
			},
		},
	}
	runCustomStopCommands(context.Background(), deps, tmpDir)
}

func TestRunCustomStopCommands_FailingCommand(t *testing.T) {
	tmpDir := t.TempDir()
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {
				Commands: &models.ServiceCommands{
					Down: "false", // exits with 1
				},
			},
			"web": {
				Commands: &models.ServiceCommands{
					Down: "true", // exits with 0
				},
			},
		},
	}
	// Failures must surface, but every service still gets
	// a stop attempt (best-effort teardown).
	failed := runCustomStopCommands(context.Background(), deps, tmpDir)
	if len(failed) != 1 || failed[0] != "api" {
		t.Errorf("runCustomStopCommands() failed = %v, want [api]", failed)
	}
}

func TestRunCustomStopCommands_EmptyCommand(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{
			"api": {
				Commands: &models.ServiceCommands{
					Down: "   ", // whitespace only
				},
			},
		},
	}
	// Should be skipped (len(parts) == 0 after Fields)
	runCustomStopCommands(context.Background(), deps, "/tmp")
}
