package mocks

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/workspace"
)

// Compile-time check
var _ interfaces.HostRunner = (*MockHostRunner)(nil)

// MockHostRunner is a mock implementation of interfaces.HostRunner
type MockHostRunner struct {
	StartServiceFunc func(
		ctx context.Context, ws *workspace.Workspace, deps *config.Deps,
		serviceName string, svc config.Service, projectDir string,
	) (*host.ProcessInfo, error)
	StopServiceWithCommandFunc        func(ctx context.Context, pid int, stopCommand string) error
	LoadProcessesStateFunc            func(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error)
	SaveProcessesStateFunc            func(ws *workspace.Workspace, processes map[string]*host.ProcessInfo) error
	RemoveProcessesStateFunc          func(ws *workspace.Workspace) error
	DetectComposePathFunc             func(servicePath string, command string, explicitComposePath string) string
	StopServiceWithCommandAndPathFunc func(ctx context.Context, pid int, stopCommand string, servicePath string) error
	IsServiceRunningFunc              func(pid int) (bool, error)
}

func (m *MockHostRunner) StartService(
	ctx context.Context, ws *workspace.Workspace, deps *config.Deps,
	serviceName string, svc config.Service, projectDir string,
) (*host.ProcessInfo, error) {
	if m.StartServiceFunc != nil {
		return m.StartServiceFunc(ctx, ws, deps, serviceName, svc, projectDir)
	}
	return nil, nil
}

func (m *MockHostRunner) StopServiceWithCommand(ctx context.Context, pid int, stopCommand string) error {
	if m.StopServiceWithCommandFunc != nil {
		return m.StopServiceWithCommandFunc(ctx, pid, stopCommand)
	}
	return nil
}

func (m *MockHostRunner) LoadProcessesState(ws *workspace.Workspace) (map[string]*host.ProcessInfo, error) {
	if m.LoadProcessesStateFunc != nil {
		return m.LoadProcessesStateFunc(ws)
	}
	return nil, nil
}

func (m *MockHostRunner) SaveProcessesState(ws *workspace.Workspace, processes map[string]*host.ProcessInfo) error {
	if m.SaveProcessesStateFunc != nil {
		return m.SaveProcessesStateFunc(ws, processes)
	}
	return nil
}

func (m *MockHostRunner) RemoveProcessesState(ws *workspace.Workspace) error {
	if m.RemoveProcessesStateFunc != nil {
		return m.RemoveProcessesStateFunc(ws)
	}
	return nil
}

func (m *MockHostRunner) DetectComposePath(servicePath string, command string, explicitComposePath string) string {
	if m.DetectComposePathFunc != nil {
		return m.DetectComposePathFunc(servicePath, command, explicitComposePath)
	}
	return ""
}

func (m *MockHostRunner) StopServiceWithCommandAndPath(
	ctx context.Context, pid int, stopCommand string, servicePath string,
) error {
	if m.StopServiceWithCommandAndPathFunc != nil {
		return m.StopServiceWithCommandAndPathFunc(ctx, pid, stopCommand, servicePath)
	}
	return nil
}

func (m *MockHostRunner) IsServiceRunning(pid int) (bool, error) {
	if m.IsServiceRunningFunc != nil {
		return m.IsServiceRunningFunc(pid)
	}
	return false, nil
}
