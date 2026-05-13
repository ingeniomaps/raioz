package upcase

import (
	"context"
	stderrors "errors"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- processHostServices: service with commands but no source.command ---------

func TestProcessHostServicesWithServiceCommands(t *testing.T) {
	initI18nUp(t)

	startCalled := 0
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *models.Deps,
				n string, svc models.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled++
				return &host.ProcessInfo{PID: 100, Command: "npm start"}, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"web": {
				Commands: &models.ServiceCommands{
					Dev: &models.EnvironmentCommands{Up: "npm start"},
				},
			},
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
	if _, ok := got["web"]; !ok {
		t.Error("expected web in result")
	}
}

// --- processHostServices: start error ----------------------------------------

func TestProcessHostServicesStartError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return nil, "", stderrors.New("nope")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *models.Deps,
				n string, svc models.Service, pd string,
			) (*host.ProcessInfo, error) {
				return nil, stderrors.New("start failed")
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"worker": {Source: models.SourceConfig{Command: "node worker.js"}},
		},
	}

	_, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err == nil {
		t.Error("expected error when service start fails")
	}
}

// --- processHostServices: no command available → error -----------------------

func TestProcessHostServicesMissingCommand(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			// Has commands struct but no actual up command
			"broken": {
				Commands: &models.ServiceCommands{
					Health: "curl localhost",
				},
			},
		},
	}

	_, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err == nil {
		t.Error("expected error when no command available")
	}
}

// --- processHostServices: with stop command -----------------------------------

func TestProcessHostServicesWithStopCommand(t *testing.T) {
	initI18nUp(t)

	var gotInfo *host.ProcessInfo
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *models.Deps,
				n string, svc models.Service, pd string,
			) (*host.ProcessInfo, error) {
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {
				Source: models.SourceConfig{Command: "go run ."},
				Commands: &models.ServiceCommands{
					Dev: &models.EnvironmentCommands{Down: "pkill api"},
				},
			},
		},
	}

	got, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotInfo = got["api"]
	if gotInfo == nil {
		t.Fatal("expected api in results")
	}
	if gotInfo.StopCommand != "pkill api" {
		t.Errorf("StopCommand = %q, want 'pkill api'", gotInfo.StopCommand)
	}
}

// --- processHostServices: fallback to root project commands -------------------

func TestProcessHostServicesFallbackToRootCommands(t *testing.T) {
	initI18nUp(t)

	startCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *models.Deps,
				n string, svc models.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled = true
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{
			Name: "p",
			Commands: &models.ProjectCommands{
				Dev: &models.EnvironmentCommands{Up: "make dev"},
			},
		},
		Services: map[string]models.Service{
			"app": {
				Commands: &models.ServiceCommands{}, // Empty, no up
			},
		},
	}

	_, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !startCalled {
		t.Error("should start using root project commands as fallback")
	}
}

// --- processHostServices: prod mode ------------------------------------------

func TestProcessHostServicesProdMode(t *testing.T) {
	initI18nUp(t)

	startCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return nil, "", stderrors.New("nope")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *models.Deps,
				n string, svc models.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled = true
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"api": {
				Source: models.SourceConfig{Command: "node dist/index.js"},
			},
		},
	}

	got, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !startCalled {
		t.Error("should start host service with source.command")
	}
	if _, ok := got["api"]; !ok {
		t.Error("expected api in result")
	}
}

// --- processHostServices: git service with .raioz.json commands fallback -----

func TestProcessHostServicesGitServiceWithConfig(t *testing.T) {
	initI18nUp(t)

	startCalled := false
	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc models.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*models.Deps, string, error) {
				return &models.Deps{
					Project: models.Project{
						Commands: &models.ProjectCommands{
							Dev: &models.EnvironmentCommands{Up: "npm start"},
						},
					},
				}, "raioz.yaml", nil
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *models.Deps,
				n string, svc models.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled = true
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &models.Deps{
		Project: models.Project{Name: "p"},
		Services: map[string]models.Service{
			"frontend": {
				Source:   models.SourceConfig{Kind: "git"},
				Commands: &models.ServiceCommands{}, // empty, will fallback to service config
			},
		},
	}

	_, err := uc.processHostServices(
		context.Background(), deps, &workspace.Workspace{Root: "/t"}, "/proj",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !startCalled {
		t.Error("should start using service's .raioz.json commands")
	}
}
