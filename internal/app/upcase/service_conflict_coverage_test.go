package upcase

import (
	"context"
	"testing"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// --- detectServiceConflict ---------------------------------------------------

func TestDetectServiceConflict_NoConflict(t *testing.T) {
	sm := &mocks.MockStateManager{
		GetServicePreferenceFunc: func(ws *interfaces.Workspace, name string) (*state.ServicePreference, error) {
			return nil, nil
		},
	}
	wm := &mocks.MockWorkspaceManager{
		GetComposePathFunc: func(ws *interfaces.Workspace) string { return "" },
	}
	dr := &mocks.MockDockerRunner{}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
		Workspace:    wm,
		DockerRunner: dr,
	}}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	ws := &workspace.Workspace{}

	conflict, err := uc.detectServiceConflict(
		context.Background(), "api", deps, ws, "/tmp/proj", false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict != nil {
		t.Errorf("expected no conflict, got %+v", conflict)
	}
}

func TestDetectServiceConflict_ClonedRunning(t *testing.T) {
	sm := &mocks.MockStateManager{
		GetServicePreferenceFunc: func(ws *interfaces.Workspace, name string) (*state.ServicePreference, error) {
			return nil, nil
		},
		LoadGlobalStateFunc: func() (*state.GlobalState, error) {
			return &state.GlobalState{
				Projects: map[string]state.ProjectState{
					"other-proj": {
						Services: []state.ServiceState{
							{Name: "api", Status: "running"},
						},
					},
				},
			}, nil
		},
	}
	wm := &mocks.MockWorkspaceManager{
		GetComposePathFunc: func(ws *interfaces.Workspace) string {
			return "/tmp/ws/docker-compose.yml"
		},
	}
	dr := &mocks.MockDockerRunner{
		GetContainerNameWithContextFunc: func(ctx context.Context, composePath, svcName string) (string, error) {
			return "ws-api-1", nil
		},
	}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
		Workspace:    wm,
		DockerRunner: dr,
	}}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	ws := &workspace.Workspace{}

	conflict, err := uc.detectServiceConflict(
		context.Background(), "api", deps, ws, "/tmp/proj", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil {
		t.Fatal("expected conflict")
	}
	if conflict.ConflictType != "cloned_running" {
		t.Errorf("ConflictType = %s, want cloned_running", conflict.ConflictType)
	}
	if conflict.CurrentProject != "other-proj" {
		t.Errorf("CurrentProject = %s, want other-proj", conflict.CurrentProject)
	}
	if conflict.CurrentContainer != "ws-api-1" {
		t.Errorf("CurrentContainer = %s, want ws-api-1", conflict.CurrentContainer)
	}
}

func TestDetectServiceConflict_PreferenceLocal(t *testing.T) {
	pref := &state.ServicePreference{
		ServiceName: "web",
		Preference:  "local",
		ProjectPath: "/home/user/web",
		Workspace:   "acme",
		Timestamp:   time.Now(),
	}
	sm := &mocks.MockStateManager{
		GetServicePreferenceFunc: func(ws *interfaces.Workspace, name string) (*state.ServicePreference, error) {
			return pref, nil
		},
	}
	wm := &mocks.MockWorkspaceManager{
		GetComposePathFunc: func(ws *interfaces.Workspace) string { return "" },
	}
	dr := &mocks.MockDockerRunner{
		NormalizeContainerNameFunc: func(workspace, service, project string, global bool) (string, error) {
			return "acme-web-1", nil
		},
	}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
		Workspace:    wm,
		DockerRunner: dr,
	}}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	ws := &workspace.Workspace{}

	// isLocalProject=false + pref="local" → conflict
	conflict, err := uc.detectServiceConflict(
		context.Background(), "web", deps, ws, "/tmp/proj", false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil {
		t.Fatal("expected preference conflict")
	}
	if conflict.ConflictType != "preference" {
		t.Errorf("ConflictType = %s, want preference", conflict.ConflictType)
	}
}

func TestDetectServiceConflict_PreferenceCloned(t *testing.T) {
	pref := &state.ServicePreference{
		ServiceName: "db",
		Preference:  "cloned",
		Workspace:   "acme",
		Timestamp:   time.Now(),
	}
	sm := &mocks.MockStateManager{
		GetServicePreferenceFunc: func(ws *interfaces.Workspace, name string) (*state.ServicePreference, error) {
			return pref, nil
		},
	}
	wm := &mocks.MockWorkspaceManager{
		GetComposePathFunc: func(ws *interfaces.Workspace) string { return "" },
	}
	dr := &mocks.MockDockerRunner{
		NormalizeContainerNameFunc: func(workspace, service, project string, global bool) (string, error) {
			return "acme-db-1", nil
		},
	}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
		Workspace:    wm,
		DockerRunner: dr,
	}}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	ws := &workspace.Workspace{}

	// isLocalProject=true + pref="cloned" → conflict
	conflict, err := uc.detectServiceConflict(
		context.Background(), "db", deps, ws, "/tmp/proj", true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil {
		t.Fatal("expected preference conflict")
	}
	if conflict.ConflictType != "preference" {
		t.Errorf("ConflictType = %s, want preference", conflict.ConflictType)
	}
	if conflict.CurrentContainer != "acme-db-1" {
		t.Errorf("CurrentContainer = %s, want acme-db-1", conflict.CurrentContainer)
	}
}

func TestDetectServiceConflict_PreferenceAskNoConflict(t *testing.T) {
	pref := &state.ServicePreference{
		ServiceName: "api",
		Preference:  "ask",
		Timestamp:   time.Now(),
	}
	sm := &mocks.MockStateManager{
		GetServicePreferenceFunc: func(ws *interfaces.Workspace, name string) (*state.ServicePreference, error) {
			return pref, nil
		},
	}
	wm := &mocks.MockWorkspaceManager{
		GetComposePathFunc: func(ws *interfaces.Workspace) string { return "" },
	}
	dr := &mocks.MockDockerRunner{}

	uc := &UseCase{deps: &Dependencies{
		StateManager: sm,
		Workspace:    wm,
		DockerRunner: dr,
	}}

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	ws := &workspace.Workspace{}

	conflict, err := uc.detectServiceConflict(
		context.Background(), "api", deps, ws, "/tmp/proj", false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if conflict != nil {
		t.Error("preference=ask with no running container should not conflict")
	}
}
