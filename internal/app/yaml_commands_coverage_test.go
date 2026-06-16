package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fmt"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

// --- StatusYAML ---

func TestStatusUseCase_StatusYAML_Basic(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	// Create a local state file to exercise host PID logic
	ls := &models.LocalState{HostPIDs: map[string]int{"api": os.Getpid()}}
	_ = state.SaveLocalState(tmpDir, ls)

	uc := NewStatusUseCase(&Dependencies{
		ProxyManager: nil, // no proxy
	})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: "."}},
			},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{Image: "redis", Tag: "7"}},
			},
		},
	}
	// This will fail to reach docker containers (no Docker running), but
	// should not panic. ContainerStatus/ContainerStats will return defaults.
	err := uc.StatusYAML(context.Background(), proj, nil)
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
		Deps: &models.Deps{
			Project:  models.Project{Name: "test"},
			Proxy:    true,
			Services: map[string]models.Service{},
			Infra:    map[string]models.InfraEntry{},
		},
	}
	err := uc.StatusYAML(context.Background(), proj, nil)
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
	ls := &models.LocalState{
		HostPIDs:     map[string]int{},
		DevOverrides: map[string]models.DevOverride{"api": {LocalPath: "/tmp/local", OriginalImage: "api:latest"}},
	}
	_ = state.SaveLocalState(tmpDir, ls)

	uc := NewStatusUseCase(&Dependencies{})
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  filepath.Join(tmpDir, "raioz.yaml"),
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: "."}},
			},
			Infra: map[string]models.InfraEntry{},
		},
	}
	err := uc.StatusYAML(context.Background(), proj, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- LogsYAML ---

func TestLogsYAML_WithServices(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &models.Deps{
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: "/tmp/api"}},
			},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{Image: "redis:7"}},
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
		Deps: &models.Deps{
			Services: map[string]models.Service{
				"api": {},
			},
			Infra: map[string]models.InfraEntry{
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
		Deps:        &models.Deps{},
	}
	// Docker won't be running but the function should not panic
	err := (&RestartUseCase{}).RestartYAML(
		context.Background(), proj, RestartOptions{Services: []string{"api"}})
	// Will fail because Docker is not available, but we exercise the code path
	_ = err
}

func TestCollectYAMLServiceNames_SortedAndStable(t *testing.T) {
	proj := &YAMLProject{Deps: &models.Deps{
		Services: map[string]models.Service{
			"web": {}, "api": {}, "worker": {},
		},
	}}
	got := collectYAMLServiceNames(proj)
	want := []string{"api", "web", "worker"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("index %d: got %q, want %q", i, got[i], n)
		}
	}
}

func TestCollectYAMLDepNames_SortedAndStable(t *testing.T) {
	proj := &YAMLProject{Deps: &models.Deps{
		Infra: map[string]models.InfraEntry{
			"redis": {}, "postgres": {}, "kafka": {},
		},
	}}
	got := collectYAMLDepNames(proj)
	want := []string{"kafka", "postgres", "redis"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("index %d: got %q, want %q", i, got[i], n)
		}
	}
}

func TestRestartYAML_AllOnEmptyProjectStillReturns(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps:        &models.Deps{},
	}
	// --all on an empty project must not be a silent no-op pretending --all
	// wasn't passed; it must short-circuit with a clear warning and exit
	// cleanly so scripts don't see a docker error.
	if err := (&RestartUseCase{}).RestartYAML(
		context.Background(), proj, RestartOptions{All: true},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ExecYAML ---

func TestExecYAML_StoppedHostService(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &models.Deps{
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: tmpDir}},
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
		Deps: &models.Deps{
			Services: map[string]models.Service{
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
		Deps: &models.Deps{
			Services: map[string]models.Service{},
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
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"api": {Source: models.SourceConfig{Path: tmpDir}},
			},
			Infra: map[string]models.InfraEntry{},
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
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"api": {}, // no path, no command, no compose
			},
			Infra: map[string]models.InfraEntry{},
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
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:       models.Project{Name: "proj"},
					SchemaVersion: "1.0",
					Services:      map[string]models.Service{},
					Infra:         map[string]models.InfraEntry{},
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
	statusFunc                 func(ctx context.Context) (bool, error)
	stopFunc                   func(ctx context.Context) error
	reloadFunc                 func(ctx context.Context) error
	remainingProjectsFunc      func() int
	listProjectsWithRoutesFunc func() []string
	removeRoutesForFunc        func(project string) error

	removeProjectRoutesCalled bool
	removedRoutesFor          []string
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
func (m *mockProxyManager) SetDomain(domain string)      {}
func (m *mockProxyManager) SetTLSMode(mode string)       {}
func (m *mockProxyManager) SetBindHost(host string)      {}
func (m *mockProxyManager) SetProjectName(name string)   {}
func (m *mockProxyManager) SetNetworkSubnet(cidr string) {}
func (m *mockProxyManager) SetContainerIP(ip string)     {}
func (m *mockProxyManager) SetWorkspace(name string)     {}
func (m *mockProxyManager) SaveProjectRoutes() error     { return nil }
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
func (m *mockProxyManager) ListProjectsWithRoutes() []string {
	if m.listProjectsWithRoutesFunc != nil {
		return m.listProjectsWithRoutesFunc()
	}
	return nil
}
func (m *mockProxyManager) RemoveRoutesFor(project string) error {
	m.removedRoutesFor = append(m.removedRoutesFor, project)
	if m.removeRoutesForFunc != nil {
		return m.removeRoutesForFunc(project)
	}
	return nil
}
func (m *mockProxyManager) SetPublish(*bool)                 {}
func (m *mockProxyManager) IsPublished() bool                { return true }
func (m *mockProxyManager) HostsLine() string                { return "" }
func (m *mockProxyManager) Configure(interfaces.ProxyConfig) {}
