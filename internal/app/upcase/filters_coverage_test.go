package upcase

import (
	stderrors "errors"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

// --- applyFilters: feature flag validation error -----------------------------

func TestApplyFiltersFeatureFlagValidationError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error {
				return stderrors.New("invalid feature flags")
			},
		},
	})

	deps := validDeps()
	_, err := uc.applyFilters(deps, "", nil)
	if err == nil {
		t.Error("expected error when feature flag validation fails")
	}
}

// --- applyFilters: with --only filter ----------------------------------------

func TestApplyFiltersWithOnlyExisting(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Project:       models.Project{Name: "test"},
		Services: map[string]models.Service{
			"api":    {Source: models.SourceConfig{Kind: "image"}},
			"web":    {Source: models.SourceConfig{Kind: "image"}},
			"worker": {Source: models.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]models.InfraEntry{
			"db": {Inline: &models.Infra{Image: "postgres"}},
		},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
				return deps, nil
			},
		},
	})

	result, err := uc.applyFilters(deps, "", []string{"api"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// api should be in the result (exact filtering depends on config.FilterByServices)
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestApplyFiltersWithOnlyNotFound(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Project:       models.Project{Name: "test"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]models.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
				return deps, nil
			},
		},
	})

	_, err := uc.applyFilters(deps, "", []string{"nonexistent"})
	if err == nil {
		t.Error("expected error when --only service not found")
	}
}

// --- applyFilters: with default profiles -------------------------------------

func TestApplyFiltersWithDefaultProfiles(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Project:       models.Project{Name: "test"},
		Profiles:      []string{"backend"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image"}, Profiles: []string{"backend"}},
			"web": {Source: models.SourceConfig{Kind: "image"}, Profiles: []string{"frontend"}},
		},
		Infra: map[string]models.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByProfilesFunc: func(deps *models.Deps, profiles []string) *models.Deps {
				filtered := *deps
				filtered.Services = map[string]models.Service{
					"api": deps.Services["api"],
				}
				return &filtered
			},
			FilterByFeatureFlagsFunc: func(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
				return deps, nil
			},
		},
	})

	result, err := uc.applyFilters(deps, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Services["api"]; !ok {
		t.Error("api should be included with backend profile")
	}
}

func TestApplyFiltersDefaultProfilesEmpty(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Project:       models.Project{Name: "test"},
		Profiles:      []string{"nonexistent"},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]models.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByProfilesFunc: func(deps *models.Deps, profiles []string) *models.Deps {
				filtered := *deps
				filtered.Services = map[string]models.Service{}
				filtered.Infra = map[string]models.InfraEntry{}
				return &filtered
			},
		},
	})

	_, err := uc.applyFilters(deps, "", nil)
	if err == nil {
		t.Error("expected error when default profiles filter out everything")
	}
}

// --- applyFilters: with mock services ----------------------------------------

func TestApplyFiltersWithMockServices(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
				return deps, []string{"api-mock"}
			},
		},
	})

	result, err := uc.applyFilters(deps, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}

// --- applyFilters: disabled by feature flags ---------------------------------

func TestApplyFiltersDisabledByFlags(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Project: models.Project{
			Name:     "test",
			Commands: &models.ProjectCommands{Up: "make up"},
		},
		Services: map[string]models.Service{
			"api": {Source: models.SourceConfig{Kind: "image"}},
			"web": {Source: models.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]models.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
				// Remove api, keep web
				filtered := *deps
				filtered.Services = map[string]models.Service{
					"web": deps.Services["web"],
				}
				return &filtered, nil
			},
		},
	})

	result, err := uc.applyFilters(deps, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Services) != 1 {
		t.Errorf("expected 1 service after flag filtering, got %d", len(result.Services))
	}
}

// --- applyFilters: with project commands (no services/infra) ----------------

func TestApplyFiltersOnlyProjectCommands(t *testing.T) {
	initI18nUp(t)

	deps := &models.Deps{
		SchemaVersion: "1.0",
		Project: models.Project{
			Name: "cmd-only",
			Commands: &models.ProjectCommands{
				Up: "make start",
			},
		},
		Services: map[string]models.Service{},
		Infra:    map[string]models.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *models.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *models.Deps, profile string, envVars map[string]string) (*models.Deps, []string) {
				return deps, nil
			},
		},
	})

	result, err := uc.applyFilters(deps, "", nil)
	if err != nil {
		t.Fatalf("should succeed with project commands: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}
