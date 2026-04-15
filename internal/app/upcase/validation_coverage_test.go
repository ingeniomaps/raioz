package upcase

import (
	"context"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// --- CheckProxyRequirements --------------------------------------------------

func TestCheckProxyRequirements_ProxyDisabled(t *testing.T) {
	deps := &config.Deps{Proxy: false}
	if err := CheckProxyRequirements(deps); err != nil {
		t.Errorf("proxy disabled should not error: %v", err)
	}
}

func TestCheckProxyRequirements_NonMkcertTLS(t *testing.T) {
	deps := &config.Deps{
		Proxy: true,
		ProxyConfig: &config.ProxyConfig{
			TLS: "letsencrypt",
		},
	}
	if err := CheckProxyRequirements(deps); err != nil {
		t.Errorf("non-mkcert TLS should not error: %v", err)
	}
}

// --- checkWorkspaceProjectConflict additional branches -----------------------

func TestCheckWorkspaceProjectConflict_PreferredProjectMatchNoMerge(t *testing.T) {
	oldDeps := &config.Deps{
		Project:  config.Project{Name: "old-proj"},
		Services: map[string]config.Service{"api": {}},
		Infra:    map[string]config.InfraEntry{},
	}
	currentDeps := &config.Deps{
		Project:  config.Project{Name: "new-proj"},
		Services: map[string]config.Service{"api": {}}, // overlap
		Infra:    map[string]config.InfraEntry{},
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*config.Deps, error) {
			return oldDeps, nil
		},
		GetWorkspaceProjectPreferenceFunc: func(ws string) (*state.WorkspaceProjectPreference, error) {
			return &state.WorkspaceProjectPreference{
				PreferredProject:   "new-proj",
				AlwaysAsk:          false,
				MergeWhenPreferred: false,
			}, nil
		},
		CompareDepsFunc: func(old, new *config.Deps) ([]state.ConfigChange, error) {
			return nil, nil
		},
		FormatChangesFunc: func(changes []state.ConfigChange) string {
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

func TestCheckWorkspaceProjectConflict_PreferredProjectMatchWithMerge(t *testing.T) {
	oldDeps := &config.Deps{
		Project:  config.Project{Name: "old-proj"},
		Services: map[string]config.Service{"api": {}},
		Infra:    map[string]config.InfraEntry{},
	}
	currentDeps := &config.Deps{
		Project:  config.Project{Name: "new-proj"},
		Services: map[string]config.Service{"api": {}},
		Infra:    map[string]config.InfraEntry{},
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*config.Deps, error) {
			return oldDeps, nil
		},
		GetWorkspaceProjectPreferenceFunc: func(ws string) (*state.WorkspaceProjectPreference, error) {
			return &state.WorkspaceProjectPreference{
				PreferredProject:   "new-proj",
				AlwaysAsk:          false,
				MergeWhenPreferred: true,
			}, nil
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
	if merged == nil {
		t.Error("mergedDeps should not be nil when MergeWhenPreferred=true")
	}
}

func TestCheckWorkspaceProjectConflict_NoOverlapInfra(t *testing.T) {
	oldDeps := &config.Deps{
		Project:  config.Project{Name: "old-proj"},
		Services: map[string]config.Service{"web": {}},
		Infra:    map[string]config.InfraEntry{"postgres": {}},
	}
	currentDeps := &config.Deps{
		Project:  config.Project{Name: "new-proj"},
		Services: map[string]config.Service{"api": {}},      // no overlap
		Infra:    map[string]config.InfraEntry{"redis": {}}, // no overlap
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*config.Deps, error) {
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
	oldDeps := &config.Deps{
		Project:  config.Project{Name: "old-proj"},
		Services: map[string]config.Service{"web": {}},
		Infra:    map[string]config.InfraEntry{"postgres": {}},
	}
	currentDeps := &config.Deps{
		Project:  config.Project{Name: "new-proj"},
		Services: map[string]config.Service{"api": {}},
		Infra:    map[string]config.InfraEntry{"postgres": {}}, // overlap
	}

	sm := &mocks.MockStateManager{
		LoadFunc: func(ws *interfaces.Workspace) (*config.Deps, error) {
			return oldDeps, nil
		},
		GetWorkspaceProjectPreferenceFunc: func(ws string) (*state.WorkspaceProjectPreference, error) {
			return &state.WorkspaceProjectPreference{
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
