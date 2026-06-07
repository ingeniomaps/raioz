package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

// proxyTestDeps returns Dependencies with ConfigLoader + ProxyManager wired up
// so stopProxy doesn't nil-panic on ConfigLoader.LoadDeps.
func proxyTestDeps(proxy interfaces.ProxyManager) *Dependencies {
	return &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(configPath string) (*models.Deps, []string, error) {
				return &models.Deps{Project: models.Project{Name: "test"}}, nil, nil
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
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
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
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
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

// TestDownUseCase_stopProxy_OrphanRoutePrunedThenTumbas covers the ADR-005
// orphan route-file GC: the last project leaving a workspace must tumba the
// shared proxy even when a route file from a crashed project still sits on disk.
// The orphan GC deletes that file (its owner has no live container) so the
// gate sees RemainingProjects()==0.
func TestDownUseCase_stopProxy_OrphanRoutePrunedThenTumbas(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	// No siblings alive at all — neither the leaving project nor the
	// crashed "connector" has containers.
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, nil // reachable, zero containers
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { stopCalled = true; return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"connector"} },
	}
	// RemainingProjects mirrors reality: 1 until the orphan is pruned, then 0.
	proxy.remainingProjectsFunc = func() int {
		if len(proxy.removedRoutesFor) > 0 {
			return 0
		}
		return 1
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
					Workspace: "acme",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if len(proxy.removedRoutesFor) != 1 || proxy.removedRoutesFor[0] != "connector" {
		t.Errorf("orphan route file for 'connector' must be pruned, got %v", proxy.removedRoutesFor)
	}
	if !stopCalled {
		t.Error("last project out must tumba the proxy once the orphan file is gone")
	}
}

// TestDownUseCase_stopProxy_OrphanGCSkippedWhenDockerUnreachable proves the
// guard: if the liveness probe errors, the GC must NOT delete any route file
// (it can't prove the file is orphaned) and the proxy stays alive.
func TestDownUseCase_stopProxy_OrphanGCSkippedWhenDockerUnreachable(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, fmt.Errorf("docker daemon unreachable")
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { stopCalled = true; return nil },
		reloadFunc:                 func(_ context.Context) error { return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"connector"} },
		remainingProjectsFunc:      func() int { return 1 }, // file never pruned
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
					Workspace: "acme",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if len(proxy.removedRoutesFor) != 0 {
		t.Errorf("no route file may be pruned when docker is unreachable, got %v", proxy.removedRoutesFor)
	}
	if stopCalled {
		t.Error("proxy must stay alive (keep-alive) when the GC is skipped on a docker error")
	}
}

// TestDownUseCase_stopProxy_LiveSiblingRouteNotPruned ensures the GC only
// evicts files whose owner is dead: a route file for a project with a live
// container must survive.
func TestDownUseCase_stopProxy_LiveSiblingRouteNotPruned(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"acme-beta-api"}
	}
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "beta", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return []string{"acme-beta-api"}, nil
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		reloadFunc:                 func(_ context.Context) error { return nil },
		stopFunc:                   func(_ context.Context) error { return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"beta"} },
		remainingProjectsFunc:      func() int { return 1 },
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
					Workspace: "acme",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if len(proxy.removedRoutesFor) != 0 {
		t.Errorf("a live sibling's route file must not be pruned, got %v", proxy.removedRoutesFor)
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
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
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
