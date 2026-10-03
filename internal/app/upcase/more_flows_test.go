package upcase

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- recordUserDecision / loadUserDecision -----------------------------------

// --- processLocalProject (short-circuit: not local, no commands) --------------

// --- cleanStaleHostProcesses --------------------------------------------------

// --- saveHostPIDs -------------------------------------------------------------

func TestSaveHostPIDsSkipsDockerRuntime(t *testing.T) {
	// Function touches dispatcher.GetHostPID; we can't fake that here easily.
	// Instead test the early-return branch: no service names == nothing saved.
	dir := t.TempDir()
	saveHostPIDs(dir, "p", "", "net", nil, nil, nil, nil)
	// State file must now exist even without host PIDs — the project +
	// network provenance is on its own worth persisting.
	if _, err := os.Stat(filepath.Join(dir, ".raioz.state.json")); err != nil {
		t.Errorf("expected state file to be written, got %v", err)
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
	err := uc.preHookExec(context.Background(), &models.Deps{}, t.TempDir())
	if err != nil {
		t.Errorf("empty pre-hook should be no-op, got %v", err)
	}
}

func TestPreHookExecSuccess(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{PreHook: "true"}
	err := uc.preHookExec(context.Background(), deps, t.TempDir())
	if err != nil {
		t.Errorf("true should succeed, got %v", err)
	}
}

func TestPreHookExecFails(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{PreHook: "false"}
	err := uc.preHookExec(context.Background(), deps, t.TempDir())
	if err == nil {
		t.Error("false should fail")
	}
}

func TestPreHookExecChain(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	deps := &models.Deps{PreHook: "true && true"}
	err := uc.preHookExec(context.Background(), deps, t.TempDir())
	if err != nil {
		t.Errorf("chain should succeed, got %v", err)
	}
}

// --- preUpHookExec ------------------------------------------------------------

func TestPreUpHookExec(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})

	t.Run("empty hook is no-op", func(t *testing.T) {
		if err := uc.preUpHookExec(context.Background(), &models.Deps{}, t.TempDir()); err != nil {
			t.Errorf("empty hook returned %v", err)
		}
	})

	t.Run("success returns nil", func(t *testing.T) {
		if err := uc.preUpHookExec(
			context.Background(),
			&models.Deps{PreUpHook: "true"},
			t.TempDir(),
		); err != nil {
			t.Errorf("true exit returned %v", err)
		}
	})

	t.Run("failure aborts", func(t *testing.T) {
		err := uc.preUpHookExec(
			context.Background(),
			&models.Deps{PreUpHook: "false"},
			t.TempDir(),
		)
		if err == nil {
			t.Error("false exit must return error")
		}
	})

	t.Run("chain runs both commands", func(t *testing.T) {
		if err := uc.preUpHookExec(
			context.Background(),
			&models.Deps{PreUpHook: "true && true"},
			t.TempDir(),
		); err != nil {
			t.Errorf("chain returned %v", err)
		}
	})
}

func TestPostHookExecEmpty(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// Should not panic with empty
	uc.postHookExec(context.Background(), &models.Deps{}, t.TempDir())
}

func TestPostHookExecSuccess(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// Post-hook errors are swallowed
	uc.postHookExec(context.Background(), &models.Deps{PostHook: "true"}, t.TempDir())
}

func TestPostHookExecFailureSwallowed(t *testing.T) {
	initI18nUp(t)
	uc := NewUseCase(&Dependencies{})
	// Post-hook errors are swallowed — no error should be returned
	uc.postHookExec(context.Background(), &models.Deps{PostHook: "false"}, t.TempDir())
}

// --- checkInfraHealth ---------------------------------------------------------

func TestCheckInfraHealthEmpty(t *testing.T) {
	initI18nUp(t)
	err := checkInfraHealth(context.Background(), nil, "proj", nil)
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
				services map[string]models.Service, ws *interfaces.Workspace,
			) (map[string]*interfaces.ServiceInfo, error) {
				return map[string]*interfaces.ServiceInfo{
					"api": {Status: "running"},
				}, nil
			},
		},
		StateManager: &mocks.MockStateManager{
			BuildServiceStatesFunc: func(
				d *models.Deps, sis map[string]*models.ServiceInfo,
			) []models.ServiceState {
				return []models.ServiceState{{Name: "api"}}
			},
			UpdateProjectStateFunc: func(n string, ps *models.ProjectState) error {
				called = true
				return nil
			},
		},
	})
	deps := &models.Deps{
		Project:  models.Project{Name: "p"},
		Services: map[string]models.Service{"api": {}},
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
				services map[string]models.Service, ws *interfaces.Workspace,
			) (map[string]*interfaces.ServiceInfo, error) {
				return nil, stderrors.New("docker err")
			},
		},
		StateManager: &mocks.MockStateManager{
			BuildServiceStatesFunc: func(
				d *models.Deps, sis map[string]*models.ServiceInfo,
			) []models.ServiceState {
				return nil
			},
			UpdateProjectStateFunc: func(n string, ps *models.ProjectState) error {
				called = true
				return nil
			},
		},
	})
	deps := &models.Deps{Project: models.Project{Name: "p"}}
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
			UpdateProjectStateFunc: func(n string, ps *models.ProjectState) error {
				return stderrors.New("update failed")
			},
		},
	})
	deps := &models.Deps{Project: models.Project{Name: "p"}}
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
			SaveFunc: func(ws *workspace.Workspace, d *models.Deps) error {
				return nil
			},
		},
	})
	deps := &models.Deps{
		Project:  models.Project{Name: "p"},
		Services: map[string]models.Service{},
		Infra:    map[string]models.InfraEntry{},
	}
	err := uc.saveState(
		context.Background(), deps, ws, "",
		nil, nil, nil, nil,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSaveStateManagerError removed: ADR-011 Phase 1 eliminated the
// StateManager.Save call from saveState. The error-propagation path the
// test exercised no longer exists. The remaining saveState behavior
// (root config generation) is covered by TestSaveStateWritesRoot above.

// --- checkDependencyProjects with matching but no services state -------------
