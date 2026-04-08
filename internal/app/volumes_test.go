package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func newTestDepsForVolumes(t *testing.T) (*Dependencies, *mocks.MockConfigLoader, *mocks.MockWorkspaceManager, *mocks.MockStateManager, *mocks.MockDockerRunner) {
	t.Helper()

	tmpDir := t.TempDir()

	configLoader := &mocks.MockConfigLoader{}
	wsMgr := &mocks.MockWorkspaceManager{
		ResolveFunc: func(projectName string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetComposePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/docker-compose.generated.yml"
		},
		GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string {
			return tmpDir
		},
	}
	stateMgr := &mocks.MockStateManager{
		ExistsFunc: func(ws *workspace.Workspace) bool { return false },
	}
	dockerRunner := &mocks.MockDockerRunner{}

	deps := &Dependencies{
		ConfigLoader:  configLoader,
		Workspace:     wsMgr,
		StateManager:  stateMgr,
		DockerRunner:  dockerRunner,
		Validator:     &mocks.MockValidator{},
		GitRepository: &mocks.MockGitRepository{},
		LockManager:   &mocks.MockLockManager{},
		HostRunner:    &mocks.MockHostRunner{},
		EnvManager:    &mocks.MockEnvManager{},
	}

	return deps, configLoader, wsMgr, stateMgr, dockerRunner
}

func TestVolumesUseCase_List_NoConfig(t *testing.T) {
	deps, configLoader, _, _, _ := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return nil, nil, fmt.Errorf("not found")
	}

	uc := NewVolumesUseCase(deps)
	err := uc.List(context.Background(), VolumesOptions{})

	if err == nil {
		t.Fatal("expected error when no config, got nil")
	}
}

func TestVolumesUseCase_List_NoVolumes(t *testing.T) {
	deps, configLoader, _, _, _ := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project:  config.Project{Name: "test"},
			Services: map[string]config.Service{},
		}, nil, nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.List(context.Background(), VolumesOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumesUseCase_List_WithVolumes(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"postgres": {Inline: &config.Infra{Volumes: []string{"pg-data:/var/lib/postgresql/data"}}},
			},
		}, nil, nil
	}

	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		var named []string
		for _, v := range volumes {
			if len(v) > 0 {
				named = append(named, "pg-data")
			}
		}
		return named, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return nil, nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.List(context.Background(), VolumesOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumesUseCase_Remove_NoTarget(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Volumes: []string{"redis-data:/data"}}},
			},
		}, nil, nil
	}
	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		return []string{"redis-data"}, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return nil, nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.Remove(context.Background(), VolumesRemoveOptions{})

	if err == nil {
		t.Fatal("expected error when no target specified, got nil")
	}
}

func TestVolumesUseCase_Remove_AllWithForce(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Volumes: []string{"redis-data:/data"}}},
			},
		}, nil, nil
	}
	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		return []string{"redis-data"}, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return nil, nil
	}

	var removedVolume string
	dockerRunner.RemoveVolumeWithContextFunc = func(ctx context.Context, name string) error {
		removedVolume = name
		return nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.Remove(context.Background(), VolumesRemoveOptions{
		All:   true,
		Force: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removedVolume != "test_redis-data" {
		t.Errorf("expected removed volume 'test_redis-data', got %q", removedVolume)
	}
}

func TestVolumesUseCase_Remove_SpecificVolume(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"postgres": {Inline: &config.Infra{Volumes: []string{"pg-data:/var/lib/postgresql/data"}}},
				"redis":    {Inline: &config.Infra{Volumes: []string{"redis-data:/data"}}},
			},
		}, nil, nil
	}
	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		var named []string
		for _, v := range volumes {
			parts := make([]string, 0)
			for i, c := range v {
				if c == ':' {
					parts = append(parts, v[:i])
					break
				}
			}
			if len(parts) > 0 {
				named = append(named, parts[0])
			}
		}
		return named, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return nil, nil
	}

	var removedVolumes []string
	dockerRunner.RemoveVolumeWithContextFunc = func(ctx context.Context, name string) error {
		removedVolumes = append(removedVolumes, name)
		return nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.Remove(context.Background(), VolumesRemoveOptions{
		Volumes: []string{"test_pg-data"},
		Force:   true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removedVolumes) != 1 || removedVolumes[0] != "test_pg-data" {
		t.Errorf("expected [test_pg-data], got %v", removedVolumes)
	}
}

func TestVolumesUseCase_Remove_VolumeNotFound(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Volumes: []string{"redis-data:/data"}}},
			},
		}, nil, nil
	}
	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		return []string{"redis-data"}, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return nil, nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.Remove(context.Background(), VolumesRemoveOptions{
		Volumes: []string{"nonexistent-volume"},
		Force:   true,
	})

	if err == nil {
		t.Fatal("expected error for non-existent volume, got nil")
	}
}

func TestVolumesUseCase_Remove_SharedVolumeExcluded(t *testing.T) {
	deps, configLoader, _, _, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Volumes: []string{"redis-data:/data"}}},
			},
		}, nil, nil
	}
	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		return []string{"redis-data"}, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return []string{"test", "other-project"}, nil
	}

	var removeCalled bool
	dockerRunner.RemoveVolumeWithContextFunc = func(ctx context.Context, name string) error {
		removeCalled = true
		return nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.Remove(context.Background(), VolumesRemoveOptions{
		All:   true,
		Force: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removeCalled {
		t.Error("should not remove volumes that are in use by other projects")
	}
}

func TestVolumesUseCase_List_FromState(t *testing.T) {
	deps, configLoader, _, stateMgr, dockerRunner := newTestDepsForVolumes(t)

	configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return &config.Deps{Project: config.Project{Name: "test"}}, nil, nil
	}
	stateMgr.ExistsFunc = func(ws *workspace.Workspace) bool { return true }
	stateMgr.LoadFunc = func(ws *workspace.Workspace) (*config.Deps, error) {
		return &config.Deps{
			Project: config.Project{Name: "test"},
			Infra: map[string]config.InfraEntry{
				"mongo": {Inline: &config.Infra{Volumes: []string{"mongo-data:/data/db"}}},
			},
		}, nil
	}
	dockerRunner.ExtractNamedVolumesFunc = func(volumes []string) ([]string, error) {
		if len(volumes) > 0 {
			return []string{"mongo-data"}, nil
		}
		return nil, nil
	}
	dockerRunner.NormalizeVolumeNameFunc = func(prefix string, name string) (string, error) {
		return prefix + "_" + name, nil
	}
	dockerRunner.GetVolumeProjectsFunc = func(volumeName string, baseDir string) ([]string, error) {
		return nil, nil
	}

	uc := NewVolumesUseCase(deps)
	err := uc.List(context.Background(), VolumesOptions{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
