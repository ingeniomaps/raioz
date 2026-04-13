package app

import (
	"context"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- status_output.go -------------------------------------------------------

func TestStatusUseCase_outputJSON(t *testing.T) {
	initI18nForTest(t)
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{},
	})
	servicesInfo := map[string]*interfaces.ServiceInfo{
		"api": {Status: "running"},
	}
	stateDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Network: config.NetworkConfig{Name: "net"},
	}
	if err := uc.outputJSON(servicesInfo, []string{"disabled"}, stateDeps, "ws"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Empty disabled / no workspace
	if err := uc.outputJSON(servicesInfo, nil, stateDeps, ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStatusUseCase_outputHumanReadable(t *testing.T) {
	initI18nForTest(t)
	formatCalled := false
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			FormatStatusTableFunc: func(svc map[string]*interfaces.ServiceInfo, jsonOut bool) error {
				formatCalled = true
				return nil
			},
		},
	})
	servicesInfo := map[string]*interfaces.ServiceInfo{
		"api": {Status: "running"},
	}
	stateDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Network: config.NetworkConfig{Name: "net"},
	}
	if err := uc.outputHumanReadable(servicesInfo, []string{"disabled"}, stateDeps, "ws"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !formatCalled {
		t.Error("FormatStatusTable should be called")
	}
}

func TestStatusUseCase_outputHumanReadable_Empty(t *testing.T) {
	initI18nForTest(t)
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{},
	})
	stateDeps := &config.Deps{
		Project: config.Project{Name: "p"},
		Network: config.NetworkConfig{Name: "net"},
	}
	if err := uc.outputHumanReadable(map[string]*interfaces.ServiceInfo{}, nil, stateDeps, ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- status_orchestrated.go -------------------------------------------------

func TestInfraImageRef(t *testing.T) {
	tests := []struct {
		name  string
		entry config.InfraEntry
		want  string
	}{
		{"nil inline", config.InfraEntry{}, ""},
		{"image only", config.InfraEntry{Inline: &config.Infra{Image: "redis"}}, "redis"},
		{"image with tag", config.InfraEntry{Inline: &config.Infra{Image: "redis", Tag: "7"}}, "redis:7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infraImageRef(tt.entry)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusUseCase_queryServiceStatus(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return map[string]string{"raioz-p-api": "running"}, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	// Should return whatever naming.Container produces for that key
	status := uc.queryServiceStatus(context.Background(), "api", deps)
	// naming.Container produces raioz-p-api (or similar)
	if status == "" {
		t.Error("expected non-empty status")
	}
}

func TestStatusUseCase_queryServiceStatus_Error(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return nil, context.Canceled
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	got := uc.queryServiceStatus(context.Background(), "api", deps)
	if got != "unknown" {
		t.Errorf("expected unknown, got %q", got)
	}
}

func TestStatusUseCase_showOrchestratedStatus_ConfigLoadError(t *testing.T) {
	initI18nForTest(t)
	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, context.DeadlineExceeded
			},
		},
	})
	tmpDir := t.TempDir()
	err := uc.showOrchestratedStatus(context.Background(), StatusOptions{ConfigPath: tmpDir + "/raioz.yaml"})
	if err == nil {
		t.Error("expected error on config load failure")
	}
}

func TestStatusUseCase_showOrchestratedStatus_Empty(t *testing.T) {
	initI18nForTest(t)
	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project:  config.Project{Name: "p"},
					Services: map[string]config.Service{},
					Infra:    map[string]config.InfraEntry{},
				}, nil, nil
			},
		},
	})
	tmpDir := t.TempDir()
	err := uc.showOrchestratedStatus(context.Background(), StatusOptions{ConfigPath: tmpDir + "/raioz.yaml"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStatusUseCase_showOrchestratedStatus_WithServicesAndInfra(t *testing.T) {
	initI18nForTest(t)
	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project:   config.Project{Name: "p"},
					Workspace: "ws",
					Services: map[string]config.Service{
						"api": {Source: config.SourceConfig{Path: "."}},
					},
					Infra: map[string]config.InfraEntry{
						"redis": {Inline: &config.Infra{Image: "redis:7"}},
					},
				}, nil, nil
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return map[string]string{}, nil
			},
		},
	})
	tmpDir := t.TempDir()
	err := uc.showOrchestratedStatus(context.Background(), StatusOptions{ConfigPath: tmpDir + "/raioz.yaml"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- status_host.go ---------------------------------------------------------

func TestStatusUseCase_getHostServiceInfo_Stopped(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/tmp/x"
			},
		},
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(p, c, e string) string { return "" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return nil, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	svc := config.Service{}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "svc", svc,
		deps, map[string]*host.ProcessInfo{},
	)
	if info == nil || info.Status != "stopped" {
		t.Errorf("expected stopped, got %+v", info)
	}
}

func TestStatusUseCase_getHostServiceInfo_ComposeRunning(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/tmp/x"
			},
		},
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(p, c, e string) string { return "/tmp/x/docker-compose.yml" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return map[string]string{"svc1": "running"}, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	svc := config.Service{}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "svc", svc,
		deps, map[string]*host.ProcessInfo{},
	)
	if info == nil || info.Status != "running" {
		t.Errorf("expected running, got %+v", info)
	}
}

func TestStatusUseCase_getHostServiceInfo_ProcessInfoComposePath(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/tmp/x"
			},
		},
		HostRunner: &mocks.MockHostRunner{},
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return map[string]string{}, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	svc := config.Service{}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "svc", svc, deps,
		map[string]*host.ProcessInfo{
			"svc": {PID: 1, ComposePath: "/tmp/custom-compose.yml"},
		},
	)
	if info == nil {
		t.Fatal("expected non-nil")
	}
}

func TestStatusUseCase_getHostServiceInfo_ProcessAlive(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/tmp/x"
			},
		},
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(p, c, e string) string { return "" },
			IsServiceRunningFunc: func(pid int) (bool, error) {
				return true, nil
			},
		},
		DockerRunner: &mocks.MockDockerRunner{
			GetServicesStatusWithContextFunc: func(ctx context.Context, cp string) (map[string]string, error) {
				return nil, nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	svc := config.Service{}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "svc", svc, deps,
		map[string]*host.ProcessInfo{
			"svc": {PID: 1234},
		},
	)
	if info == nil || info.Status != "running" {
		t.Errorf("expected running, got %+v", info)
	}
}
