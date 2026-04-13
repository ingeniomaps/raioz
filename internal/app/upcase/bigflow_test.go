package upcase

import (
	"context"
	"errors"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// --- validate -----------------------------------------------------------------

func newValidateUC(
	preflightErr, allErr, permErr error,
	detectMissingFunc func(
		*config.Deps, func(string, config.Service) string,
	) ([]config.MissingDependency, error),
	detectConflictsFunc func(
		*config.Deps, func(string, config.Service) string,
	) ([]config.DependencyConflict, error),
) *UseCase {
	return NewUseCase(&Dependencies{
		Validator: &mocks.MockValidator{
			PreflightCheckWithContextFunc: func(ctx context.Context) error { return preflightErr },
			AllFunc:                       func(deps *config.Deps) error { return allErr },
			CheckWorkspacePermissionsFunc: func(p string) error { return permErr },
		},
		Workspace: &mocks.MockWorkspaceManager{
			MigrateLegacyServicesFunc: func(ws *workspace.Workspace, deps *config.Deps) error {
				return nil
			},
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			BuildServiceVolumesMapFunc: func(
				deps *config.Deps,
			) (map[string]interfaces.ServiceVolumes, error) {
				return nil, nil
			},
			DetectSharedVolumesFunc: func(
				m map[string]interfaces.ServiceVolumes,
			) map[string][]string {
				return nil
			},
			FormatSharedVolumesWarningFunc: func(shared map[string][]string) string {
				return ""
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: detectMissingFunc,
			DetectDependencyConflictsFunc: detectConflictsFunc,
		},
	})
}

func TestValidatePreflightFails(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(errors.New("docker off"), nil, nil, nil, nil)
	err := uc.validate(
		context.Background(), &config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected preflight error to propagate")
	}
}

func TestValidateAllFails(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(nil, errors.New("bad config"), nil, nil, nil)
	err := uc.validate(
		context.Background(), &config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected validate.All error to propagate")
	}
}

func TestValidateSuccess(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(nil, nil, nil,
		func(*config.Deps, func(string, config.Service) string) ([]config.MissingDependency, error) {
			return nil, nil
		},
		func(*config.Deps, func(string, config.Service) string) ([]config.DependencyConflict, error) {
			return nil, nil
		},
	)
	err := uc.validate(
		context.Background(), &config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestValidateWorkspacePermissionFails(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(nil, nil, errors.New("no perm"),
		func(*config.Deps, func(string, config.Service) string) ([]config.MissingDependency, error) {
			return nil, nil
		},
		func(*config.Deps, func(string, config.Service) string) ([]config.DependencyConflict, error) {
			return nil, nil
		},
	)
	err := uc.validate(
		context.Background(), &config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected permissions error to propagate")
	}
}

// --- handleDependencyConflicts ------------------------------------------------

func TestHandleDependencyConflictsNone(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.DependencyConflict, error) {
				return nil, nil
			},
		},
	})
	shouldContinue, _, err := uc.handleDependencyConflicts(
		&config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldContinue {
		t.Error("should continue when no conflicts")
	}
}

func TestHandleDependencyConflictsDetectError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.DependencyConflict, error) {
				return nil, errors.New("boom")
			},
		},
	})
	_, _, err := uc.handleDependencyConflicts(
		&config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected error")
	}
}

func TestHandleDependencyConflictsDryRun(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.DependencyConflict, error) {
				return []config.DependencyConflict{
					{ServiceName: "svc", Differences: []string{"branch differs"}},
				}, nil
			},
		},
	})
	shouldContinue, _, err := uc.handleDependencyConflicts(
		&config.Deps{}, &workspace.Workspace{Root: "/tmp"}, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if shouldContinue {
		t.Error("dry-run should abort on conflicts")
	}
}

// --- handleDependencyAssist ---------------------------------------------------

func TestHandleDependencyAssistNone(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.MissingDependency, error) {
				return nil, nil
			},
		},
	})
	ok, added, err := uc.handleDependencyAssist(
		&config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should continue when no missing deps")
	}
	if added == nil {
		t.Error("added should be non-nil empty slice")
	}
}

func TestHandleDependencyAssistDryRun(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.MissingDependency, error) {
				return []config.MissingDependency{
					{ServiceName: "cache", RequiredBy: "api"},
				}, nil
			},
		},
	})
	ok, _, err := uc.handleDependencyAssist(
		&config.Deps{}, &workspace.Workspace{Root: "/tmp"}, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("dry-run should abort on missing deps")
	}
}

func TestHandleDependencyAssistDetectError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc config.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.MissingDependency, error) {
				return nil, errors.New("boom")
			},
		},
	})
	_, _, err := uc.handleDependencyAssist(
		&config.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected error")
	}
}

// --- prepareDockerResources ---------------------------------------------------

func TestPrepareDockerResourcesSuccess(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string {
				return "/tmp/base"
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(
				d *config.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(
				ctx context.Context, name, subnet string, ask bool,
			) error {
				return nil
			},
			ExtractNamedVolumesFunc: func(v []string) ([]string, error) {
				return nil, nil
			},
			NormalizeVolumeNameFunc: func(p, n string) (string, error) {
				return p + "-" + n, nil
			},
			EnsureVolumeWithContextFunc: func(ctx context.Context, name string) error {
				return nil
			},
		},
	})
	deps := &config.Deps{
		SchemaVersion: "2.0",
		Project:       config.Project{Name: "p"},
		Network:       config.NetworkConfig{Name: "test-net"},
	}
	err := uc.prepareDockerResources(
		context.Background(), deps, &workspace.Workspace{Root: "/tmp"},
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrepareDockerResourcesPortConflicts(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/b" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(
				d *config.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return []interfaces.PortConflict{{Port: "3000", Service: "api"}}, nil
			},
			FormatPortConflictsFunc: func([]interfaces.PortConflict) string { return "conflict" },
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"},
	)
	if err == nil {
		t.Error("expected error on port conflicts")
	}
}

func TestPrepareDockerResourcesValidatePortsError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/b" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(
				d *config.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, errors.New("validation error")
			},
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"},
	)
	if err == nil {
		t.Error("expected error on validate ports failure")
	}
}

func TestPrepareDockerResourcesImagePullFails(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/b" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(
				d *config.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return errors.New("pull failed") },
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"},
	)
	if err == nil {
		t.Error("expected image pull error")
	}
}

func TestPrepareDockerResourcesNetworkFails(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/b" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			ValidatePortsFunc: func(
				d *config.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*config.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(
				ctx context.Context, name, subnet string, ask bool,
			) error {
				return errors.New("network failed")
			},
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"},
	)
	if err == nil {
		t.Error("expected network error")
	}
}

// --- processCompose -----------------------------------------------------------

func TestProcessComposeEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	composePath, svcs, infra, err := uc.processCompose(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if composePath != "" || len(svcs) != 0 || len(infra) != 0 {
		t.Errorf("expected empty results, got path=%q svcs=%v infra=%v", composePath, svcs, infra)
	}
}

func TestProcessComposeGenerateError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GenerateComposeFunc: func(
				deps *config.Deps, ws *interfaces.Workspace, projectDir string,
			) (string, []string, error) {
				return "", nil, errors.New("generate err")
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image", Image: "nginx"}},
		},
	}
	_, _, _, err := uc.processCompose(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err == nil {
		t.Error("expected generate compose error")
	}
}

func TestProcessComposeSkipsDisabled(t *testing.T) {
	initI18nUp(t)
	disabled := false
	genCalled := false
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GenerateComposeFunc: func(
				deps *config.Deps, ws *interfaces.Workspace, projectDir string,
			) (string, []string, error) {
				genCalled = true
				return "/path/compose.yml", nil, nil
			},
			UpServicesWithContextFunc: func(
				ctx context.Context, path string, names []string,
			) error {
				return nil
			},
			WaitForServicesHealthyFunc: func(
				ctx context.Context, path string,
				svcs []string, infra []string, proj string,
			) error {
				return nil
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"enabled":  {Source: config.SourceConfig{Kind: "image"}},
			"disabled": {Source: config.SourceConfig{Kind: "image"}, Enabled: &disabled},
		},
	}
	_, svcs, _, err := uc.processCompose(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !genCalled {
		t.Error("generate should be called")
	}
	// Only "enabled" should be in svcs
	if len(svcs) != 1 || svcs[0] != "enabled" {
		t.Errorf("svcs = %v, want [enabled]", svcs)
	}
}

func TestProcessComposeInfraStartError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GenerateComposeFunc: func(
				deps *config.Deps, ws *interfaces.Workspace, projectDir string,
			) (string, []string, error) {
				return "/path/compose.yml", nil, nil
			},
			UpServicesWithContextFunc: func(
				ctx context.Context, path string, names []string,
			) error {
				return errors.New("infra up failed")
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres"}},
		},
	}
	_, _, _, err := uc.processCompose(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err == nil {
		t.Error("expected infra start error")
	}
}

// --- checkDependencyProjects --------------------------------------------------

func TestCheckDependencyProjectsNoState(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadGlobalStateFunc: func() (*state.GlobalState, error) {
				return nil, errors.New("no state")
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	// Should not error when global state can't be loaded
	err := uc.checkDependencyProjects(context.Background(), deps)
	if err != nil {
		t.Errorf("should be tolerant to state load error: %v", err)
	}
}

func TestCheckDependencyProjectsNoMatch(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadGlobalStateFunc: func() (*state.GlobalState, error) {
				return &state.GlobalState{
					Projects: map[string]state.ProjectState{
						"other": {Name: "other"},
					},
				}, nil
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {DependsOn: []string{"db"}},
		},
	}
	err := uc.checkDependencyProjects(context.Background(), deps)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- mergeDeps ----------------------------------------------------------------

func TestMergeDeps(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			ResolveRelativeVolumesFunc: func(
				volumes []string, projectDir string,
			) ([]string, error) {
				return volumes, nil
			},
		},
	})

	oldDeps := &config.Deps{
		Project:     config.Project{Name: "old"},
		ProjectRoot: "/old",
		Env: config.EnvConfig{
			Files:     []string{"old.env"},
			Variables: map[string]string{"K1": "old"},
		},
		Services: map[string]config.Service{
			"shared": {Source: config.SourceConfig{Kind: "git"}},
			"oldsvc": {Source: config.SourceConfig{Kind: "git"}},
		},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres"}},
		},
	}

	newDeps := &config.Deps{
		Project: config.Project{Name: "new"},
		Env: config.EnvConfig{
			Files:     []string{"new.env"},
			Variables: map[string]string{"K1": "new", "K2": "val"},
		},
		Services: map[string]config.Service{
			"shared": {Source: config.SourceConfig{Kind: "image"}},
			"newsvc": {Source: config.SourceConfig{Kind: "git"}},
		},
		Infra: map[string]config.InfraEntry{
			"redis": {Inline: &config.Infra{Image: "redis"}},
		},
	}

	merged := uc.mergeDeps(oldDeps, newDeps, "/new")
	if merged.Project.Name != "new" {
		t.Errorf("merged project = %q, want new", merged.Project.Name)
	}
	// union of services
	if _, ok := merged.Services["shared"]; !ok {
		t.Error("missing shared")
	}
	if _, ok := merged.Services["oldsvc"]; !ok {
		t.Error("missing oldsvc")
	}
	if _, ok := merged.Services["newsvc"]; !ok {
		t.Error("missing newsvc")
	}
	// new service overwrites shared
	if merged.Services["shared"].Source.Kind != "image" {
		t.Errorf("shared kind = %q, want image (new)", merged.Services["shared"].Source.Kind)
	}
	// union of infra
	if _, ok := merged.Infra["postgres"]; !ok {
		t.Error("missing postgres")
	}
	if _, ok := merged.Infra["redis"]; !ok {
		t.Error("missing redis")
	}
	// env merge: new overrides old, both files present
	if merged.Env.Variables["K1"] != "new" {
		t.Errorf("K1 = %q, want new", merged.Env.Variables["K1"])
	}
	if merged.Env.Variables["K2"] != "val" {
		t.Errorf("K2 = %q, want val", merged.Env.Variables["K2"])
	}
	if len(merged.Env.Files) != 2 {
		t.Errorf("Files len = %d, want 2", len(merged.Env.Files))
	}
}

func TestMergeDepsNoOldProjectRoot(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			ResolveRelativeVolumesFunc: func(
				volumes []string, projectDir string,
			) ([]string, error) {
				return volumes, nil
			},
		},
	})
	oldDeps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	newDeps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	merged := uc.mergeDeps(oldDeps, newDeps, "/current")
	if merged.ProjectRoot != "/current" {
		t.Errorf("ProjectRoot = %q", merged.ProjectRoot)
	}
}

// --- checkWorkspaceProjectConflict --------------------------------------------

func TestCheckWorkspaceProjectConflictNoState(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return nil, errors.New("load err")
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	result, merged, err := uc.checkWorkspaceProjectConflict(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("expected Proceed, got %v", result)
	}
	if merged != nil {
		t.Error("expected nil merged when no conflict")
	}
}

func TestCheckWorkspaceProjectConflictNilOldDeps(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return nil, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	result, _, err := uc.checkWorkspaceProjectConflict(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("expected Proceed, got %v", result)
	}
}

func TestCheckWorkspaceProjectConflictSameProject(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return &config.Deps{
					Project:  config.Project{Name: "p"},
					Services: map[string]config.Service{},
					Infra:    map[string]config.InfraEntry{},
				}, nil
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			ResolveRelativeVolumesFunc: func(
				volumes []string, projectDir string,
			) ([]string, error) {
				return volumes, nil
			},
		},
	})
	deps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	result, merged, err := uc.checkWorkspaceProjectConflict(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("expected Proceed, got %v", result)
	}
	if merged == nil {
		t.Error("expected merged deps for same project")
	}
}

func TestCheckWorkspaceProjectConflictDifferentNoOverlap(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return &config.Deps{
					Project: config.Project{Name: "other"},
					Services: map[string]config.Service{
						"oldonly": {},
					},
					Infra: map[string]config.InfraEntry{},
				}, nil
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "mine"},
		Services: map[string]config.Service{
			"newonly": {},
		},
		Infra: map[string]config.InfraEntry{},
	}
	result, _, err := uc.checkWorkspaceProjectConflict(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("expected Proceed, got %v", result)
	}
}

// --- showSummary --------------------------------------------------------------

func TestShowSummary(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// showSummary just prints; ensure it doesn't panic.
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	uc.showSummary(
		context.Background(), deps,
		[]string{"a"}, []string{"b"},
		time.Now(),
	)
}
