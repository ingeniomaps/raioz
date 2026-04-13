package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// --- down_proxy.go ----------------------------------------------------------

func TestDownUseCase_stopProxy_NilManager(t *testing.T) {
	uc := NewDownUseCase(&Dependencies{})
	uc.stopProxy(context.Background(), DownOptions{})
}

func TestDownUseCase_cleanLocalState_EmptyPath(t *testing.T) {
	uc := NewDownUseCase(&Dependencies{})
	uc.cleanLocalState(context.Background(), DownOptions{ConfigPath: ""})
}

func TestDownUseCase_cleanLocalState_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a dummy state file
	statePath := filepath.Join(tmpDir, ".raioz.state.json")
	_ = os.WriteFile(statePath, []byte(`{}`), 0644)
	uc := NewDownUseCase(&Dependencies{})
	uc.cleanLocalState(context.Background(), DownOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
	})
}

// --- down_docker.go: handleProjectComposeDown ------------------------------

func TestDownUseCase_handleProjectComposeDown_FromState(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	_ = os.WriteFile(composePath, []byte("services: {}"), 0644)

	downCalled := false
	uc := NewDownUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			DownWithContextFunc: func(ctx context.Context, cp string) error {
				downCalled = true
				return nil
			},
		},
	})
	stateDeps := &config.Deps{
		Project:            config.Project{Name: "p"},
		ProjectComposePath: composePath,
	}
	uc.handleProjectComposeDown(context.Background(), stateDeps, DownOptions{})
	if !downCalled {
		t.Error("expected DownWithContext to be called")
	}
}

func TestDownUseCase_handleProjectComposeDown_FromConfigPath(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	_ = os.WriteFile(composePath, []byte("services: {}"), 0644)

	downCalled := false
	uc := NewDownUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			DownWithContextFunc: func(ctx context.Context, cp string) error {
				downCalled = true
				return nil
			},
		},
	})
	stateDeps := &config.Deps{Project: config.Project{Name: "p"}}
	uc.handleProjectComposeDown(context.Background(), stateDeps, DownOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
	})
	if !downCalled {
		t.Error("expected DownWithContext to be called via config path detection")
	}
}

func TestDownUseCase_handleProjectComposeDown_NoPath(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{})
	stateDeps := &config.Deps{Project: config.Project{Name: "p"}}
	uc.handleProjectComposeDown(context.Background(), stateDeps, DownOptions{})
}

// --- down_docker.go: executeProjectDownCommand, runDownCommand --------------

func TestDownUseCase_executeProjectDownCommand_NoCommands(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{})
	stateDeps := &config.Deps{Project: config.Project{Name: "p", Commands: nil}}
	uc.executeProjectDownCommand(
		context.Background(), stateDeps, &workspace.Workspace{Root: "/tmp"}, DownOptions{}, "ws",
	)
}

func TestDownUseCase_executeProjectDownCommand_DownOnly(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	uc := NewDownUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetRootFunc: func(ws *workspace.Workspace) string { return tmpDir },
		},
	})
	stateDeps := &config.Deps{
		Project: config.Project{
			Name: "p",
			Commands: &config.ProjectCommands{
				Down: "true", // Harmless shell command
			},
		},
	}
	uc.executeProjectDownCommand(
		context.Background(), stateDeps, &workspace.Workspace{Root: tmpDir},
		DownOptions{ConfigPath: filepath.Join(tmpDir, "raioz.yaml")}, "ws",
	)
}

func TestDownUseCase_executeProjectDownCommand_DevMode(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	uc := NewDownUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetRootFunc: func(ws *workspace.Workspace) string { return tmpDir },
		},
	})
	stateDeps := &config.Deps{
		Project: config.Project{
			Name: "p",
			Commands: &config.ProjectCommands{
				Dev: &config.EnvironmentCommands{Down: "true"},
			},
		},
	}
	uc.executeProjectDownCommand(
		context.Background(), stateDeps, &workspace.Workspace{Root: tmpDir}, DownOptions{}, "ws",
	)
}

func TestDownUseCase_executeProjectDownCommand_CommandOnlyProject(t *testing.T) {
	initI18nForTest(t)
	stopped := false
	uc := NewDownUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			NormalizeContainerNameFunc: func(ws, svc, proj string, has bool) (string, error) {
				return "raioz-" + proj, nil
			},
			StopContainerWithContextFunc: func(ctx context.Context, name string) error {
				stopped = true
				return nil
			},
		},
	})
	stateDeps := &config.Deps{
		Project: config.Project{
			Name: "p",
			Commands: &config.ProjectCommands{
				Up: "echo start", // no Down, has Up
			},
		},
		Services: map[string]config.Service{},
	}
	uc.executeProjectDownCommand(
		context.Background(), stateDeps, &workspace.Workspace{Root: "/tmp"}, DownOptions{}, "ws",
	)
	if !stopped {
		t.Error("expected StopContainerWithContext to be called for command-only project")
	}
}

// --- down_docker.go: stopCommandOnlyProjectContainers ----------------------

func TestDownUseCase_stopCommandOnlyProjectContainers(t *testing.T) {
	initI18nForTest(t)
	stopCalls := 0
	uc := NewDownUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			NormalizeContainerNameFunc: func(ws, svc, proj string, has bool) (string, error) {
				return "norm-" + proj, nil
			},
			StopContainerWithContextFunc: func(ctx context.Context, name string) error {
				stopCalls++
				return nil
			},
		},
	})
	stateDeps := &config.Deps{
		Project: config.Project{Name: "myproj"},
	}
	uc.stopCommandOnlyProjectContainers(context.Background(), stateDeps, "ws")
	if stopCalls == 0 {
		t.Error("expected at least one stop call")
	}
}

// --- down_docker.go: cleanupDockerResources --------------------------------

func TestDownUseCase_cleanupDockerResources(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			CleanUnusedImagesWithContextFunc: func(ctx context.Context, dryRun bool) ([]string, error) {
				return []string{"image1"}, nil
			},
			CleanUnusedVolumesWithContextFunc: func(ctx context.Context, dryRun, force bool) ([]string, error) {
				return []string{"vol1"}, nil
			},
		},
	})
	uc.cleanupDockerResources(context.Background())
}

func TestDownUseCase_cleanupDockerResources_Errors(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			CleanUnusedImagesWithContextFunc: func(ctx context.Context, dryRun bool) ([]string, error) {
				return nil, context.Canceled
			},
			CleanUnusedVolumesWithContextFunc: func(ctx context.Context, dryRun, force bool) ([]string, error) {
				return nil, context.Canceled
			},
		},
	})
	uc.cleanupDockerResources(context.Background())
}

// --- down_host.go: stopHostProcesses ---------------------------------------

func TestDownUseCase_stopHostProcesses_None(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			LoadProcessesStateFunc: func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
				return map[string]*host.ProcessInfo{}, nil
			},
		},
	})
	got := uc.stopHostProcesses(context.Background(), &workspace.Workspace{Root: "/tmp"}, DownOptions{})
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestDownUseCase_stopHostProcesses_LoadError(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			LoadProcessesStateFunc: func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
				return nil, context.DeadlineExceeded
			},
		},
	})
	got := uc.stopHostProcesses(context.Background(), &workspace.Workspace{Root: "/tmp"}, DownOptions{})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestDownUseCase_stopHostProcesses_WithProcesses(t *testing.T) {
	initI18nForTest(t)
	stopCalled := false
	uc := NewDownUseCase(&Dependencies{
		HostRunner: &mocks.MockHostRunner{
			LoadProcessesStateFunc: func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
				return map[string]*host.ProcessInfo{
					"svc": {PID: 1234, StopCommand: "echo stop"},
				}, nil
			},
			StopServiceWithCommandAndPathFunc: func(ctx context.Context, pid int, cmd, path string) error {
				stopCalled = true
				return nil
			},
			RemoveProcessesStateFunc: func(ws *workspace.Workspace) error { return nil },
		},
	})
	uc.stopHostProcesses(context.Background(), &workspace.Workspace{Root: "/tmp"}, DownOptions{})
	if !stopCalled {
		t.Error("expected stop to be called")
	}
}

// --- down_host.go: resolveHostStopCommand ----------------------------------

func TestDownUseCase_resolveHostStopCommand_FromProcessInfo(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{})
	pi := host.ProcessInfo{PID: 1, StopCommand: "echo stop"}
	stopCmd, path := uc.resolveHostStopCommand(
		context.Background(), "svc", pi, nil, &workspace.Workspace{Root: "/tmp"},
	)
	if stopCmd != "echo stop" {
		t.Errorf("expected 'echo stop', got %q", stopCmd)
	}
	_ = path
}

func TestDownUseCase_resolveHostStopCommand_FromConfig(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/svc-path"
			},
		},
	})
	pi := host.ProcessInfo{PID: 1}
	currentDeps := &config.Deps{
		Services: map[string]config.Service{
			"svc": {
				Source: config.SourceConfig{Kind: "git"},
				Commands: &config.ServiceCommands{
					Down: "stop.sh",
				},
			},
		},
	}
	stopCmd, _ := uc.resolveHostStopCommand(
		context.Background(), "svc", pi, currentDeps, &workspace.Workspace{Root: "/tmp"},
	)
	if stopCmd != "stop.sh" {
		t.Errorf("expected 'stop.sh', got %q", stopCmd)
	}
}

func TestDownUseCase_resolveHostStopCommand_DetectCompose(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	_ = os.WriteFile(composePath, []byte("services: {}"), 0644)

	uc := NewDownUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return tmpDir
			},
		},
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(p, c, e string) string { return composePath },
		},
	})
	pi := host.ProcessInfo{PID: 1}
	currentDeps := &config.Deps{
		Services: map[string]config.Service{
			"svc": {Source: config.SourceConfig{Kind: "git"}},
		},
	}
	stopCmd, _ := uc.resolveHostStopCommand(
		context.Background(), "svc", pi, currentDeps, &workspace.Workspace{Root: "/tmp"},
	)
	if stopCmd == "" {
		t.Error("expected docker compose down command")
	}
}

// --- down_host.go: detectHostComposePath -----------------------------------

func TestDownUseCase_detectHostComposePath_FromProcessInfo(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	_ = os.WriteFile(composePath, []byte("services: {}"), 0644)

	uc := NewDownUseCase(&Dependencies{})
	pi := host.ProcessInfo{ComposePath: composePath}
	got := uc.detectHostComposePath(
		context.Background(), "svc", pi, nil, &workspace.Workspace{Root: "/tmp"}, "",
	)
	if got != composePath {
		t.Errorf("expected %q, got %q", composePath, got)
	}
}

func TestDownUseCase_detectHostComposePath_NilDeps(t *testing.T) {
	uc := NewDownUseCase(&Dependencies{})
	got := uc.detectHostComposePath(
		context.Background(), "svc", host.ProcessInfo{}, nil, &workspace.Workspace{Root: "/tmp"}, "",
	)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDownUseCase_detectHostComposePath_ServiceNotFound(t *testing.T) {
	uc := NewDownUseCase(&Dependencies{})
	deps := &config.Deps{Services: map[string]config.Service{}}
	got := uc.detectHostComposePath(
		context.Background(), "svc", host.ProcessInfo{}, deps, &workspace.Workspace{Root: "/tmp"}, "",
	)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDownUseCase_detectHostComposePath_NoServicePath(t *testing.T) {
	uc := NewDownUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return ""
			},
		},
	})
	deps := &config.Deps{
		Services: map[string]config.Service{
			"svc": {},
		},
	}
	got := uc.detectHostComposePath(
		context.Background(), "svc", host.ProcessInfo{}, deps, &workspace.Workspace{Root: "/tmp"}, "",
	)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- down_docker.go: handleNetworkAndVolumes -------------------------------

func TestDownUseCase_handleNetworkAndVolumes(t *testing.T) {
	initI18nForTest(t)
	uc := NewDownUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetBaseDirFromWorkspaceFunc: func(ws *workspace.Workspace) string { return "/tmp" },
		},
		DockerRunner: &mocks.MockDockerRunner{
			IsNetworkInUseWithContextFunc: func(ctx context.Context, name string) (bool, error) {
				return false, nil
			},
			GetNetworkProjectsFunc: func(name, dir string) ([]string, error) {
				return []string{"other"}, nil
			},
			ExtractNamedVolumesFunc: func(vols []string) ([]string, error) {
				return []string{"v1"}, nil
			},
			GetVolumeProjectsFunc: func(vol, dir string) ([]string, error) {
				return []string{"other"}, nil
			},
		},
	})
	stateDeps := &config.Deps{
		Project: config.Project{Name: "me"},
		Network: config.NetworkConfig{Name: "net"},
		Services: map[string]config.Service{
			"a": {Docker: &config.DockerConfig{Volumes: []string{"x:/y"}}},
		},
		Infra: map[string]config.InfraEntry{},
	}
	remaining, inUse := uc.handleNetworkAndVolumes(
		context.Background(), stateDeps, &workspace.Workspace{Root: "/tmp"}, "me", "ws",
	)
	if remaining != 1 {
		t.Errorf("expected 1 remaining project, got %d", remaining)
	}
	_ = inUse
}

// Avoid compile-time unused import in case interfaces or workspace drop
var _ = interfaces.ProxyRoute{}
