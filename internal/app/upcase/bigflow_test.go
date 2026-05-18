package upcase

import (
	"context"
	"errors"
	"testing"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- validate -----------------------------------------------------------------

func newValidateUC(
	preflightErr, allErr, permErr error,
	detectMissingFunc func(
		*models.Deps, func(string, models.Service) string,
	) ([]models.MissingDependency, error),
	detectConflictsFunc func(
		*models.Deps, func(string, models.Service) string,
	) ([]models.DependencyConflict, error),
) *UseCase {
	return NewUseCase(&Dependencies{
		Validator: &mocks.MockValidator{
			PreflightCheckWithContextFunc: func(ctx context.Context) error { return preflightErr },
			AllFunc:                       func(deps *models.Deps) error { return allErr },
			CheckWorkspacePermissionsFunc: func(p string) error { return permErr },
		},
		Workspace: &mocks.MockWorkspaceManager{
			MigrateLegacyServicesFunc: func(ws *workspace.Workspace, deps *models.Deps) error {
				return nil
			},
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			BuildServiceVolumesMapFunc: func(
				deps *models.Deps,
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
		context.Background(), &models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected preflight error to propagate")
	}
}

func TestValidateAllFails(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(nil, errors.New("bad config"), nil, nil, nil)
	err := uc.validate(
		context.Background(), &models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Error("expected validate.All error to propagate")
	}
}

func TestValidateSuccess(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(nil, nil, nil,
		func(*models.Deps, func(string, models.Service) string) ([]models.MissingDependency, error) {
			return nil, nil
		},
		func(*models.Deps, func(string, models.Service) string) ([]models.DependencyConflict, error) {
			return nil, nil
		},
	)
	err := uc.validate(
		context.Background(), &models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestValidateWorkspacePermissionFails(t *testing.T) {
	initI18nUp(t)
	uc := newValidateUC(nil, nil, errors.New("no perm"),
		func(*models.Deps, func(string, models.Service) string) ([]models.MissingDependency, error) {
			return nil, nil
		},
		func(*models.Deps, func(string, models.Service) string) ([]models.DependencyConflict, error) {
			return nil, nil
		},
	)
	err := uc.validate(
		context.Background(), &models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
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
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.DependencyConflict, error) {
				return nil, nil
			},
		},
	})
	shouldContinue, _, err := uc.handleDependencyConflicts(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
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
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.DependencyConflict, error) {
				return nil, errors.New("boom")
			},
		},
	})
	_, _, err := uc.handleDependencyConflicts(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
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
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.DependencyConflict, error) {
				return []models.DependencyConflict{
					{ServiceName: "svc", Differences: []string{"branch differs"}},
				}, nil
			},
		},
	})
	shouldContinue, _, err := uc.handleDependencyConflicts(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, true,
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
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.MissingDependency, error) {
				return nil, nil
			},
		},
	})
	ok, added, err := uc.handleDependencyAssist(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
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
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.MissingDependency, error) {
				return []models.MissingDependency{
					{ServiceName: "cache", RequiredBy: "api"},
				}, nil
			},
		},
	})
	ok, _, err := uc.handleDependencyAssist(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, true,
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
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.MissingDependency, error) {
				return nil, errors.New("boom")
			},
		},
	})
	_, _, err := uc.handleDependencyAssist(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
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
				d *models.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*models.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(
				ctx context.Context, name, subnet string, _ map[string]string, ask bool,
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
	deps := &models.Deps{
		SchemaVersion: "2.0",
		SourceFormat:  models.SourceFormatYAML,
		Project:       models.Project{Name: "p"},
		Network:       models.NetworkConfig{Name: "test-net"},
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
				d *models.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return []interfaces.PortConflict{{Port: "3000", Service: "api"}}, nil
			},
			FormatPortConflictsFunc: func([]interfaces.PortConflict) string { return "conflict" },
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&models.Deps{Project: models.Project{Name: "p"}},
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
				d *models.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, errors.New("validation error")
			},
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&models.Deps{Project: models.Project{Name: "p"}},
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
				d *models.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*models.Deps) error { return errors.New("pull failed") },
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&models.Deps{Project: models.Project{Name: "p"}},
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
				d *models.Deps, b string, p string,
			) ([]interfaces.PortConflict, error) {
				return nil, nil
			},
			ValidateAllImagesFunc: func(*models.Deps) error { return nil },
			EnsureNetworkWithConfigAndContextFunc: func(
				ctx context.Context, name, subnet string, _ map[string]string, ask bool,
			) error {
				return errors.New("network failed")
			},
		},
	})
	err := uc.prepareDockerResources(
		context.Background(),
		&models.Deps{Project: models.Project{Name: "p"}},
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
	deps := &models.Deps{Project: models.Project{Name: "p"}}
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
				deps *models.Deps, ws *interfaces.Workspace, projectDir string,
			) (string, []string, error) {
				return "", nil, errors.New("generate err")
			},
		},
	})
	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image", Image: "nginx"}},
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
				deps *models.Deps, ws *interfaces.Workspace, projectDir string,
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
	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"enabled":  {Source: models.SourceConfig{Kind: "image"}},
			"disabled": {Source: models.SourceConfig{Kind: "image"}, Enabled: &disabled},
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
				deps *models.Deps, ws *interfaces.Workspace, projectDir string,
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
	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Infra: map[string]models.InfraEntry{
			"postgres": {Inline: &models.Infra{Image: "postgres"}},
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
			LoadGlobalStateFunc: func() (*models.GlobalState, error) {
				return nil, errors.New("no state")
			},
		},
	})
	deps := &models.Deps{Project: models.Project{Name: "p"}}
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
			LoadGlobalStateFunc: func() (*models.GlobalState, error) {
				return &models.GlobalState{
					Projects: map[string]models.ProjectState{
						"other": {Name: "other"},
					},
				}, nil
			},
		},
	})
	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
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

	oldDeps := &models.Deps{
		Project:     models.Project{Name: "old"},
		ProjectRoot: "/old",
		Env: models.EnvConfig{
			Files:     []string{"old.env"},
			Variables: map[string]string{"K1": "old"},
		},
		Services: map[string]models.Service{
			"shared": {Source: models.SourceConfig{Kind: "git"}},
			"oldsvc": {Source: models.SourceConfig{Kind: "git"}},
		},
		Infra: map[string]models.InfraEntry{
			"postgres": {Inline: &models.Infra{Image: "postgres"}},
		},
	}

	newDeps := &models.Deps{
		Project: models.Project{Name: "new"},
		Env: models.EnvConfig{
			Files:     []string{"new.env"},
			Variables: map[string]string{"K1": "new", "K2": "val"},
		},
		Services: map[string]models.Service{
			"shared": {Source: models.SourceConfig{Kind: "image"}},
			"newsvc": {Source: models.SourceConfig{Kind: "git"}},
		},
		Infra: map[string]models.InfraEntry{
			"redis": {Inline: &models.Infra{Image: "redis"}},
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
	oldDeps := &models.Deps{
		Project:  models.Project{Name: "p"},
		Services: map[string]models.Service{},
		Infra:    map[string]models.InfraEntry{},
	}
	newDeps := &models.Deps{
		Project:  models.Project{Name: "p"},
		Services: map[string]models.Service{},
		Infra:    map[string]models.InfraEntry{},
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
			LoadFunc: func(ws *workspace.Workspace) (*models.Deps, error) {
				return nil, errors.New("load err")
			},
		},
	})
	deps := &models.Deps{Project: models.Project{Name: "p"}}
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
			LoadFunc: func(ws *workspace.Workspace) (*models.Deps, error) {
				return nil, nil
			},
		},
	})
	deps := &models.Deps{Project: models.Project{Name: "p"}}
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
	// ADR-011 Phase 3: the conflict detection feature is gone. The
	// function always returns (Proceed, nil, nil) — there is no merge
	// path because we can no longer materialize the "other project's"
	// deps without the legacy snapshot. Test pins the new contract.
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{
		Project:  models.Project{Name: "p"},
		Services: map[string]models.Service{},
		Infra:    map[string]models.InfraEntry{},
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
	if merged != nil {
		t.Error("expected nil merged deps after ADR-011 Phase 3")
	}
}

func TestCheckWorkspaceProjectConflictDifferentNoOverlap(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*models.Deps, error) {
				return &models.Deps{
					Project: models.Project{Name: "other"},
					Services: map[string]models.Service{
						"oldonly": {},
					},
					Infra: map[string]models.InfraEntry{},
				}, nil
			},
		},
	})
	deps := &models.Deps{
		Project: models.Project{Name: "mine"},
		Services: map[string]models.Service{
			"newonly": {},
		},
		Infra: map[string]models.InfraEntry{},
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
	deps := &models.Deps{Project: models.Project{Name: "p"}}
	uc.showSummary(
		context.Background(), deps,
		[]string{"a"}, []string{"b"},
		time.Now(),
	)
}
