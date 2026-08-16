package app

import (
	"context"
	"fmt"
	"os"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/naming"
	"raioz/internal/state"
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
		// The crashed project's dir is known and holds no state file →
		// LoadLocalState yields an empty state (no host PIDs): positive
		// proof of death, so the GC may prune.
		projectDirForFunc: func(string) string { return t.TempDir() },
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

// TestDownUseCase_stopProxy_SharedDepVetoesTeardown covers the ADR-005
// shared-dep veto: a live shared dep (project="" + kind=dependency) means
// some consumer is still up (ADR-002) even when it owns no labeled
// containers — e.g. a sibling whose services run via `command:`. The
// teardown must be vetoed.
func TestDownUseCase_stopProxy_SharedDepVetoesTeardown(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"acme-redis"}
	}
	getContainerLabelFn = func(_ context.Context, _, label string) (string, error) {
		if label == naming.LabelKind {
			return naming.KindDependency, nil
		}
		return "", nil // project label empty: shared dep by design (ADR-002)
	}
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return []string{"acme-redis"}, nil
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	var stopCalled, reloadCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:   func(_ context.Context) error { stopCalled = true; return nil },
		reloadFunc: func(_ context.Context) error { reloadCalled = true; return nil },
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
		t.Error("a live shared dep must veto the proxy teardown (ADR-002 contract)")
	}
	if !reloadCalled {
		t.Error("keep-alive path must reload the proxy so our routes are dropped")
	}
}

// TestDownUseCase_stopProxy_ProxyKindDoesNotVeto proves the veto is scoped:
// the workspace proxy container itself (project="" + kind=proxy) is not
// evidence of a consumer, so the last project out still tumba it.
func TestDownUseCase_stopProxy_ProxyKindDoesNotVeto(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string {
		return []string{"acme-proxy"}
	}
	getContainerLabelFn = func(_ context.Context, _, label string) (string, error) {
		if label == naming.LabelKind {
			return naming.KindProxy, nil
		}
		return "", nil
	}
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return []string{"acme-proxy"}, nil
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc: func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:   func(_ context.Context) error { stopCalled = true; return nil },
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

	if !stopCalled {
		t.Error("the proxy's own container must not veto the last-out teardown")
	}
}

// TestDownUseCase_stopProxy_HostRunSiblingRouteKeptByLivePID covers the
// ADR-005 host-aware GC: a sibling with zero containers but a live host PID
// recorded in its .raioz.state.json must keep its route file (and thereby
// the shared proxy).
func TestDownUseCase_stopProxy_HostRunSiblingRouteKeptByLivePID(t *testing.T) {
	initI18nForTest(t)

	siblingDir := t.TempDir()
	if err := state.SaveLocalState(siblingDir, &state.LocalState{
		Project:  "beta",
		HostPIDs: map[string]int{"web": os.Getpid()}, // provably alive
	}); err != nil {
		t.Fatal(err)
	}

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, nil // docker reachable, zero containers anywhere
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { stopCalled = true; return nil },
		reloadFunc:                 func(_ context.Context) error { return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"beta"} },
		remainingProjectsFunc:      func() int { return 1 }, // beta's file survives
		projectDirForFunc:          func(string) string { return siblingDir },
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
		t.Errorf("route file of a host-run live sibling must not be pruned, got %v", proxy.removedRoutesFor)
	}
	if stopCalled {
		t.Error("proxy must stay alive while a host-run sibling is provably alive")
	}
}

// TestDownUseCase_stopProxy_DeadHostPIDsGetPruned is the complement: when the
// sibling's state is readable and every recorded PID is dead, the GC has its
// positive proof of death and must prune (no immortal route files).
func TestDownUseCase_stopProxy_DeadHostPIDsGetPruned(t *testing.T) {
	initI18nForTest(t)

	siblingDir := t.TempDir()
	if err := state.SaveLocalState(siblingDir, &state.LocalState{
		Project:  "beta",
		HostPIDs: map[string]int{"web": 4194304}, // arbitrary; probe stubbed dead
	}); err != nil {
		t.Fatal(err)
	}

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	prevAlive := hostPIDAliveFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, nil
	}
	hostPIDAliveFn = func(int) bool { return false }
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
		hostPIDAliveFn = prevAlive
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { stopCalled = true; return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"beta"} },
		projectDirForFunc:          func(string) string { return siblingDir },
	}
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

	if len(proxy.removedRoutesFor) != 1 {
		t.Errorf("dead sibling's route file must be pruned, got %v", proxy.removedRoutesFor)
	}
	if !stopCalled {
		t.Error("last project out must tumba the proxy once the dead sibling's file is gone")
	}
}

// TestDownUseCase_stopProxy_LegacyRouteFileNeverPruned: a route file without
// a persisted projectDir (written pre-field, or by a remote sub-project,
// ADR-049) gives the GC no way to prove death — it must be kept.
func TestDownUseCase_stopProxy_LegacyRouteFileNeverPruned(t *testing.T) {
	initI18nForTest(t)

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, nil
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { stopCalled = true; return nil },
		reloadFunc:                 func(_ context.Context) error { return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"legacy"} },
		remainingProjectsFunc:      func() int { return 1 },
		projectDirForFunc:          func(string) string { return "" }, // pre-field file
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
		t.Errorf("legacy route file must never be pruned, got %v", proxy.removedRoutesFor)
	}
	if stopCalled {
		t.Error("proxy must stay alive when a route file's owner liveness is unknown")
	}
}

// TestDownUseCase_stopProxy_LiveRouteTargetKeepsRouteFile is issue 023: a
// launcher-pattern sibling (`command: make start`) has a dead launcher PID
// and unlabeled user-owned containers, so the label + host-PID probes both
// miss it. Its persisted route target still points at a RUNNING container —
// the GC must keep the file (a live backend is proof of life, ADR-005).
func TestDownUseCase_stopProxy_LiveRouteTargetKeepsRouteFile(t *testing.T) {
	initI18nForTest(t)

	siblingDir := t.TempDir()
	if err := state.SaveLocalState(siblingDir, &state.LocalState{
		Project:  "keycloak",
		HostPIDs: map[string]int{"keycloak": 4194304}, // the make-start launcher, dead
	}); err != nil {
		t.Fatal(err)
	}

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	prevAlive, prevAny := hostPIDAliveFn, anyContainerRunningFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, nil
	}
	hostPIDAliveFn = func(int) bool { return false } // launcher dead
	var probedTargets []string
	anyContainerRunningFn = func(_ context.Context, names []string) (bool, error) {
		probedTargets = names
		return true, nil // the user-owned container is up
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
		hostPIDAliveFn, anyContainerRunningFn = prevAlive, prevAny
	}()

	var stopCalled bool
	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { stopCalled = true; return nil },
		reloadFunc:                 func(_ context.Context) error { return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"keycloak"} },
		remainingProjectsFunc:      func() int { return 1 },
		projectDirForFunc:          func(string) string { return siblingDir },
		routeTargetsForFunc:        func(string) []string { return []string{"gouduet-keycloak"} },
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
					Workspace: "gouduet",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if len(proxy.removedRoutesFor) != 0 {
		t.Errorf("live launcher-pattern sibling's route file must be kept, got %v", proxy.removedRoutesFor)
	}
	if stopCalled {
		t.Error("proxy must stay alive: the sibling's route-target container is running")
	}
	if len(probedTargets) != 1 || probedTargets[0] != "gouduet-keycloak" {
		t.Errorf("expected the persisted route target to be probed, got %v", probedTargets)
	}
}

// TestDownUseCase_stopProxy_RouteTargetProbeErrorKeepsFile: a docker error
// while probing route targets must fail closed — the file is kept rather
// than risk pruning a live sibling on a transient outage (ADR-005).
func TestDownUseCase_stopProxy_RouteTargetProbeErrorKeepsFile(t *testing.T) {
	initI18nForTest(t)

	siblingDir := t.TempDir()
	if err := state.SaveLocalState(siblingDir, &state.LocalState{
		Project:  "keycloak",
		HostPIDs: map[string]int{"keycloak": 4194304},
	}); err != nil {
		t.Fatal(err)
	}

	prevList, prevLabel, prevErr := listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn
	prevAlive, prevAny := hostPIDAliveFn, anyContainerRunningFn
	listContainersByLabelsFn = func(_ context.Context, _ map[string]string) []string { return nil }
	getContainerLabelFn = func(_ context.Context, _, _ string) (string, error) { return "", nil }
	listContainersByLabelsErrFn = func(_ context.Context, _ map[string]string) ([]string, error) {
		return nil, nil
	}
	hostPIDAliveFn = func(int) bool { return false }
	anyContainerRunningFn = func(_ context.Context, _ []string) (bool, error) {
		return false, fmt.Errorf("docker daemon unreachable")
	}
	defer func() {
		listContainersByLabelsFn, getContainerLabelFn, listContainersByLabelsErrFn = prevList, prevLabel, prevErr
		hostPIDAliveFn, anyContainerRunningFn = prevAlive, prevAny
	}()

	proxy := &mockProxyManager{
		statusFunc:                 func(_ context.Context) (bool, error) { return true, nil },
		stopFunc:                   func(_ context.Context) error { return nil },
		reloadFunc:                 func(_ context.Context) error { return nil },
		listProjectsWithRoutesFunc: func() []string { return []string{"keycloak"} },
		remainingProjectsFunc:      func() int { return 1 },
		projectDirForFunc:          func(string) string { return siblingDir },
		routeTargetsForFunc:        func(string) []string { return []string{"gouduet-keycloak"} },
	}
	deps := &Dependencies{
		ProxyManager: proxy,
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*models.Deps, []string, error) {
				return &models.Deps{
					Project:   models.Project{Name: "alpha"},
					Workspace: "gouduet",
				}, nil, nil
			},
		},
	}
	uc := NewDownUseCase(deps)
	uc.stopProxy(context.Background(), DownOptions{})

	if len(proxy.removedRoutesFor) != 0 {
		t.Errorf("probe error must fail closed (keep the file), got %v", proxy.removedRoutesFor)
	}
}
