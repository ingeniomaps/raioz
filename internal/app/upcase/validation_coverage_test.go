package upcase

import (
	"context"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- CheckProxyRequirements --------------------------------------------------

func TestCheckProxyRequirements_ProxyDisabled(t *testing.T) {
	deps := &models.Deps{Proxy: false}
	if err := CheckProxyRequirements(deps); err != nil {
		t.Errorf("proxy disabled should not error: %v", err)
	}
}

func TestCheckProxyRequirements_NonMkcertTLS(t *testing.T) {
	deps := &models.Deps{
		Proxy: true,
		ProxyConfig: &models.ProxyConfig{
			TLS: "letsencrypt",
		},
	}
	if err := CheckProxyRequirements(deps); err != nil {
		t.Errorf("non-mkcert TLS should not error: %v", err)
	}
}

// --- checkWorkspaceProjectConflict additional branches -----------------------

func TestCheckWorkspaceProjectConflict_PreferredProjectMatchNoMerge(t *testing.T) {
	oldDeps := &models.Deps{
		Project:  models.Project{Name: "old-proj"},
		Services: map[string]models.Service{"api": {}},
		Infra:    map[string]models.InfraEntry{},
	}
	currentDeps := &models.Deps{
		Project:  models.Project{Name: "new-proj"},
		Services: map[string]models.Service{"api": {}}, // overlap
		Infra:    map[string]models.InfraEntry{},
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*models.Deps, error) {
			return oldDeps, nil
		},
		GetWorkspaceProjectPreferenceFunc: func(ws string) (*models.WorkspaceProjectPreference, error) {
			return &models.WorkspaceProjectPreference{
				PreferredProject:   "new-proj",
				AlwaysAsk:          false,
				MergeWhenPreferred: false,
			}, nil
		},
		CompareDepsFunc: func(old, new *models.Deps) ([]models.ConfigChange, error) {
			return nil, nil
		},
		FormatChangesFunc: func(changes []models.ConfigChange) string {
			return ""
		},
	}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
	}}

	ws := &workspace.Workspace{}
	result, merged, err := uc.checkWorkspaceProjectConflict(
		context.Background(), currentDeps, ws, "/tmp/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("result = %d, want Proceed", result)
	}
	if merged != nil {
		t.Error("mergedDeps should be nil when MergeWhenPreferred=false")
	}
}

// TestCheckWorkspaceProjectConflict_PreferredProjectMatchWithMerge removed:
// ADR-011 Phase 3 dropped the workspace-conflict prompt and its
// MergeWhenPreferred branch.

func TestCheckWorkspaceProjectConflict_NoOverlapInfra(t *testing.T) {
	oldDeps := &models.Deps{
		Project:  models.Project{Name: "old-proj"},
		Services: map[string]models.Service{"web": {}},
		Infra:    map[string]models.InfraEntry{"postgres": {}},
	}
	currentDeps := &models.Deps{
		Project:  models.Project{Name: "new-proj"},
		Services: map[string]models.Service{"api": {}},      // no overlap
		Infra:    map[string]models.InfraEntry{"redis": {}}, // no overlap
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*models.Deps, error) {
			return oldDeps, nil
		},
	}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
	}}

	ws := &workspace.Workspace{}
	result, merged, err := uc.checkWorkspaceProjectConflict(
		context.Background(), currentDeps, ws, "/tmp/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("result = %d, want Proceed (no overlap)", result)
	}
	if merged != nil {
		t.Error("merged should be nil with no overlap")
	}
}

func TestCheckWorkspaceProjectConflict_InfraOverlap(t *testing.T) {
	oldDeps := &models.Deps{
		Project:  models.Project{Name: "old-proj"},
		Services: map[string]models.Service{"web": {}},
		Infra:    map[string]models.InfraEntry{"postgres": {}},
	}
	currentDeps := &models.Deps{
		Project:  models.Project{Name: "new-proj"},
		Services: map[string]models.Service{"api": {}},
		Infra:    map[string]models.InfraEntry{"postgres": {}}, // overlap
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*models.Deps, error) {
			return oldDeps, nil
		},
		GetWorkspaceProjectPreferenceFunc: func(ws string) (*models.WorkspaceProjectPreference, error) {
			return &models.WorkspaceProjectPreference{
				PreferredProject: "new-proj",
				AlwaysAsk:        false,
			}, nil
		},
	}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
	}}

	ws := &workspace.Workspace{}
	result, _, err := uc.checkWorkspaceProjectConflict(
		context.Background(), currentDeps, ws, "/tmp/proj",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != WorkspaceConflictProceed {
		t.Errorf("result = %d, want Proceed (preferred match)", result)
	}
}
