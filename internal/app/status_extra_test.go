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
			GetContainerStatusByNameFunc: func(_ context.Context, name string) (string, error) {
				// naming.Container(p, api) → <prefix>-p-api
				if name == "" {
					t.Error("empty container name")
				}
				return "running", nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	got := uc.queryServiceStatus(context.Background(), "api", deps)
	if got != "running" {
		t.Errorf("expected running, got %q", got)
	}
}

func TestStatusUseCase_queryServiceStatus_NotFound(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(_ context.Context, _ string) (string, error) {
				return "", nil
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	got := uc.queryServiceStatus(context.Background(), "api", deps)
	if got != "stopped" {
		t.Errorf("expected stopped, got %q", got)
	}
}

func TestStatusUseCase_queryServiceStatus_Error(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(_ context.Context, _ string) (string, error) {
				return "", context.Canceled
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

// --- Issue 009: dep status fallback by labels --------------------------------

// When the canonical container name lookup misses (typical for compose-mode
// deps where container_name is dictated by the user's compose), status must
// fall back to a label-based lookup so `raioz status` doesn't lie.
func TestQueryDepStatus_FallsBackToLabelsWhenCanonicalNameMissing(t *testing.T) {
	calls := struct {
		statusCalls []string
		findCalls   int
	}{}
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(ctx context.Context, name string) (string, error) {
				calls.statusCalls = append(calls.statusCalls, name)
				switch name {
				case "raioz-yemdiou-postgres": // canonical, missing
					return "", nil
				case "yemdiou-postgres": // real container the user's compose created
					return "running", nil
				}
				return "", nil
			},
			FindManagedContainerByServiceFunc: func(ctx context.Context, project, service string) string {
				calls.findCalls++
				if project == "yemdiou" && service == "postgres" {
					return "yemdiou-postgres"
				}
				return ""
			},
		},
	})

	deps := &config.Deps{Project: config.Project{Name: "yemdiou"}}
	entry := config.InfraEntry{Inline: &config.Infra{Image: "postgres:16"}}
	got := uc.queryDepStatus(context.Background(), "postgres", entry, deps)
	if got != "running" {
		t.Errorf("queryDepStatus = %q, want %q", got, "running")
	}
	if calls.findCalls != 1 {
		t.Errorf("FindManagedContainerByService calls = %d, want 1", calls.findCalls)
	}
	if len(calls.statusCalls) != 2 {
		t.Errorf("expected two GetContainerStatusByName calls (canonical + real), got %v", calls.statusCalls)
	}
}

// When the canonical lookup already finds the container, no label fallback
// must run — the heuristic is opt-in only when the cheaper path missed.
func TestQueryDepStatus_NoFallbackWhenCanonicalHits(t *testing.T) {
	findCalls := 0
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(ctx context.Context, name string) (string, error) {
				return "running", nil
			},
			FindManagedContainerByServiceFunc: func(ctx context.Context, project, service string) string {
				findCalls++
				return "should-not-be-called"
			},
		},
	})

	deps := &config.Deps{Project: config.Project{Name: "p"}}
	entry := config.InfraEntry{Inline: &config.Infra{Image: "redis:7"}}
	got := uc.queryDepStatus(context.Background(), "redis", entry, deps)
	if got != "running" {
		t.Errorf("queryDepStatus = %q, want running", got)
	}
	if findCalls != 0 {
		t.Errorf("label fallback ran when canonical lookup succeeded (calls=%d)", findCalls)
	}
}

// If nothing matches the canonical name AND the label search returns "",
// status remains "stopped" — we must not pretend a missing dep is running.
func TestQueryDepStatus_StillStoppedWhenNoMatch(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(ctx context.Context, name string) (string, error) {
				return "", nil
			},
			FindManagedContainerByServiceFunc: func(ctx context.Context, project, service string) string {
				return ""
			},
		},
	})
	deps := &config.Deps{Project: config.Project{Name: "p"}}
	entry := config.InfraEntry{Inline: &config.Infra{Image: "x"}}
	if got := uc.queryDepStatus(context.Background(), "x", entry, deps); got != "stopped" {
		t.Errorf("queryDepStatus = %q, want stopped", got)
	}
}

// --- Issue 010: proxy.target as source of truth for host status -------------

// When the user declares `services.<name>.proxy.target: <container>` and that
// container is running, status must report "running" regardless of what the
// PID-alive / compose-autodetect heuristics say. The classic break is a
// host launcher (`make dev-docker`) that exits 0 after `compose up -d`:
// PID is gone, autodetect targets the wrong compose file, but the container
// is happily up.
func TestGetHostServiceInfo_ProxyTargetRunningWins(t *testing.T) {
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
			GetContainerStatusByNameFunc: func(ctx context.Context, name string) (string, error) {
				if name == "hypixo-accounts" {
					return "running", nil
				}
				return "", nil
			},
		},
	})

	deps := &config.Deps{Project: config.Project{Name: "accounts"}, Workspace: "hypixo"}
	svc := config.Service{
		Source: config.SourceConfig{Command: "make dev-docker"},
		ProxyOverride: &config.ServiceProxyOverride{
			Target: "hypixo-accounts",
			Port:   4002,
		},
	}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "accounts", svc,
		deps, map[string]*host.ProcessInfo{}, // no PID — launcher already exited
	)
	if info.Status != "running" {
		t.Errorf("expected running (proxy.target alive), got %+v", info)
	}
}

// Counterpart: when the named container is not running, we surface that
// directly without falling back to the (lying) PID/compose path.
func TestGetHostServiceInfo_ProxyTargetExitedWins(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/tmp/x"
			},
		},
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(p, c, e string) string { return "" },
			IsServiceRunningFunc:  func(pid int) (bool, error) { return true, nil },
		},
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(ctx context.Context, name string) (string, error) {
				if name == "hypixo-accounts" {
					return "exited", nil
				}
				return "", nil
			},
		},
	})

	deps := &config.Deps{Project: config.Project{Name: "accounts"}}
	svc := config.Service{
		Source:        config.SourceConfig{Command: "make dev-docker"},
		ProxyOverride: &config.ServiceProxyOverride{Target: "hypixo-accounts"},
	}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "accounts", svc,
		deps, map[string]*host.ProcessInfo{
			// PID is alive, but the *container* is exited — the container wins.
			"accounts": {PID: 4242},
		},
	)
	if info.Status != "stopped" {
		t.Errorf("expected stopped (proxy.target exited), got %+v", info)
	}
}

// When proxy.target points to a container that does not exist yet (typical
// before the first `up`), we must NOT short-circuit — the existing
// PID/compose path still has a chance to decide.
func TestGetHostServiceInfo_ProxyTargetMissingFallsThrough(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(ws *workspace.Workspace, n string, svc config.Service) string {
				return "/tmp/x"
			},
		},
		HostRunner: &mocks.MockHostRunner{
			DetectComposePathFunc: func(p, c, e string) string { return "" },
			IsServiceRunningFunc:  func(pid int) (bool, error) { return true, nil },
		},
		DockerRunner: &mocks.MockDockerRunner{
			GetContainerStatusByNameFunc: func(ctx context.Context, name string) (string, error) {
				return "", nil // container does not exist
			},
		},
	})

	deps := &config.Deps{Project: config.Project{Name: "accounts"}}
	svc := config.Service{
		Source:        config.SourceConfig{Command: "make dev-docker"},
		ProxyOverride: &config.ServiceProxyOverride{Target: "not-yet-built"},
	}
	info := uc.getHostServiceInfo(
		context.Background(), &workspace.Workspace{Root: "/tmp"}, "accounts", svc,
		deps, map[string]*host.ProcessInfo{
			"accounts": {PID: 4242},
		},
	)
	// PID-alive fallback should kick in.
	if info.Status != "running" {
		t.Errorf("expected fallback PID detection to mark running, got %+v", info)
	}
}
