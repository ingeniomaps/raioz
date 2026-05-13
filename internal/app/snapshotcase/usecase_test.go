package snapshotcase

import (
	"context"
	"errors"
	"testing"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

// mockSnapshotManager is a minimal stand-in for the SnapshotManager
// port. Keeps assertions local to this package; if more tests grow
// elsewhere, promote to internal/mocks.
type mockSnapshotManager struct {
	createCalls   int
	createSnap    *interfaces.Snapshot
	createErr     error
	restoreCalled bool
	restoreErr    error
	listResult    []interfaces.Snapshot
	listErr       error
	deleteCalled  bool
	deleteErr     error
}

func (m *mockSnapshotManager) Create(
	_ context.Context, project, name string, volumes map[string]string,
) (*interfaces.Snapshot, error) {
	m.createCalls++
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createSnap != nil {
		return m.createSnap, nil
	}
	return &interfaces.Snapshot{
		Name:      name,
		Project:   project,
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockSnapshotManager) Restore(_ context.Context, project, name string) error {
	m.restoreCalled = true
	return m.restoreErr
}

func (m *mockSnapshotManager) List(_ context.Context, project string) ([]interfaces.Snapshot, error) {
	return m.listResult, m.listErr
}

func (m *mockSnapshotManager) Delete(_ context.Context, project, name string) error {
	m.deleteCalled = true
	return m.deleteErr
}

func TestCreate_NoVolumes(t *testing.T) {
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{Project: models.Project{Name: "p"}}, nil, nil
		},
	}
	mgr := &mockSnapshotManager{}
	uc := CreateUseCase{Deps: &Dependencies{ConfigLoader: loader, SnapshotManager: mgr}}

	res, err := uc.Execute(context.Background(), CreateOptions{Name: "snap1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.NoVolumes {
		t.Error("expected NoVolumes=true for a project with no infra volumes")
	}
	if mgr.createCalls != 0 {
		t.Errorf("Create should not be invoked when there are no volumes (got %d calls)", mgr.createCalls)
	}
}

func TestCreate_PassesInfraVolumesToManager(t *testing.T) {
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project: models.Project{Name: "p"},
				Infra: map[string]models.InfraEntry{
					"postgres": {Inline: &models.Infra{
						Image:   "postgres:16",
						Volumes: []string{"pgdata", "pglog"},
					}},
				},
			}, nil, nil
		},
	}
	mgr := &mockSnapshotManager{}
	uc := CreateUseCase{Deps: &Dependencies{ConfigLoader: loader, SnapshotManager: mgr}}

	res, err := uc.Execute(context.Background(), CreateOptions{Name: "snap1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.NoVolumes {
		t.Error("expected NoVolumes=false when infra declares volumes")
	}
	if mgr.createCalls != 1 {
		t.Errorf("expected exactly one Create call, got %d", mgr.createCalls)
	}
}

func TestCreate_PropagatesManagerError(t *testing.T) {
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project: models.Project{Name: "p"},
				Infra: map[string]models.InfraEntry{
					"postgres": {Inline: &models.Infra{Volumes: []string{"pgdata"}}},
				},
			}, nil, nil
		},
	}
	mgr := &mockSnapshotManager{createErr: errors.New("disk full")}
	uc := CreateUseCase{Deps: &Dependencies{ConfigLoader: loader, SnapshotManager: mgr}}

	_, err := uc.Execute(context.Background(), CreateOptions{Name: "snap1"})
	if err == nil {
		t.Fatal("expected error to propagate from manager")
	}
}

func TestRestore_DelegatesToManager(t *testing.T) {
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{Project: models.Project{Name: "p"}}, nil, nil
		},
	}
	mgr := &mockSnapshotManager{}
	uc := RestoreUseCase{Deps: &Dependencies{ConfigLoader: loader, SnapshotManager: mgr}}

	if err := uc.Execute(context.Background(), RestoreOptions{Name: "snap1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mgr.restoreCalled {
		t.Error("expected SnapshotManager.Restore to be invoked")
	}
}

func TestList_ReturnsManagerResult(t *testing.T) {
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{Project: models.Project{Name: "p"}}, nil, nil
		},
	}
	mgr := &mockSnapshotManager{
		listResult: []interfaces.Snapshot{{Name: "snap1"}, {Name: "snap2"}},
	}
	uc := ListUseCase{Deps: &Dependencies{ConfigLoader: loader, SnapshotManager: mgr}}

	snaps, err := uc.Execute(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snaps) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snaps))
	}
}

func TestDelete_DelegatesToManager(t *testing.T) {
	loader := &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{Project: models.Project{Name: "p"}}, nil, nil
		},
	}
	mgr := &mockSnapshotManager{}
	uc := DeleteUseCase{Deps: &Dependencies{ConfigLoader: loader, SnapshotManager: mgr}}

	if err := uc.Execute(context.Background(), DeleteOptions{Name: "snap1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mgr.deleteCalled {
		t.Error("expected SnapshotManager.Delete to be invoked")
	}
}
