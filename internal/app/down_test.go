package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	raiozErrors "raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// testDownMocks holds all mocks used by DownUseCase tests.
type testDownMocks struct {
	configLoader *mocks.MockConfigLoader
	validator    *mocks.MockValidator
	dockerRunner *mocks.MockDockerRunner
	gitRepo      *mocks.MockGitRepository
	wsMgr        *mocks.MockWorkspaceManager
	stateMgr     *mocks.MockStateManager
	lockMgr      *mocks.MockLockManager
	hostRunner   *mocks.MockHostRunner
	envMgr       *mocks.MockEnvManager
}

// newTestDownDeps creates a Dependencies with all mocks pre-configured for the
// happy-path full-down scenario. Each test overrides what it needs.
func newTestDownDeps(tmpDir string) (*Dependencies, *testDownMocks) {
	ws := &workspace.Workspace{
		Root:        tmpDir,
		ServicesDir: filepath.Join(tmpDir, "services"),
		EnvDir:      filepath.Join(tmpDir, "env"),
	}

	statePath := filepath.Join(tmpDir, "state.json")
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	defaultDeps := &config.Deps{
		Project: config.Project{Name: "test-project"},
		Network: config.NetworkConfig{Name: "test-network"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "docker"}},
		},
	}

	m := &testDownMocks{
		configLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
				return defaultDeps, nil, nil
			},
		},
		validator: &mocks.MockValidator{},
		dockerRunner: &mocks.MockDockerRunner{
			DownWithContextFunc: func(ctx context.Context, cp string) error {
				return nil
			},
			IsNetworkInUseWithContextFunc: func(ctx context.Context, name string) (bool, error) {
				return false, nil
			},
			GetNetworkProjectsFunc: func(name string, baseDir string) ([]string, error) {
				return nil, nil
			},
			ExtractNamedVolumesFunc: func(volumes []string) ([]string, error) {
				return nil, nil
			},
			CleanUnusedImagesWithContextFunc: func(ctx context.Context, dryRun bool) ([]string, error) {
				return nil, nil
			},
			CleanUnusedVolumesWithContextFunc: func(ctx context.Context, dryRun bool, force bool) ([]string, error) {
				return nil, nil
			},
		},
		gitRepo: &mocks.MockGitRepository{},
		wsMgr: &mocks.MockWorkspaceManager{
			ResolveFunc: func(projectName string) (*workspace.Workspace, error) {
				return ws, nil
			},
			GetRootFunc: func(w *workspace.Workspace) string {
				return w.Root
			},
			GetComposePathFunc: func(w *workspace.Workspace) string {
				return composePath
			},
			GetStatePathFunc: func(w *workspace.Workspace) string {
				return statePath
			},
			GetBaseDirFromWorkspaceFunc: func(w *workspace.Workspace) string {
				return tmpDir
			},
		},
		stateMgr: &mocks.MockStateManager{
			ExistsFunc: func(w *workspace.Workspace) bool {
				return true
			},
			LoadFunc: func(w *workspace.Workspace) (*config.Deps, error) {
				return defaultDeps, nil
			},
			RemoveProjectFunc: func(projectName string) error {
				return nil
			},
		},
		lockMgr: &mocks.MockLockManager{
			AcquireFunc: func(w *workspace.Workspace) (interfaces.Lock, error) {
				return &mocks.MockLock{}, nil
			},
		},
		hostRunner: &mocks.MockHostRunner{
			LoadProcessesStateFunc: func(w *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
				return nil, nil
			},
			RemoveProcessesStateFunc: func(w *workspace.Workspace) error {
				return nil
			},
		},
		envMgr: &mocks.MockEnvManager{},
	}

	deps := NewDependenciesWithMocks(
		m.configLoader,
		m.validator,
		m.dockerRunner,
		m.gitRepo,
		m.wsMgr,
		m.stateMgr,
		m.lockMgr,
		m.hostRunner,
		m.envMgr,
	)

	return deps, m
}

func TestDownUseCase_Execute_ProjectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	deps, m := newTestDownDeps(tmpDir)

	// ConfigLoader returns nil deps, and no project name provided
	m.configLoader.LoadDepsFunc = func(configPath string) (*config.Deps, []string, error) {
		return nil, nil, nil
	}

	uc := NewDownUseCase(deps)
	err := uc.Execute(context.Background(), DownOptions{})

	if err == nil {
		t.Fatal("expected error when project cannot be determined, got nil")
	}

	var rErr *raiozErrors.RaiozError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected RaiozError, got %T: %v", err, err)
	}
	if rErr.Code != raiozErrors.ErrCodeInvalidConfig {
		t.Errorf("expected error code %s, got %s", raiozErrors.ErrCodeInvalidConfig, rErr.Code)
	}
}

func TestDownUseCase_Execute_WorkspaceResolveFails(t *testing.T) {
	tmpDir := t.TempDir()
	deps, m := newTestDownDeps(tmpDir)

	m.wsMgr.ResolveFunc = func(projectName string) (*workspace.Workspace, error) {
		return nil, fmt.Errorf("workspace not found")
	}

	uc := NewDownUseCase(deps)
	err := uc.Execute(context.Background(), DownOptions{ProjectName: "test-project"})

	if err == nil {
		t.Fatal("expected error when workspace resolve fails, got nil")
	}

	var rErr *raiozErrors.RaiozError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected RaiozError, got %T: %v", err, err)
	}
	if rErr.Code != raiozErrors.ErrCodeWorkspaceError {
		t.Errorf("expected error code %s, got %s", raiozErrors.ErrCodeWorkspaceError, rErr.Code)
	}
}

func TestDownUseCase_Execute_NoState(t *testing.T) {
	tmpDir := t.TempDir()
	deps, m := newTestDownDeps(tmpDir)

	// State does not exist and no host processes
	m.stateMgr.ExistsFunc = func(w *workspace.Workspace) bool {
		return false
	}

	uc := NewDownUseCase(deps)
	err := uc.Execute(context.Background(), DownOptions{ProjectName: "test-project"})

	if err != nil {
		t.Fatalf("expected nil error when no state exists, got: %v", err)
	}
}

func TestDownUseCase_Execute_FullDown(t *testing.T) {
	tmpDir := t.TempDir()
	deps, m := newTestDownDeps(tmpDir)

	// Create a fake state file so os.Remove succeeds
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	downCalled := false
	m.dockerRunner.DownWithContextFunc = func(ctx context.Context, cp string) error {
		downCalled = true
		return nil
	}

	removeCalled := false
	m.stateMgr.RemoveProjectFunc = func(projectName string) error {
		removeCalled = true
		return nil
	}

	uc := NewDownUseCase(deps)
	err := uc.Execute(context.Background(), DownOptions{
		ProjectName: "test-project",
		All:         true,
	})

	if err != nil {
		t.Fatalf("expected nil error for full down, got: %v", err)
	}
	if !downCalled {
		t.Error("expected DockerRunner.DownWithContext to be called")
	}
	if !removeCalled {
		t.Error("expected StateManager.RemoveProject to be called")
	}

	// Verify state file was removed
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("expected state file to be removed")
	}
}

func TestDownUseCase_Execute_ProjectDown(t *testing.T) {
	tmpDir := t.TempDir()
	deps, m := newTestDownDeps(tmpDir)

	stoppedServices := make(map[string]bool)
	m.dockerRunner.StopServiceWithContextFunc = func(ctx context.Context, cp string, svcName string) error {
		stoppedServices[svcName] = true
		return nil
	}

	// Track whether full DownWithContext is called
	fullDownCalled := false
	m.dockerRunner.DownWithContextFunc = func(ctx context.Context, cp string) error {
		fullDownCalled = true
		return nil
	}

	uc := NewDownUseCase(deps)
	err := uc.Execute(context.Background(), DownOptions{
		ProjectName: "test-project",
		All:         false, // project-only (default)
	})

	if err != nil {
		t.Fatalf("expected nil error for project down, got: %v", err)
	}
	if fullDownCalled {
		t.Error("expected DockerRunner.DownWithContext NOT to be called in project-only mode")
	}
	if !stoppedServices["api"] {
		t.Error("expected service 'api' to be stopped via StopServiceWithContext")
	}
}
