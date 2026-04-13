package upcase

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// --- checkServiceHealthDefault / checkServiceHealth ---------------------------

func TestCheckServiceHealthDefaultNoPort(t *testing.T) {
	svc := config.Service{}
	ok, err := checkServiceHealthDefault(
		context.Background(), &workspace.Workspace{Root: "/t"}, "api", svc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("service with no port should not be healthy by default")
	}
}

func TestCheckServiceHealthDefaultInvalidPort(t *testing.T) {
	svc := config.Service{
		Docker: &config.DockerConfig{Ports: []string{"nonnumber:80"}},
	}
	ok, err := checkServiceHealthDefault(
		context.Background(), &workspace.Workspace{Root: "/t"}, "api", svc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("invalid port should return not healthy")
	}
}

func TestCheckServiceHealthDefaultClosedPort(t *testing.T) {
	svc := config.Service{
		// Use a very high port unlikely to be open
		Docker: &config.DockerConfig{Ports: []string{"59999:59999"}},
	}
	ok, err := checkServiceHealthDefault(
		context.Background(), &workspace.Workspace{Root: "/t"}, "api", svc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("closed port should not be healthy")
	}
}

func TestCheckServiceHealthNoCustomCommand(t *testing.T) {
	wm := &mocks.MockWorkspaceManager{
		GetServicePathFunc: func(
			ws *workspace.Workspace, n string, svc config.Service,
		) string {
			return "/x"
		},
	}
	svc := config.Service{}
	ok, err := checkServiceHealth(
		context.Background(), &workspace.Workspace{Root: "/t"},
		"api", svc, "dev", wm,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("no command + no port → not healthy")
	}
}

func TestCheckServiceHealthTrueCommand(t *testing.T) {
	wm := &mocks.MockWorkspaceManager{
		GetServicePathFunc: func(
			ws *workspace.Workspace, n string, svc config.Service,
		) string {
			return t.TempDir()
		},
	}
	svc := config.Service{
		Commands: &config.ServiceCommands{Health: "true"},
	}
	ok, err := checkServiceHealth(
		context.Background(), &workspace.Workspace{Root: "/t"},
		"api", svc, "dev", wm,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("true command should be healthy")
	}
}

func TestCheckServiceHealthFalseCommand(t *testing.T) {
	wm := &mocks.MockWorkspaceManager{
		GetServicePathFunc: func(
			ws *workspace.Workspace, n string, svc config.Service,
		) string {
			return t.TempDir()
		},
	}
	svc := config.Service{
		Commands: &config.ServiceCommands{Health: "false"},
	}
	ok, err := checkServiceHealth(
		context.Background(), &workspace.Workspace{Root: "/t"},
		"api", svc, "dev", wm,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("false command should not be healthy")
	}
}

// --- detectServiceConflict: no conflict scenarios -----------------------------

func TestDetectServiceConflictNoPreferenceNoComposeNoLocal(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(
				ws *workspace.Workspace, name string,
			) (*state.ServicePreference, error) {
				return nil, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	conflict, err := uc.detectServiceConflict(
		context.Background(), "api", deps,
		&workspace.Workspace{Root: "/t"}, "/proj", false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict != nil {
		t.Errorf("expected no conflict, got %+v", conflict)
	}
}

func TestDetectServiceConflictPreferenceMatching(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetComposePathFunc: func(ws *workspace.Workspace) string { return "" },
		},
		StateManager: &mocks.MockStateManager{
			GetServicePreferenceFunc: func(
				ws *workspace.Workspace, name string,
			) (*state.ServicePreference, error) {
				return &state.ServicePreference{
					ServiceName: "api", Preference: "ask",
				}, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	conflict, err := uc.detectServiceConflict(
		context.Background(), "api", deps,
		&workspace.Workspace{Root: "/t"}, "/proj", false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict != nil {
		t.Errorf("preference 'ask' should not trigger auto-conflict, got %+v", conflict)
	}
}

// --- applyServiceConflictResolution -------------------------------------------

func TestApplyServiceConflictResolutionLocalPref(t *testing.T) {
	initI18nUp(t)
	called := false
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			SetServicePreferenceFunc: func(
				ws *workspace.Workspace, pref state.ServicePreference,
			) error {
				called = true
				return nil
			},
		},
	})
	conflict := &ServiceConflict{
		ServiceName: "api", ConflictType: "local_running",
	}
	err := uc.applyServiceConflictResolution(
		context.Background(), conflict, "local_pref", "api",
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"}, "/proj", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("SetServicePreference not called")
	}
}

func TestApplyServiceConflictResolutionSkip(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	err := uc.applyServiceConflictResolution(
		context.Background(),
		&ServiceConflict{ServiceName: "api"}, "skip", "api",
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"}, "/p", true,
	)
	if err == nil {
		t.Error("skip should return error")
	}
}

func TestApplyServiceConflictResolutionCancel(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	err := uc.applyServiceConflictResolution(
		context.Background(),
		&ServiceConflict{ServiceName: "api"}, "cancel", "api",
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"}, "/p", true,
	)
	if err == nil {
		t.Error("cancel should return error")
	}
}

func TestApplyServiceConflictResolutionUnknown(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	err := uc.applyServiceConflictResolution(
		context.Background(),
		&ServiceConflict{ServiceName: "api"}, "weird", "api",
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"}, "/p", true,
	)
	if err == nil {
		t.Error("unknown resolution should return error")
	}
}

func TestApplyServiceConflictResolutionLocal(t *testing.T) {
	initI18nUp(t)
	stopped := false
	prefSaved := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetComposePathFunc: func(ws *workspace.Workspace) string {
				return "/path/compose.yml"
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			StopServiceWithContextFunc: func(
				ctx context.Context, cp, name string,
			) error {
				stopped = true
				return nil
			},
		},
		StateManager: &mocks.MockStateManager{
			SetServicePreferenceFunc: func(
				ws *workspace.Workspace, pref state.ServicePreference,
			) error {
				prefSaved = true
				return nil
			},
		},
	})
	err := uc.applyServiceConflictResolution(
		context.Background(),
		&ServiceConflict{ServiceName: "api", ConflictType: "cloned_running"},
		"local", "api",
		&config.Deps{Project: config.Project{Name: "p"}},
		&workspace.Workspace{Root: "/t"}, "/proj", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !stopped {
		t.Error("StopServiceWithContext not called")
	}
	if !prefSaved {
		t.Error("preference not saved")
	}
}

// --- updateHostPID ------------------------------------------------------------

func TestUpdateHostPIDPersists(t *testing.T) {
	dir := t.TempDir()
	// Create initial state with empty PIDs
	ls := &state.LocalState{Project: "p", HostPIDs: map[string]int{}}
	if err := state.SaveLocalState(dir, ls); err != nil {
		t.Fatal(err)
	}
	updateHostPID(dir, "svc", 4321)

	// Reload and verify
	reloaded, err := state.LoadLocalState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.HostPIDs["svc"] != 4321 {
		t.Errorf("pid = %d, want 4321", reloaded.HostPIDs["svc"])
	}
}

// --- mergeDeps with volumes ---------------------------------------------------

func TestMergeDepsResolvesVolumes(t *testing.T) {
	initI18nUp(t)
	resolveCalls := 0
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			ResolveRelativeVolumesFunc: func(
				volumes []string, projectDir string,
			) ([]string, error) {
				resolveCalls++
				// Prefix each with projectDir to simulate resolution
				out := make([]string, len(volumes))
				for i, v := range volumes {
					out[i] = projectDir + "/" + v
				}
				return out, nil
			},
		},
	})
	oldDeps := &config.Deps{
		Project:     config.Project{Name: "old"},
		ProjectRoot: "/old",
		Services: map[string]config.Service{
			"shared": {
				Source: config.SourceConfig{Kind: "image"},
				Docker: &config.DockerConfig{Volumes: []string{"data:/data"}},
			},
		},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{
				Image:   "postgres",
				Volumes: []string{"pgdata:/var"},
			}},
		},
	}
	newDeps := &config.Deps{
		Project: config.Project{Name: "new"},
		Services: map[string]config.Service{
			"shared": {
				Source: config.SourceConfig{Kind: "image"},
				Docker: &config.DockerConfig{Volumes: []string{"data2:/data"}},
			},
		},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{
				Image:   "postgres",
				Volumes: []string{"new:/var"},
			}},
		},
	}
	merged := uc.mergeDeps(oldDeps, newDeps, "/new")
	if merged == nil {
		t.Fatal("merged nil")
	}
	if resolveCalls == 0 {
		t.Error("expected ResolveRelativeVolumes to be called")
	}
}

// --- bootstrap: apply overrides path ------------------------------------------

func TestBootstrapOverridesError(t *testing.T) {
	// Can't easily error override application without mutating global state.
	// Instead, ensure the workspace prefix path executes when workspace is set.
	initI18nUp(t)
	deps := &config.Deps{
		SchemaVersion: "2.0",
		Workspace:     "myws",
		Project:       config.Project{Name: "p"},
		Services:      map[string]config.Service{},
		Infra:         map[string]config.InfraEntry{},
	}
	ws := &workspace.Workspace{Root: "/tmp/ws"}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return deps, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*workspace.Workspace, error) {
				return ws, nil
			},
		},
	})
	result, err := uc.bootstrap(context.Background(), ".raioz.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if result.deps.Workspace != "myws" {
		t.Errorf("expected workspace, got %q", result.deps.Workspace)
	}
}

// --- checkWorkspaceProjectConflict with overlap + nil preference -------------

// We can't easily exercise the interactive prompt path (reads stdin).
// But we can cover the branch where preference exists and is not "ask":
func TestCheckWorkspaceProjectConflictPreferenceReplace(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return &config.Deps{
					Project:  config.Project{Name: "other"},
					Services: map[string]config.Service{"shared": {}},
					Infra:    map[string]config.InfraEntry{},
				}, nil
			},
			GetWorkspaceProjectPreferenceFunc: func(
				name string,
			) (*state.WorkspaceProjectPreference, error) {
				return &state.WorkspaceProjectPreference{
					PreferredProject:   "mine",
					AlwaysAsk:          false,
					MergeWhenPreferred: false,
				}, nil
			},
		},
	})
	deps := &config.Deps{
		Project:  config.Project{Name: "mine"},
		Services: map[string]config.Service{"shared": {}},
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
	if merged != nil {
		t.Error("expected nil merged for replace preference")
	}
}

func TestCheckWorkspaceProjectConflictPreferenceMerge(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadFunc: func(ws *workspace.Workspace) (*config.Deps, error) {
				return &config.Deps{
					Project:  config.Project{Name: "other"},
					Services: map[string]config.Service{"shared": {}},
					Infra:    map[string]config.InfraEntry{},
				}, nil
			},
			GetWorkspaceProjectPreferenceFunc: func(
				name string,
			) (*state.WorkspaceProjectPreference, error) {
				return &state.WorkspaceProjectPreference{
					PreferredProject:   "mine",
					AlwaysAsk:          false,
					MergeWhenPreferred: true,
				}, nil
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			ResolveRelativeVolumesFunc: func(
				v []string, pd string,
			) ([]string, error) {
				return v, nil
			},
		},
	})
	deps := &config.Deps{
		Project:  config.Project{Name: "mine"},
		Services: map[string]config.Service{"shared": {}},
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
		t.Error("expected merged deps for merge preference")
	}
}

// --- checkAndHandleDuplicateProject: not-local path --------------------------

func TestCheckAndHandleDuplicateProjectNotLocal(t *testing.T) {
	initI18nUp(t)
	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)
	// Create a workspace path so it's NOT local
	wsPath := filepath.Join(raiozHome, "workspaces", "x")
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(wsPath, ".raioz.json")

	uc := NewUseCase(&Dependencies{})
	err := uc.checkAndHandleDuplicateProject(
		context.Background(), "p", configPath,
	)
	if err != nil {
		t.Errorf("should short-circuit when not local, got %v", err)
	}
}

// --- processGitRepos: no old deps, no git services → no-op -------------------

func TestProcessGitReposNoServices(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
	}
	err := uc.processGitRepos(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		nil, false, "/proj",
	)
	if err != nil {
		t.Errorf("no services → no-op, got %v", err)
	}
}

// --- checkInfraHealth with unknown containers (fast path) --------------------

func TestCheckInfraHealthNoContainers(t *testing.T) {
	initI18nUp(t)
	// This will attempt to look up container status via exec; we allow it to
	// fail and return "unknown" per branch — the function should loop and
	// eventually time out (short test via context cancellation not possible
	// since the function uses its own deadline).
	// To keep the test fast, pass an empty list which short-circuits.
	err := checkInfraHealth(context.Background(), []string{}, "p")
	if err != nil {
		t.Errorf("empty infra → nil, got %v", err)
	}
}

// --- executeLocalProjectCommand empty parts --------------------------------

func TestExecuteLocalProjectCommandWhitespace(t *testing.T) {
	initI18nUp(t)
	// Only whitespace → Fields returns empty → error
	err := executeLocalProjectCommand(context.Background(), t.TempDir(), "   ", "dev")
	if err == nil {
		t.Error("whitespace-only command should error")
	}
}

// --- mergeDeps simple ProjectRoot fallback verification -----------------------

func TestMergeDepsOldProjectRootUsed(t *testing.T) {
	initI18nUp(t)
	got := &config.Deps{}
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			ResolveRelativeVolumesFunc: func(
				volumes []string, projectDir string,
			) ([]string, error) {
				got = &config.Deps{ProjectRoot: projectDir}
				return volumes, nil
			},
		},
	})
	oldDeps := &config.Deps{
		Project:     config.Project{Name: "p"},
		ProjectRoot: "/old-proj",
		Services: map[string]config.Service{
			"a": {Docker: &config.DockerConfig{Volumes: []string{"x:/y"}}},
		},
		Infra: map[string]config.InfraEntry{},
	}
	newDeps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	uc.mergeDeps(oldDeps, newDeps, "/new")
	if got.ProjectRoot != "/old-proj" {
		t.Errorf("expected old project dir for old volumes, got %q", got.ProjectRoot)
	}
}

// --- saveState: with existing root config branch ------------------------------

func TestSaveStateWithExistingRoot(t *testing.T) {
	initI18nUp(t)
	wsDir := t.TempDir()
	ws := &workspace.Workspace{Root: wsDir}

	// Write a minimal existing raioz.root.json
	rootPath := filepath.Join(wsDir, "raioz.root.json")
	content := `{"version":"1","updatedAt":"2024-01-01T00:00:00Z","metadata":{}}`
	if err := os.WriteFile(rootPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{},
	})
	deps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	err := uc.saveState(
		context.Background(), deps, ws, "",
		nil, nil, nil, nil,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- dockerrunner failures surface as typed errors ----------------------------

func TestValidateDependencyAssistDetectErrorReturnsError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		Validator: &mocks.MockValidator{
			PreflightCheckWithContextFunc: func(ctx context.Context) error { return nil },
			AllFunc:                       func(*config.Deps) error { return nil },
			CheckWorkspacePermissionsFunc: func(p string) error { return nil },
		},
		Workspace: &mocks.MockWorkspaceManager{
			MigrateLegacyServicesFunc: func(
				ws *workspace.Workspace, d *config.Deps,
			) error {
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
				*config.Deps,
			) (map[string]interfaces.ServiceVolumes, error) {
				return nil, nil
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.DependencyConflict, error) {
				return nil, nil
			},
			DetectMissingDependenciesFunc: func(
				*config.Deps, func(string, config.Service) string,
			) ([]config.MissingDependency, error) {
				return nil, stderrors.New("detection failed")
			},
		},
	})
	err := uc.validate(
		context.Background(), &config.Deps{},
		&workspace.Workspace{Root: "/t"}, false,
	)
	if err == nil {
		t.Error("expected error when missing-deps detection fails")
	}
}
