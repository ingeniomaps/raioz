package app

import (
	"fmt"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

func TestHandleDependencyAssist_NoMissing(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		DetectMissingDependenciesFunc: func(
			d *models.Deps,
			pr func(string, models.Service) string,
		) ([]models.MissingDependency, error) {
			return nil, nil
		},
	}
	cfgDeps := &models.Deps{Project: models.Project{Name: "test"}}
	ws := &interfaces.Workspace{}

	ok, added, err := HandleDependencyAssist(deps, cfgDeps, ws, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected continue=true with no missing deps")
	}
	if len(added) != 0 {
		t.Errorf("expected no added services, got %v", added)
	}
}

func TestHandleDependencyAssist_DetectError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		DetectMissingDependenciesFunc: func(
			d *models.Deps,
			pr func(string, models.Service) string,
		) ([]models.MissingDependency, error) {
			return nil, fmt.Errorf("detect failure")
		},
	}
	cfgDeps := &models.Deps{Project: models.Project{Name: "test"}}
	ws := &interfaces.Workspace{}

	_, _, err := HandleDependencyAssist(deps, cfgDeps, ws, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHandleDependencyAssist_DryRun(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		DetectMissingDependenciesFunc: func(
			d *models.Deps,
			pr func(string, models.Service) string,
		) ([]models.MissingDependency, error) {
			return []models.MissingDependency{
				{ServiceName: "db", RequiredBy: "api"},
			}, nil
		},
	}
	cfgDeps := &models.Deps{Project: models.Project{Name: "test"}}
	ws := &interfaces.Workspace{}

	ok, _, err := HandleDependencyAssist(deps, cfgDeps, ws, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected continue=false in dry-run mode when missing deps found")
	}
}

func TestHandleDependencyConflicts_None(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		DetectDependencyConflictsFunc: func(
			d *models.Deps,
			pr func(string, models.Service) string,
		) ([]models.DependencyConflict, error) {
			return nil, nil
		},
	}
	cfgDeps := &models.Deps{Project: models.Project{Name: "test"}}
	ws := &interfaces.Workspace{}

	ok, res, err := HandleDependencyConflicts(deps, cfgDeps, ws, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected continue=true")
	}
	if len(res) != 0 {
		t.Errorf("expected empty resolutions, got %v", res)
	}
}

func TestHandleDependencyConflicts_DetectError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		DetectDependencyConflictsFunc: func(
			d *models.Deps,
			pr func(string, models.Service) string,
		) ([]models.DependencyConflict, error) {
			return nil, fmt.Errorf("fail")
		},
	}
	cfgDeps := &models.Deps{Project: models.Project{Name: "test"}}
	ws := &interfaces.Workspace{}

	_, _, err := HandleDependencyConflicts(deps, cfgDeps, ws, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHandleDependencyConflicts_DryRun(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		DetectDependencyConflictsFunc: func(
			d *models.Deps,
			pr func(string, models.Service) string,
		) ([]models.DependencyConflict, error) {
			return []models.DependencyConflict{
				{ServiceName: "api", Differences: []string{"image differs"}},
			}, nil
		},
	}
	cfgDeps := &models.Deps{Project: models.Project{Name: "test"}}
	ws := &interfaces.Workspace{}

	ok, _, err := HandleDependencyConflicts(deps, cfgDeps, ws, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected continue=false in dry-run when conflicts found")
	}
}
