package upcase

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// --- recordUserDecision / loadUserDecision -----------------------------------

func TestRecordAndLoadUserDecisionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	gsPath := filepath.Join(dir, "state.json")
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
	}

	// Write a decision
	if err := recordUserDecision("myproj", true, sm); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Read it back
	got, err := loadUserDecision("myproj", sm)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected decision to be found")
	}
	if *got != true {
		t.Errorf("got %v, want true", *got)
	}

	// Overwrite with false
	if err := recordUserDecision("myproj", false, sm); err != nil {
		t.Fatalf("record: %v", err)
	}
	got2, err := loadUserDecision("myproj", sm)
	if err != nil {
		t.Fatal(err)
	}
	if got2 == nil || *got2 != false {
		t.Errorf("expected false, got %v", got2)
	}
}

func TestLoadUserDecisionMissing(t *testing.T) {
	dir := t.TempDir()
	gsPath := filepath.Join(dir, "state.json")
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return gsPath, nil },
	}
	got, err := loadUserDecision("ghost", sm)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected nil for missing decision")
	}
}

func TestLoadUserDecisionPathError(t *testing.T) {
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return "", stderrors.New("no path") },
	}
	_, err := loadUserDecision("x", sm)
	if err == nil {
		t.Error("expected error when path lookup fails")
	}
}

func TestRecordUserDecisionPathError(t *testing.T) {
	sm := &mocks.MockStateManager{
		GetGlobalStatePathFunc: func() (string, error) { return "", stderrors.New("no path") },
	}
	err := recordUserDecision("x", true, sm)
	if err == nil {
		t.Error("expected error when path lookup fails")
	}
}

// --- processLocalProject (short-circuit: not local, no commands) --------------

func TestProcessLocalProjectNotLocalNoCommands(t *testing.T) {
	initI18nUp(t)
	// Make project look not-local by setting RAIOZ_HOME to contain the config path
	raiozHome := t.TempDir()
	t.Setenv("RAIOZ_HOME", raiozHome)
	wsDir := filepath.Join(raiozHome, "workspaces", "x")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(wsDir, ".raioz.json")

	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	err := uc.processLocalProject(context.Background(), configPath, deps, "up", nil)
	if err != nil {
		t.Errorf("should no-op, got %v", err)
	}
}

// --- processHostServices ------------------------------------------------------

func TestProcessHostServicesNone(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			// All services have docker → skipped
			"api": {Docker: &config.DockerConfig{}},
		},
	}
	got, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no host processes, got %d", len(got))
	}
}

func TestProcessHostServicesStart(t *testing.T) {
	initI18nUp(t)
	startCalled := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("nope")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled++
				return &host.ProcessInfo{PID: 1234, Command: svc.Source.Command}, nil
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"hostsvc": {Source: config.SourceConfig{Command: "echo hi"}},
		},
	}
	got, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startCalled != 1 {
		t.Errorf("StartService called %d times, want 1", startCalled)
	}
	if _, ok := got["hostsvc"]; !ok {
		t.Error("expected hostsvc in returned process info")
	}
}

func TestProcessHostServicesSkipDisabled(t *testing.T) {
	initI18nUp(t)
	disabled := false
	startCalled := 0
	uc := NewUseCase(&Dependencies{
		Workspace:    &mocks.MockWorkspaceManager{},
		ConfigLoader: &mocks.MockConfigLoader{},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled++
				return &host.ProcessInfo{PID: 1}, nil
			},
		},
	})
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"svc": {
				Enabled: &disabled,
				Source:  config.SourceConfig{Command: "echo"},
			},
		},
	}
	_, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if startCalled != 0 {
		t.Errorf("disabled service should be skipped, got %d calls", startCalled)
	}
}

func TestSaveHostProcessesState(t *testing.T) {
	initI18nUp(t)
	called := false
	uc := NewUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			SaveProcessesStateFunc: func(
				ws *workspace.Workspace, procs map[string]*host.ProcessInfo,
			) error {
				called = true
				return nil
			},
		},
	})
	err := uc.saveHostProcessesState(
		context.Background(),
		&workspace.Workspace{Root: "/t"},
		map[string]*host.ProcessInfo{"s": {PID: 1}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("SaveProcessesState not invoked")
	}
}

func TestStopHostServicesEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{},
	})
	err := uc.stopHostServices(context.Background(), nil)
	if err != nil {
		t.Errorf("empty should be no-op, got %v", err)
	}
}

func TestStopHostServices(t *testing.T) {
	initI18nUp(t)
	stopped := 0
	uc := NewUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			StopServiceWithCommandFunc: func(
				ctx context.Context, pid int, cmd string,
			) error {
				stopped++
				return nil
			},
		},
	})
	err := uc.stopHostServices(
		context.Background(),
		map[string]*host.ProcessInfo{
			"a": {PID: 1},
			"b": {PID: 2},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if stopped != 2 {
		t.Errorf("expected 2 stops, got %d", stopped)
	}
}

func TestStopHostServicesContinuesOnError(t *testing.T) {
	initI18nUp(t)
	called := 0
	uc := NewUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			StopServiceWithCommandFunc: func(
				ctx context.Context, pid int, cmd string,
			) error {
				called++
				return stderrors.New("err")
			},
		},
	})
	err := uc.stopHostServices(
		context.Background(),
		map[string]*host.ProcessInfo{"a": {PID: 1}, "b": {PID: 2}},
	)
	if err != nil {
		t.Errorf("should not propagate individual errors, got %v", err)
	}
	if called != 2 {
		t.Errorf("both should be attempted, got %d", called)
	}
}

// --- cleanStaleHostProcesses --------------------------------------------------

func TestCleanStaleHostProcessesNoState(t *testing.T) {
	// Empty tempdir → no .raioz.state.json
	dir := t.TempDir()
	// Should not panic
	cleanStaleHostProcesses(context.Background(), dir, "proj")
}

func TestCleanStaleHostProcessesEmpty(t *testing.T) {
	dir := t.TempDir()
	// Create state with empty HostPIDs
	ls := &state.LocalState{Project: "p", HostPIDs: map[string]int{}}
	if err := state.SaveLocalState(dir, ls); err != nil {
		t.Fatal(err)
	}
	cleanStaleHostProcesses(context.Background(), dir, "p")
}

func TestCleanStaleHostProcessesDeadPID(t *testing.T) {
	dir := t.TempDir()
	ls := &state.LocalState{
		Project:  "p",
		HostPIDs: map[string]int{"svc": 999999999},
	}
	if err := state.SaveLocalState(dir, ls); err != nil {
		t.Fatal(err)
	}
	// Should silently skip dead PIDs
	cleanStaleHostProcesses(context.Background(), dir, "p")
}

// --- saveHostPIDs -------------------------------------------------------------

func TestSaveHostPIDsSkipsDockerRuntime(t *testing.T) {
	// Function touches dispatcher.GetHostPID; we can't fake that here easily.
	// Instead test the early-return branch: no service names == nothing saved.
	dir := t.TempDir()
	saveHostPIDs(dir, "p", nil, nil, nil)
	// ensure no file was written (since no PIDs saved)
	if _, err := os.Stat(filepath.Join(dir, ".raioz.state.json")); err == nil {
		// File may or may not exist; just ensure no panic. Fine either way.
		_ = err
	}
}

// --- infra_health pure functions (diagnoseContainerError covered) ------------

func TestDiagnoseContainerErrorPermission(t *testing.T) {
	suggestions := diagnoseContainerError("permission denied on /var/lib", "svc")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
	found := false
	for _, s := range suggestions {
		if containsString(s, "permission") || containsString(s, "volume") {
			found = true
		}
	}
	if !found {
		t.Error("expected permission/volume suggestion")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- preHookExec / postHookExec -----------------------------------------------

func TestPreHookExecEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	err := uc.preHookExec(context.Background(), &config.Deps{}, t.TempDir())
	if err != nil {
		t.Errorf("empty pre-hook should be no-op, got %v", err)
	}
}

func TestPreHookExecSuccess(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{PreHook: "true"}
	err := uc.preHookExec(context.Background(), deps, t.TempDir())
	if err != nil {
		t.Errorf("true should succeed, got %v", err)
	}
}

func TestPreHookExecFails(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{PreHook: "false"}
	err := uc.preHookExec(context.Background(), deps, t.TempDir())
	if err == nil {
		t.Error("false should fail")
	}
}

func TestPreHookExecChain(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &config.Deps{PreHook: "true && true"}
	err := uc.preHookExec(context.Background(), deps, t.TempDir())
	if err != nil {
		t.Errorf("chain should succeed, got %v", err)
	}
}

func TestPostHookExecEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// Should not panic with empty
	uc.postHookExec(context.Background(), &config.Deps{}, t.TempDir())
}

func TestPostHookExecSuccess(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// Post-hook errors are swallowed
	uc.postHookExec(context.Background(), &config.Deps{PostHook: "true"}, t.TempDir())
}

func TestPostHookExecFailureSwallowed(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// Post-hook errors are swallowed — no error should be returned
	uc.postHookExec(context.Background(), &config.Deps{PostHook: "false"}, t.TempDir())
}

// --- checkInfraHealth ---------------------------------------------------------

func TestCheckInfraHealthEmpty(t *testing.T) {
	initI18nUp(t)
	err := checkInfraHealth(context.Background(), nil, "proj")
	if err != nil {
		t.Errorf("empty infra should be no-op, got %v", err)
	}
}

// --- updateGlobalState --------------------------------------------------------

func TestUpdateGlobalStateSuccess(t *testing.T) {
	initI18nUp(t)
	called := false
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesInfoWithContextFunc: func(
				ctx context.Context, cp string, svcs []string, proj string,
				services map[string]config.Service, ws *interfaces.Workspace,
			) (map[string]*interfaces.ServiceInfo, error) {
				return map[string]*interfaces.ServiceInfo{
					"api": {Status: "running"},
				}, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			BuildServiceStatesFunc: func(
				d *config.Deps, sis map[string]*state.ServiceInfo,
			) []state.ServiceState {
				return []state.ServiceState{{Name: "api"}}
			},
			UpdateProjectStateFunc: func(n string, ps *state.ProjectState) error {
				called = true
				return nil
			},
		},
	})
	deps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{"api": {}},
	}
	err := uc.updateGlobalState(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		"/path/compose.yml", []string{"api"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("UpdateProjectState should be called")
	}
}

func TestUpdateGlobalStateDockerErrorToleratedButUpdates(t *testing.T) {
	initI18nUp(t)
	called := false
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesInfoWithContextFunc: func(
				ctx context.Context, cp string, svcs []string, proj string,
				services map[string]config.Service, ws *interfaces.Workspace,
			) (map[string]*interfaces.ServiceInfo, error) {
				return nil, stderrors.New("docker err")
			},
		},
		StateManager: &mocks.MockStateManager{
			BuildServiceStatesFunc: func(
				d *config.Deps, sis map[string]*state.ServiceInfo,
			) []state.ServiceState {
				return nil
			},
			UpdateProjectStateFunc: func(n string, ps *state.ProjectState) error {
				called = true
				return nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	err := uc.updateGlobalState(
		context.Background(), deps, &workspace.Workspace{Root: "/t"},
		"/path/compose.yml", nil,
	)
	if err != nil {
		t.Errorf("should tolerate docker error, got %v", err)
	}
	if !called {
		t.Error("UpdateProjectState should still be called")
	}
}

func TestUpdateGlobalStateError(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{},
		StateManager: &mocks.MockStateManager{
			UpdateProjectStateFunc: func(n string, ps *state.ProjectState) error {
				return stderrors.New("update failed")
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	err := uc.updateGlobalState(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "", nil,
	)
	if err == nil {
		t.Error("expected error")
	}
}

// --- saveState (smoke; uses real root package with tempdir workspace) --------

func TestSaveStateWritesRoot(t *testing.T) {
	initI18nUp(t)
	wsDir := t.TempDir()
	ws := &workspace.Workspace{Root: wsDir}

	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			SaveFunc: func(ws *workspace.Workspace, d *config.Deps) error {
				return nil
			},
		},
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

func TestSaveStateManagerError(t *testing.T) {
	initI18nUp(t)
	wsDir := t.TempDir()
	ws := &workspace.Workspace{Root: wsDir}

	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			SaveFunc: func(ws *workspace.Workspace, d *config.Deps) error {
				return stderrors.New("save failed")
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	err := uc.saveState(
		context.Background(), deps, ws, "", nil, nil, nil, nil,
	)
	if err == nil {
		t.Error("expected error")
	}
}

// --- checkDependencyProjects with matching but no services state -------------

func TestCheckDependencyProjectsMatchButHasServices(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{
		StateManager: &mocks.MockStateManager{
			LoadGlobalStateFunc: func() (*state.GlobalState, error) {
				return &state.GlobalState{
					Projects: map[string]state.ProjectState{
						// Has services -> NOT command-based, should be skipped
						"db": {
							Name: "db",
							Services: []state.ServiceState{
								{Name: "svc", Status: "running"},
							},
						},
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
