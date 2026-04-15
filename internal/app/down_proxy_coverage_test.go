package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
)

// proxyTestDeps returns Dependencies with ConfigLoader + ProxyManager wired up
// so stopProxy doesn't nil-panic on ConfigLoader.LoadDeps.
func proxyTestDeps(proxy interfaces.ProxyManager) *Dependencies {
	return &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
				return &config.Deps{Project: config.Project{Name: "test"}}, nil, nil
			},
		},
	}
}

func TestDownUseCase_stopProxy_Running(t *testing.T) {
	initI18nForTest(t)
	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(ctx context.Context) (bool, error) {
			return true, nil
		},
		stopFunc: func(ctx context.Context) error {
			stopCalled = true
			return nil
		},
	}
	uc := NewDownUseCase(proxyTestDeps(proxy))
	uc.stopProxy(context.Background(), DownOptions{})
	if !stopCalled {
		t.Error("expected Stop to be called")
	}
}

func TestDownUseCase_stopProxy_NotRunning(t *testing.T) {
	initI18nForTest(t)
	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(ctx context.Context) (bool, error) {
			return false, nil
		},
		stopFunc: func(ctx context.Context) error {
			stopCalled = true
			return nil
		},
	}
	uc := NewDownUseCase(proxyTestDeps(proxy))
	uc.stopProxy(context.Background(), DownOptions{})
	if stopCalled {
		t.Error("expected Stop not to be called when proxy not running")
	}
}

func TestDownUseCase_stopProxy_StatusError(t *testing.T) {
	initI18nForTest(t)
	proxy := &mockProxyManager{
		statusFunc: func(ctx context.Context) (bool, error) {
			return false, fmt.Errorf("status error")
		},
	}
	uc := NewDownUseCase(proxyTestDeps(proxy))
	// Should not panic
	uc.stopProxy(context.Background(), DownOptions{})
}

func TestDownUseCase_stopProxy_StopError(t *testing.T) {
	initI18nForTest(t)
	proxy := &mockProxyManager{
		statusFunc: func(ctx context.Context) (bool, error) {
			return true, nil
		},
		stopFunc: func(ctx context.Context) error {
			return fmt.Errorf("stop fail")
		},
	}
	uc := NewDownUseCase(proxyTestDeps(proxy))
	// Should not panic, just log warning
	uc.stopProxy(context.Background(), DownOptions{})
}

// Verify mockProxyManager implements the interface
var _ interfaces.ProxyManager = (*mockProxyManager)(nil)

// TestDownUseCase_stopProxy_WorkspaceSharedSkipsWhenSiblingsActive proves
// that downing project A in workspace `acme` does NOT tumba the shared
// proxy when project B's containers still exist. This is the Phase B
// guarantee for the workspace-shared proxy lifecycle.
func TestDownUseCase_stopProxy_WorkspaceSharedSkipsWhenSiblingsActive(t *testing.T) {
	initI18nForTest(t)

	// Stub the workspace-occupancy probe to claim a sibling exists.
	prevList, prevLabel := listContainersByLabelsFn, getContainerLabelFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"acme-other-api"} // a non-project-A container
	}
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) {
		return "other", nil // belongs to project "other", not "alpha"
	}
	defer func() {
		listContainersByLabelsFn = prevList
		getContainerLabelFn = prevLabel
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(_ context.Context) (bool, error) { return true, nil },
		stopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	}

	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project:   config.Project{Name: "alpha"},
					Workspace: "acme",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if stopCalled {
		t.Error("Stop should NOT be called when siblings are still active in the workspace")
	}
}

// TestDownUseCase_stopProxy_WorkspaceSharedReloadsWhenSiblingsRemain proves
// the Phase C lifecycle: removing one project's routes triggers a reload
// (so Caddy stops serving them) instead of either a no-op or a Stop.
func TestDownUseCase_stopProxy_WorkspaceSharedReloadsWhenSiblingsRemain(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel := listContainersByLabelsFn, getContainerLabelFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"acme-other-api"}
	}
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) {
		return "other", nil
	}
	defer func() {
		listContainersByLabelsFn = prevList
		getContainerLabelFn = prevLabel
	}()

	var stopCalled, reloadCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(_ context.Context) (bool, error) { return true, nil },
		stopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
		reloadFunc: func(_ context.Context) error {
			reloadCalled = true
			return nil
		},
		remainingProjectsFunc: func() int { return 1 },
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project:   config.Project{Name: "alpha"},
					Workspace: "acme",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if !proxy.removeProjectRoutesCalled {
		t.Error("RemoveProjectRoutes must run before deciding reload vs stop")
	}
	if !reloadCalled {
		t.Error("Reload must run when siblings remain (so Caddy drops our routes)")
	}
	if stopCalled {
		t.Error("Stop must NOT run when siblings remain")
	}
}

// TestDownUseCase_stopProxy_WorkspaceSharedTumbasWhenAlone confirms the
// last-out-turns-off-the-lights semantics: when the workspace probe shows
// no other project active, the shared proxy gets torn down normally.
func TestDownUseCase_stopProxy_WorkspaceSharedTumbasWhenAlone(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel := listContainersByLabelsFn, getContainerLabelFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return nil // no siblings — only the leaving project remains
	}
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) {
		return "", nil
	}
	defer func() {
		listContainersByLabelsFn = prevList
		getContainerLabelFn = prevLabel
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(_ context.Context) (bool, error) { return true, nil },
		stopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
		remainingProjectsFunc: func() int { return 0 },
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*config.Deps, []string, error) {
				return &config.Deps{
					Project:   config.Project{Name: "alpha"},
					Workspace: "acme",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if !proxy.removeProjectRoutesCalled {
		t.Error("RemoveProjectRoutes must run on every shared down")
	}
	if !stopCalled {
		t.Error("last project out must tumba the shared proxy")
	}
}
