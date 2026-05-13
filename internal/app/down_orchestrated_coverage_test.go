package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

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
		},
	}
	// Should not panic, just log warning
	runCustomStopCommands(context.Background(), deps, tmpDir)
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
