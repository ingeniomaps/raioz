package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fmt"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

// --- StatusYAML ---

func TestStatusUseCase_StatusYAML_Basic(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	// Create a local state file to exercise host PID logic
	ls := &state.LocalState{HostPIDs: map[string]int{"api": os.Getpid()}}
	_ = state.SaveLocalState(tmpDir, ls)

	uc := NewStatusUseCase(&Dependencies{
		ProxyManager: nil, // no proxy
	})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &config.Deps{
			Project: config.Project{Name: "test"},
			Services: map[string]config.Service{
				"api": {Source: config.SourceConfig{Path: "."}},
			},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Image: "redis", Tag: "7"}},
			},
		},
	}
	// This will fail to reach docker containers (no Docker running), but
	// should not panic. ContainerStatus/ContainerStats will return defaults.
	err := uc.StatusYAML(context.Background(), proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStatusUseCase_StatusYAML_WithProxy(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	proxyStatusCalled := false
	mockProxy := &mockProxyManager{
		statusFunc: func(ctx context.Context) (bool, error) {
			proxyStatusCalled = true
			return true, nil
		},
	}
	uc := NewStatusUseCase(&Dependencies{
		ProxyManager: mockProxy,
	})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &config.Deps{
			Project:  config.Project{Name: "test"},
			Proxy:    true,
			Services: map[string]config.Service{},
			Infra:    map[string]config.InfraEntry{},
		},
	}
	err := uc.StatusYAML(context.Background(), proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proxyStatusCalled {
		t.Error("expected ProxyManager.Status to be called")
	}
}

func TestStatusUseCase_StatusYAML_DevOverride(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	ls := &state.LocalState{
		HostPIDs:     map[string]int{},
		DevOverrides: map[string]state.DevOverride{"api": {LocalPath: "/tmp/local", OriginalImage: "api:latest"}},
	}
	_ = state.SaveLocalState(tmpDir, ls)

	uc := NewStatusUseCase(&Dependencies{})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &config.Deps{
			Project: config.Project{Name: "test"},
			Services: map[string]config.Service{
				"api": {Source: config.SourceConfig{Path: "."}},
			},
			Infra: map[string]config.InfraEntry{},
		},
	}
	err := uc.StatusYAML(context.Background(), proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LogsYAML ---

func TestLogsYAML_WithServices(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Services: map[string]config.Service{
				"api": {Source: config.SourceConfig{Path: "/tmp/api"}},
			},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Image: "redis:7"}},
			},
		},
	}
	// With specific service names (host service — will attempt to read log file)
	err := LogsYAML(context.Background(), proj, []string{"api"}, false, 10)
	// Log file doesn't exist, but it shouldn't be fatal
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogsYAML_AllServices(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Services: map[string]config.Service{
				"api": {},
			},
			Infra: map[string]config.InfraEntry{
				"redis": {},
			},
		},
	}
	// No services specified - should gather all
	// Docker containers will fail (no Docker running) but host log warning is ok
	_ = LogsYAML(context.Background(), proj, nil, false, 0)
}

// --- showHostLogs ---

func TestShowHostLogs_NoFile(t *testing.T) {
	err := showHostLogs(context.Background(), "/nonexistent/path/log.txt", false, 10)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestShowHostLogs_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	_ = os.WriteFile(logFile, []byte("line1\nline2\nline3\n"), 0644)

	err := showHostLogs(context.Background(), logFile, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowHostLogs_WithTail(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	_ = os.WriteFile(logFile, []byte("line1\nline2\nline3\n"), 0644)

	err := showHostLogs(context.Background(), logFile, false, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RestartYAML ---

func TestRestartYAML_WithServices(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps:        &config.Deps{},
	}
	// Docker won't be running but the function should not panic
	err := RestartYAML(context.Background(), proj, []string{"api"})
	// Will fail because Docker is not available, but we exercise the code path
	_ = err
}

// --- ExecYAML ---

func TestExecYAML_StoppedHostService(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Services: map[string]config.Service{
				"api": {Source: config.SourceConfig{Path: tmpDir}},
			},
		},
	}
	// Container won't exist (Docker not running), so it'll be "stopped"
	// Then it'll try to exec in the service directory
	err := ExecYAML(context.Background(), proj, "api", []string{"true"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecYAML_StoppedNoPath(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Services: map[string]config.Service{
				"api": {},
			},
		},
	}
	err := ExecYAML(context.Background(), proj, "api", []string{"echo", "hello"}, false)
	if err == nil {
		t.Fatal("expected error for stopped service without path")
	}
}

func TestExecYAML_NoServiceFound(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Services: map[string]config.Service{},
		},
	}
	err := ExecYAML(context.Background(), proj, "missing", nil, false)
	if err == nil {
		t.Fatal("expected error for missing service")
	}
}

// --- CheckYAML with various scenarios ---

func TestCheckYAML_ServiceWithRuntime(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	// Create a go.mod to make detect.Detect find a runtime
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.22\n"), 0644)

	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Project: config.Project{Name: "test"},
			Services: map[string]config.Service{
				"api": {Source: config.SourceConfig{Path: tmpDir}},
			},
			Infra: map[string]config.InfraEntry{},
		},
	}
	err := CheckYAML(proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckYAML_NoRuntimeNoPath(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Project: config.Project{Name: "test"},
			Services: map[string]config.Service{
				"api": {}, // no path, no command, no compose
			},
			Infra: map[string]config.InfraEntry{},
		},
	}
	err := CheckYAML(proj)
	if err == nil {
		t.Fatal("expected error for service with no runtime")
	}
}

// --- Status legacy path: workspace resolve error ---

func TestStatusUseCase_Execute_WorkspaceResolveError(t *testing.T) {
	initI18nForTest(t)
	deps := &Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project:       config.Project{Name: "proj"},
					SchemaVersion: "1.0",
					Services:      map[string]config.Service{},
					Infra:         map[string]config.InfraEntry{},
				}, nil, nil
			},
		},
		Workspace: &mocks.MockWorkspaceManager{
			ResolveFunc: func(name string) (*interfaces.Workspace, error) {
				return nil, fmt.Errorf("fail")
			},
		},
	}
	uc := NewStatusUseCase(deps)
	err := uc.Execute(context.Background(), StatusOptions{ConfigPath: "raioz.json"})
	if err == nil {
		t.Fatal("expected error for workspace resolve failure")
	}
}

// mockProxyManager is a minimal mock for ProxyManager, used here since
// no mock exists in the mocks package for this interface.
type mockProxyManager struct {
	statusFunc            func(ctx context.Context) (bool, error)
	stopFunc              func(ctx context.Context) error
	reloadFunc            func(ctx context.Context) error
	remainingProjectsFunc func() int

	removeProjectRoutesCalled bool
}

func (m *mockProxyManager) Start(ctx context.Context, networkName string) error { return nil }
func (m *mockProxyManager) Stop(ctx context.Context) error {
	if m.stopFunc != nil {
		return m.stopFunc(ctx)
	}
	return nil
}
func (m *mockProxyManager) AddRoute(ctx context.Context, route interfaces.ProxyRoute) error {
	return nil
}
func (m *mockProxyManager) RemoveRoute(ctx context.Context, serviceName string) error {
	return nil
}
func (m *mockProxyManager) GetURL(serviceName string) string { return "" }
func (m *mockProxyManager) Reload(ctx context.Context) error {
	if m.reloadFunc != nil {
		return m.reloadFunc(ctx)
	}
	return nil
}
func (m *mockProxyManager) Status(ctx context.Context) (bool, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return false, nil
}
func (m *mockProxyManager) SetDomain(domain string)       {}
func (m *mockProxyManager) SetTLSMode(mode string)        {}
func (m *mockProxyManager) SetBindHost(host string)       {}
func (m *mockProxyManager) SetProjectName(name string)    {}
func (m *mockProxyManager) SetNetworkSubnet(cidr string)  {}
func (m *mockProxyManager) SetContainerIP(ip string)      {}
func (m *mockProxyManager) SetWorkspace(name string)      {}
func (m *mockProxyManager) SaveProjectRoutes() error      { return nil }
func (m *mockProxyManager) RemoveProjectRoutes() error {
	m.removeProjectRoutesCalled = true
	return nil
}
func (m *mockProxyManager) RemainingProjects() int {
	if m.remainingProjectsFunc != nil {
		return m.remainingProjectsFunc()
	}
	return 0
}
func (m *mockProxyManager) SetPublish(*bool) {}
func (m *mockProxyManager) IsPublished() bool { return true }
func (m *mockProxyManager) HostsLine() string { return "" }
