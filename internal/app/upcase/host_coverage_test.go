package upcase

import (
	"context"
	stderrors "errors"
	"testing"

	"raioz/internal/config"
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled++
				return &host.ProcessInfo{PID: 100, Command: "npm start"}, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"web": {
				Commands: &config.ServiceCommands{
					Dev: &config.EnvironmentCommands{Up: "npm start"},
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
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("nope")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				return nil, stderrors.New("start failed")
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"worker": {Source: config.SourceConfig{Command: "node worker.js"}},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			// Has commands struct but no actual up command
			"broken": {
				Commands: &config.ServiceCommands{
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
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Command: "go run ."},
				Commands: &config.ServiceCommands{
					Dev: &config.EnvironmentCommands{Down: "pkill api"},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("no config")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled = true
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{
			Name: "p",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Up: "make dev"},
			},
		},
		Services: map[string]config.Service{
			"app": {
				Commands: &config.ServiceCommands{}, // Empty, no up
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
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return nil, "", stderrors.New("nope")
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled = true
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Command: "node dist/index.js"},
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
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			FindServiceConfigFunc: func(p string) (*config.Deps, string, error) {
				return &config.Deps{
					Project: config.Project{
						Commands: &config.ProjectCommands{
							Dev: &config.EnvironmentCommands{Up: "npm start"},
						},
					},
				}, "raioz.yaml", nil
			},
		},
		HostRunner: &mocks.MockHostRunner{
			StartServiceFunc: func(
				ctx context.Context, ws *workspace.Workspace, d *config.Deps,
				n string, svc config.Service, pd string,
			) (*host.ProcessInfo, error) {
				startCalled = true
				return &host.ProcessInfo{PID: 100, Command: svc.Source.Command}, nil
			},
		},
	})

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"frontend": {
				Source:   config.SourceConfig{Kind: "git"},
				Commands: &config.ServiceCommands{}, // empty, will fallback to service config
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
