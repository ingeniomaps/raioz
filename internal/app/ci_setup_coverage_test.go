package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

func TestCIUseCase_validateCIPorts_NilConflicts(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/tmp" },
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidatePortsFunc: func(d *config.Deps, baseDir string, projectName string) ([]interfaces.PortConflict, error) {
			return nil, nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	ws := &workspace.Workspace{Root: "/tmp"}
	cfgDeps := &config.Deps{Project: config.Project{Name: "test"}}

	if err := uc.validateCIPorts(cfgDeps, ws, "test", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, v := range result.Validations {
		if v.Check == "ports" && v.Status == "passed" {
			found = true
		}
	}
	if !found {
		t.Error("expected passed port validation")
	}
}

func TestCIUseCase_validateCIPorts_Error(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/tmp" },
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidatePortsFunc: func(d *config.Deps, baseDir string, projectName string) ([]interfaces.PortConflict, error) {
			return nil, fmt.Errorf("port error")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	ws := &workspace.Workspace{Root: "/tmp"}
	cfgDeps := &config.Deps{Project: config.Project{Name: "test"}}

	if err := uc.validateCIPorts(cfgDeps, ws, "test", result); err == nil {
		t.Fatal("expected error")
	}
	if len(result.Errors) == 0 {
		t.Error("expected error recorded")
	}
}

func TestCIUseCase_validateCIPorts_WithConflicts(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/tmp" },
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidatePortsFunc: func(d *config.Deps, baseDir string, projectName string) ([]interfaces.PortConflict, error) {
			return []interfaces.PortConflict{
				{Port: "8080", Project: "other", Service: "api"},
			}, nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	ws := &workspace.Workspace{Root: "/tmp"}
	cfgDeps := &config.Deps{Project: config.Project{Name: "test"}}

	if err := uc.validateCIPorts(cfgDeps, ws, "test", result); err == nil {
		t.Fatal("expected error for port conflicts")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors recorded")
	}
}

func TestCIUseCase_validateCIImages_NoPull(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{Project: config.Project{Name: "test"}}

	if err := uc.validateCIImages(cfgDeps, CIOptions{SkipPull: true}, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, v := range result.Validations {
		if v.Check == "images" && v.Status == "skipped" {
			found = true
		}
	}
	if !found {
		t.Error("expected skipped images validation")
	}
}

func TestCIUseCase_validateCIImages_Pass(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidateAllImagesFunc: func(d *config.Deps) error { return nil },
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{Project: config.Project{Name: "test"}}

	if err := uc.validateCIImages(cfgDeps, CIOptions{}, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCIUseCase_validateCIImages_Error(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		ValidateAllImagesFunc: func(d *config.Deps) error { return fmt.Errorf("pull fail") },
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{Project: config.Project{Name: "test"}}

	if err := uc.validateCIImages(cfgDeps, CIOptions{}, result); err == nil {
		t.Fatal("expected error")
	}
}

func TestCIUseCase_ensureCINetwork_Pass(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name string, subnet string, ask bool) error {
			return nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Network: config.NetworkConfig{Name: "testnet"},
	}

	if err := uc.ensureCINetwork(cfgDeps, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCIUseCase_ensureCINetwork_Error(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		EnsureNetworkWithConfigAndContextFunc: func(ctx context.Context, name string, subnet string, ask bool) error {
			return fmt.Errorf("network fail")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Network: config.NetworkConfig{Name: "testnet"},
	}

	if err := uc.ensureCINetwork(cfgDeps, result); err == nil {
		t.Fatal("expected error")
	}
}

func TestCIUseCase_ensureCIVolumes_NoVolumes(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		ExtractNamedVolumesFunc: func(volumes []string) ([]string, error) {
			return nil, nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	if err := uc.ensureCIVolumes(cfgDeps, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCIUseCase_ensureCIVolumes_WithVolumes(t *testing.T) {
	initI18nForTest(t)
	ensuredVolumes := []string{}
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		ExtractNamedVolumesFunc: func(volumes []string) ([]string, error) {
			return []string{"pgdata", "redis-data"}, nil
		},
		EnsureVolumeWithContextFunc: func(ctx context.Context, name string) error {
			ensuredVolumes = append(ensuredVolumes, name)
			return nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Docker: &config.DockerConfig{Volumes: []string{"pgdata:/data"}}},
		},
		Infra: map[string]config.InfraEntry{
			"redis": {Inline: &config.Infra{Volumes: []string{"redis-data:/data"}}},
		},
	}

	if err := uc.ensureCIVolumes(cfgDeps, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ensuredVolumes) != 2 {
		t.Errorf("expected 2 ensured volumes, got %d", len(ensuredVolumes))
	}
}

func TestCIUseCase_ensureCIVolumes_ExtractError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		ExtractNamedVolumesFunc: func(volumes []string) ([]string, error) {
			return nil, fmt.Errorf("extract fail")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Docker: &config.DockerConfig{Volumes: []string{"vol:/data"}}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	if err := uc.ensureCIVolumes(cfgDeps, result); err == nil {
		t.Fatal("expected error")
	}
}

func TestCIUseCase_ensureCIVolumes_EnsureError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.DockerRunner = &mocks.MockDockerRunner{
		ExtractNamedVolumesFunc: func(volumes []string) ([]string, error) {
			return []string{"vol1"}, nil
		},
		EnsureVolumeWithContextFunc: func(ctx context.Context, name string) error {
			return fmt.Errorf("ensure fail")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	if err := uc.ensureCIVolumes(cfgDeps, result); err == nil {
		t.Fatal("expected error")
	}
}

func TestCIUseCase_resolveCIGit_SkipBuild(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{Services: map[string]config.Service{}}
	ws := &workspace.Workspace{Root: "/tmp", ServicesDir: "/tmp/services"}

	if err := uc.resolveCIGit(cfgDeps, ws, CIOptions{SkipBuild: true}, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, v := range result.Validations {
		if v.Check == "git" && v.Status == "skipped" {
			found = true
		}
	}
	if !found {
		t.Error("expected skipped git validation")
	}
}

func TestCIUseCase_resolveCIGit_WithGitService(t *testing.T) {
	initI18nForTest(t)
	var ensureCalled bool
	deps := newFullMockDeps()
	deps.GitRepository = &mocks.MockGitRepository{
		EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
			ensureCalled = true
			return nil
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "git", Repo: "https://example.com/repo.git"}},
		},
	}
	ws := &workspace.Workspace{Root: "/tmp", ServicesDir: "/tmp/services"}

	if err := uc.resolveCIGit(cfgDeps, ws, CIOptions{}, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ensureCalled {
		t.Error("expected EnsureRepoWithForce to be called")
	}
}

func TestCIUseCase_resolveCIGit_Error(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.GitRepository = &mocks.MockGitRepository{
		EnsureRepoWithForceFunc: func(src config.SourceConfig, baseDir string, force bool) error {
			return fmt.Errorf("git fail")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "git"}},
		},
	}
	ws := &workspace.Workspace{Root: "/tmp", ServicesDir: "/tmp/services"}

	if err := uc.resolveCIGit(cfgDeps, ws, CIOptions{}, result); err == nil {
		t.Fatal("expected error")
	}
}

func TestCIUseCase_cleanupEphemeral(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	var downCalled bool
	deps := newFullMockDeps()
	deps.Workspace = &mocks.MockWorkspaceManager{
		GetComposePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/compose.yml"
		},
		GetStatePathFunc: func(ws *workspace.Workspace) string {
			return tmpDir + "/state.json"
		},
	}
	deps.DockerRunner = &mocks.MockDockerRunner{
		DownFunc: func(composePath string) error {
			downCalled = true
			return nil
		},
	}
	uc := NewCIUseCase(deps)
	ws := &workspace.Workspace{Root: tmpDir}

	// compose file doesn't exist, so Down shouldn't be called
	uc.cleanupEphemeral(ws, "test")
	if downCalled {
		t.Error("expected Down not to be called when compose file does not exist")
	}
}

func TestCIUseCase_executeSetup_WorkspaceResolveError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		FilterByFeatureFlagsFunc: func(d *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
			return d, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return nil, fmt.Errorf("workspace fail")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	err := uc.executeSetup(cfgDeps, CIOptions{}, result)
	if err == nil {
		t.Fatal("expected error for workspace resolve failure")
	}
}

func TestCIUseCase_executeSetup_PermissionsError(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		FilterByFeatureFlagsFunc: func(d *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
			return d, nil
		},
	}
	deps.Workspace = &mocks.MockWorkspaceManager{
		ResolveFunc: func(name string) (*workspace.Workspace, error) {
			return &workspace.Workspace{Root: tmpDir}, nil
		},
	}
	deps.Validator = &mocks.MockValidator{
		CheckWorkspacePermissionsFunc: func(path string) error {
			return fmt.Errorf("no permission")
		},
	}
	uc := NewCIUseCase(deps)
	result := &CIResult{Validations: []ValidationResult{}, Errors: []string{}}
	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	err := uc.executeSetup(cfgDeps, CIOptions{}, result)
	if err == nil {
		t.Fatal("expected error for permission failure")
	}
}
