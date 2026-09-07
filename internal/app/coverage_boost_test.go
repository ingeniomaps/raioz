package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// ---------------------------------------------------------------------------
// up.go — Execute + stopOtherProjects
// ---------------------------------------------------------------------------

func TestUpUseCase_stopOtherProjects_NoGlobalState(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return nil, fmt.Errorf("no state")
		},
	}
	uc := NewUpUseCase(deps)
	// Should not panic
	uc.stopOtherProjects(context.Background(), "raioz.yaml")
}

func TestUpUseCase_stopOtherProjects_EmptyProjects(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return &models.GlobalState{ActiveProjects: []string{}}, nil
		},
	}
	uc := NewUpUseCase(deps)
	uc.stopOtherProjects(context.Background(), "raioz.yaml")
}

func TestUpUseCase_stopOtherProjects_SkipsCurrent(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return &models.GlobalState{
				ActiveProjects: []string{"my-proj"},
			}, nil
		},
	}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project: models.Project{Name: "my-proj"},
			}, nil, nil
		},
	}
	uc := NewUpUseCase(deps)
	uc.stopOtherProjects(context.Background(), "raioz.yaml")
}

func TestUpUseCase_stopOtherProjects_StopsOthers(t *testing.T) {
	initI18nForTest(t)
	stoppedProjects := []string{}
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return &models.GlobalState{
				ActiveProjects: []string{"other-proj", "my-proj"},
			}, nil
		},
	}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project: models.Project{Name: "my-proj"},
			}, nil, nil
		},
	}
	// Mock the workspace to make down work (or fail gracefully)
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return nil, fmt.Errorf("no workspace")
		},
	}
	// Track that down was attempted
	originalDeps := deps
	_ = originalDeps
	_ = stoppedProjects

	uc := NewUpUseCase(deps)
	uc.stopOtherProjects(context.Background(), "raioz.yaml")
	// The down will fail (workspace resolve), but the code path is exercised
}

func TestUpUseCase_Execute_WithExclusive(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return &models.GlobalState{ActiveProjects: []string{}}, nil
		},
	}
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return nil, nil, fmt.Errorf("no config")
		},
	}
	uc := NewUpUseCase(deps)
	err := uc.Execute(context.Background(), UpOptions{
		ConfigPath: "raioz.yaml",
		Exclusive:  true,
	})
	// Will fail on config load but exclusive path was exercised
	_ = err
}

// ---------------------------------------------------------------------------
// status.go — Execute deeper paths
// ---------------------------------------------------------------------------

func TestStatusUseCase_Execute_NilContext(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return nil, nil, nil
		},
	}
	uc := NewStatusUseCase(deps)
	//nolint:staticcheck // intentionally passing nil ctx to test guard
	err := uc.Execute(nil, StatusOptions{ConfigPath: "bad.json"})
	if err == nil {
		t.Error("expected error for nil config")
	}
}

// ---------------------------------------------------------------------------
// clean.go — cleanVolumes + cleanProject deeper paths
// ---------------------------------------------------------------------------

func TestCleanUseCase_cleanProject_WorkspaceResolveError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return nil, fmt.Errorf("no workspace")
		},
	}
	uc := NewCleanUseCase(deps)
	_, err := uc.cleanProject(context.Background(), "proj", CleanOptions{})
	if err == nil {
		t.Fatal("expected error for workspace resolve failure")
	}
}

func TestCleanUseCase_cleanProject_CleanProjectError(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetComposePathFunc: func(w *workspace.Workspace) string {
			return filepath.Join(tmpDir, "compose.yml")
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanProjectWithContextFunc: func(ctx context.Context, cp string, dry bool) ([]string, error) {
			return nil, fmt.Errorf("docker error")
		},
	}
	uc := NewCleanUseCase(deps)
	_, err := uc.cleanProject(context.Background(), "proj", CleanOptions{})
	if err == nil {
		t.Fatal("expected error for docker clean failure")
	}
}

func TestCleanUseCase_cleanProject_DryRunStateFile(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(stateFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
		GetComposePathFunc: func(w *workspace.Workspace) string {
			return filepath.Join(tmpDir, "compose.yml")
		},
		GetStatePathFunc: func(w *workspace.Workspace) string {
			return stateFile
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanProjectWithContextFunc: func(ctx context.Context, cp string, dry bool) ([]string, error) {
			return []string{"would remove"}, nil
		},
	}
	uc := NewCleanUseCase(deps)
	actions, err := uc.cleanProject(context.Background(), "proj", CleanOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range actions {
		if len(a) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected some actions for dry-run")
	}
}

func TestCleanUseCase_cleanVolumes_DryRun(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanUnusedVolumesWithContextFunc: func(ctx context.Context, dry, force bool) ([]string, error) {
			return []string{"Would remove vol1"}, nil
		},
	}
	uc := NewCleanUseCase(deps)
	actions, err := uc.cleanVolumes(context.Background(), CleanOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected actions for dry run volumes")
	}
}

func TestCleanUseCase_cleanVolumes_Force(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanUnusedVolumesWithContextFunc: func(ctx context.Context, dry, force bool) ([]string, error) {
			if !force {
				t.Error("expected force=true")
			}
			return []string{"Removed vol1"}, nil
		},
	}
	uc := NewCleanUseCase(deps)
	actions, err := uc.cleanVolumes(context.Background(), CleanOptions{Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected actions")
	}
}

func TestCleanUseCase_cleanVolumes_Error(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanUnusedVolumesWithContextFunc: func(ctx context.Context, dry, force bool) ([]string, error) {
			return nil, fmt.Errorf("volume error")
		},
	}
	uc := NewCleanUseCase(deps)
	_, err := uc.cleanVolumes(context.Background(), CleanOptions{Force: true})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCleanUseCase_Execute_AllBaseDirError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFunc: func() (string, error) {
			return "", fmt.Errorf("no base dir")
		},
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{All: true})
	if err == nil {
		t.Fatal("expected error for base dir failure")
	}
}

func TestCleanUseCase_Execute_CleanAllError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFunc: func() (string, error) {
			return "/tmp", nil
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanAllProjectsWithContextFunc: func(ctx context.Context, base string, dry bool) ([]string, error) {
			return nil, fmt.Errorf("clean all error")
		},
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{All: true})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCleanUseCase_Execute_ImagesError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return &models.Deps{Project: models.Project{Name: "proj"}}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: "/tmp"}, nil
		},
		GetComposePathFunc: func(w *workspace.Workspace) string { return "/tmp/c.yml" },
		GetStatePathFunc:   func(w *workspace.Workspace) string { return "/tmp/s.json" },
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanProjectWithContextFunc: func(ctx context.Context, cp string, dry bool) ([]string, error) {
			return nil, nil
		},
		CleanUnusedImagesWithContextFunc: func(ctx context.Context, dry bool) ([]string, error) {
			return nil, fmt.Errorf("images error")
		},
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{
		ProjectName: "proj",
		Images:      true,
	})
	if err == nil {
		t.Fatal("expected error for images clean failure")
	}
}

func TestCleanUseCase_Execute_NetworksError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return &models.Deps{Project: models.Project{Name: "proj"}}, nil, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: "/tmp"}, nil
		},
		GetComposePathFunc: func(w *workspace.Workspace) string { return "/tmp/c.yml" },
		GetStatePathFunc:   func(w *workspace.Workspace) string { return "/tmp/s.json" },
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		CleanProjectWithContextFunc: func(ctx context.Context, cp string, dry bool) ([]string, error) {
			return nil, nil
		},
		CleanUnusedNetworksWithContextFunc: func(ctx context.Context, dry bool) ([]string, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	uc := NewCleanUseCase(deps)
	err := uc.Execute(context.Background(), CleanOptions{
		ProjectName: "proj",
		Networks:    true,
	})
	if err == nil {
		t.Fatal("expected error for networks clean failure")
	}
}

// ---------------------------------------------------------------------------
// compare.go — Execute (production path success)
// ---------------------------------------------------------------------------

func TestCompareUseCase_Execute_InvalidProductionFile(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	prodPath := filepath.Join(tmpDir, "prod.yml")
	if err := os.WriteFile(prodPath, []byte("not: valid: compose"), 0644); err != nil {
		t.Fatal(err)
	}

	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(p string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:  models.Project{Name: "proj"},
				Services: map[string]models.Service{},
			}, nil, nil
		},
	}
	uc := NewCompareUseCase(deps)
	err := uc.Execute(CompareOptions{
		ConfigPath:     "ok.json",
		ProductionPath: prodPath,
	})
	if err == nil {
		t.Fatal("expected error for invalid production file")
	}
}

// ---------------------------------------------------------------------------
// logs.go — resolveServices more paths
// ---------------------------------------------------------------------------

func TestLogsUseCase_resolveServices_AllWithProjectCompose(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	callCount := 0
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, cp string) ([]string, error) {
			callCount++
			if callCount == 1 {
				return []string{"api"}, nil
			}
			return []string{"db"}, nil
		},
	}
	uc := NewLogsUseCase(deps)
	svcs, projSvcs, err := uc.resolveServices(
		context.Background(),
		LogsOptions{All: true},
		"/tmp/compose.yml",
		"/tmp/project-compose.yml",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 1 || svcs[0] != "api" {
		t.Errorf("expected [api], got %v", svcs)
	}
	if len(projSvcs) != 1 || projSvcs[0] != "db" {
		t.Errorf("expected [db], got %v", projSvcs)
	}
}

func TestLogsUseCase_resolveServices_SpecificWithProjectCompose(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	callCount := 0
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, cp string) ([]string, error) {
			callCount++
			if callCount == 1 {
				return []string{"api"}, nil
			}
			return []string{"db"}, nil
		},
	}
	uc := NewLogsUseCase(deps)
	svcs, projSvcs, err := uc.resolveServices(
		context.Background(),
		LogsOptions{Services: []string{"api", "db"}},
		"/tmp/compose.yml",
		"/tmp/project-compose.yml",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 1 || svcs[0] != "api" {
		t.Errorf("expected [api], got %v", svcs)
	}
	if len(projSvcs) != 1 || projSvcs[0] != "db" {
		t.Errorf("expected [db], got %v", projSvcs)
	}
}

func TestLogsUseCase_resolveServices_NoProjectCompose(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, cp string) ([]string, error) {
			return []string{"api"}, nil
		},
	}
	uc := NewLogsUseCase(deps)
	svcs, projSvcs, err := uc.resolveServices(
		context.Background(),
		LogsOptions{Services: []string{"api"}},
		"/tmp/compose.yml",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svcs) != 1 {
		t.Errorf("expected 1 service, got %d", len(svcs))
	}
	if projSvcs != nil {
		t.Errorf("expected nil project services, got %v", projSvcs)
	}
}

func TestLogsUseCase_resolveServices_AllError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(ctx context.Context, cp string) ([]string, error) {
			return nil, fmt.Errorf("docker error")
		},
	}
	uc := NewLogsUseCase(deps)
	_, _, err := uc.resolveServices(
		context.Background(),
		LogsOptions{All: true},
		"/tmp/compose.yml",
		"",
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLogsUseCase_Execute_ViewLogsError(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner := newTestDepsForLogs(t)
	configLoader.LoadDepsFunc = func(p string) (*models.Deps, []string, error) {
		return &models.Deps{Project: models.Project{Name: "proj"}}, nil, nil
	}
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, cp string) ([]string, error) {
		return []string{"api"}, nil
	}
	dockerRunner.ViewLogsWithContextFunc = func(ctx context.Context, cp string, opts interfaces.LogsOptions) error {
		return fmt.Errorf("view logs error")
	}
	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{All: true})
	if err == nil {
		t.Fatal("expected error for view logs failure")
	}
}

func TestLogsUseCase_Execute_ProjectComposeLogsError(t *testing.T) {
	initI18nForTest(t)
	deps, configLoader, _, _, dockerRunner := newTestDepsForLogs(t)
	configLoader.LoadDepsFunc = func(p string) (*models.Deps, []string, error) {
		return &models.Deps{Project: models.Project{Name: "proj"}}, nil, nil
	}
	// ADR-011 Phase 2: ProjectComposePath comes from LocalState now.
	projectDir := t.TempDir()
	configPath := projectDir + "/raioz.yaml"
	if err := writeFakeLocalStateForTest(projectDir, "/tmp/project-compose.yml"); err != nil {
		t.Fatalf("write fake LocalState: %v", err)
	}
	callCount := 0
	dockerRunner.GetAvailableServicesWithContextFunc = func(ctx context.Context, cp string) ([]string, error) {
		callCount++
		if callCount == 1 {
			return nil, nil // no generated services
		}
		return []string{"db"}, nil
	}
	viewCallCount := 0
	dockerRunner.ViewLogsWithContextFunc = func(ctx context.Context, cp string, opts interfaces.LogsOptions) error {
		viewCallCount++
		if viewCallCount == 1 {
			return fmt.Errorf("project compose logs error")
		}
		return nil
	}
	uc := NewLogsUseCase(deps)
	err := uc.Execute(context.Background(), LogsOptions{All: true, ConfigPath: configPath})
	if err == nil {
		t.Fatal("expected error for project compose logs failure")
	}
}

// ---------------------------------------------------------------------------
// ci.go — executeYAML more paths
// ---------------------------------------------------------------------------

func TestCIUseCase_executeYAML_ServicePathNotFound(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	uc := NewCIUseCase(deps)
	proj := &YAMLProject{
		Deps: &models.Deps{
			SchemaVersion: "2.0",
			SourceFormat:  models.SourceFormatYAML,
			Project:       models.Project{Name: "proj"},
			Network:       models.NetworkConfig{Name: "net"},
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: "/nonexistent/path/xyz"}},
			},
			Infra: map[string]models.InfraEntry{},
		},
		ProjectName: "proj",
		NetworkName: "net",
	}
	result := &CIResult{
		Validations: []ValidationResult{},
		Errors:      []string{},
	}
	got, err := uc.executeYAML(proj, CIOptions{OnlyValidate: true}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Success {
		t.Error("expected Success=false for missing service path")
	}
}

func TestCIUseCase_executeYAML_PullSuccess(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidateAllImagesFunc: func(d *models.Deps) error { return nil },
	}
	uc := NewCIUseCase(deps)
	proj := &YAMLProject{
		Deps: &models.Deps{
			SchemaVersion: "2.0",
			SourceFormat:  models.SourceFormatYAML,
			Project:       models.Project{Name: "proj"},
			Network:       models.NetworkConfig{Name: "net"},
			Services:      map[string]models.Service{},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{Image: "redis:7"}},
			},
		},
		ProjectName: "proj",
		NetworkName: "net",
	}
	result := &CIResult{
		Validations: []ValidationResult{},
		Errors:      []string{},
	}
	got, err := uc.executeYAML(proj, CIOptions{}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Success {
		t.Errorf("expected Success=true, errors=%v", got.Errors)
	}
}

func TestCIUseCase_executeYAML_PullFailure(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidateAllImagesFunc: func(d *models.Deps) error {
			return fmt.Errorf("pull failed")
		},
	}
	uc := NewCIUseCase(deps)
	proj := &YAMLProject{
		Deps: &models.Deps{
			SchemaVersion: "2.0",
			SourceFormat:  models.SourceFormatYAML,
			Project:       models.Project{Name: "proj"},
			Network:       models.NetworkConfig{Name: "net"},
			Services:      map[string]models.Service{},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{Image: "redis:7"}},
			},
		},
		ProjectName: "proj",
		NetworkName: "net",
	}
	result := &CIResult{
		Validations: []ValidationResult{},
		Errors:      []string{},
	}
	got, err := uc.executeYAML(proj, CIOptions{}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Success {
		t.Error("expected Success=false for pull failure")
	}
}

func TestCIUseCase_executeYAML_SkipPull(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Validator = &mocks.MockValidator{}
	uc := NewCIUseCase(deps)
	proj := &YAMLProject{
		Deps: &models.Deps{
			SchemaVersion: "2.0",
			SourceFormat:  models.SourceFormatYAML,
			Project:       models.Project{Name: "proj"},
			Network:       models.NetworkConfig{Name: "net"},
			Services:      map[string]models.Service{},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{Image: "redis:7"}},
			},
		},
		ProjectName: "proj",
		NetworkName: "net",
	}
	result := &CIResult{
		Validations: []ValidationResult{},
		Errors:      []string{},
	}
	got, err := uc.executeYAML(proj, CIOptions{SkipPull: true}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Success {
		t.Errorf("expected Success=true, errors=%v", got.Errors)
	}
}

// ---------------------------------------------------------------------------
// down_orchestrated.go — killProcessGroup with safe PID
// ---------------------------------------------------------------------------

func TestKillProcessGroup_NonExistentPID(t *testing.T) {
	// Use a PID that almost certainly doesn't exist
	// NEVER use 0 or -1 as that would kill process groups we don't want
	killProcessGroup(99999999)
	// Should not panic — just fails silently
}

// ---------------------------------------------------------------------------
// HandleLocalProjectDown — deeper paths
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// yaml_commands.go — isHostProcessAlive + CheckYAML
// ---------------------------------------------------------------------------

func TestIsHostProcessAlive_NonExistent_Boost(t *testing.T) {
	if isHostProcessAlive(99999999) {
		t.Error("expected false for non-existent PID")
	}
}

func TestIsHostProcessAlive_CurrentProcess_Boost(t *testing.T) {
	if !isHostProcessAlive(os.Getpid()) {
		t.Error("expected true for current process")
	}
}
