package upcase

import (
	stderrors "errors"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
)

// --- applyFilters: feature flag validation error -----------------------------

func TestApplyFiltersFeatureFlagValidationError(t *testing.T) {
	initI18nUp(t)

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error {
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

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project:       config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api":    {Source: config.SourceConfig{Kind: "image"}},
			"web":    {Source: config.SourceConfig{Kind: "image"}},
			"worker": {Source: config.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres"}},
		},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
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

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project:       config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
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

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project:       config.Project{Name: "test"},
		Profiles:      []string{"backend"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image"}, Profiles: []string{"backend"}},
			"web": {Source: config.SourceConfig{Kind: "image"}, Profiles: []string{"frontend"}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByProfilesFunc: func(deps *config.Deps, profiles []string) *config.Deps {
				filtered := *deps
				filtered.Services = map[string]config.Service{
					"api": deps.Services["api"],
				}
				return &filtered
			},
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
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

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project:       config.Project{Name: "test"},
		Profiles:      []string{"nonexistent"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByProfilesFunc: func(deps *config.Deps, profiles []string) *config.Deps {
				filtered := *deps
				filtered.Services = map[string]config.Service{}
				filtered.Infra = map[string]config.InfraEntry{}
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
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
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

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name:     "test",
			Commands: &config.ProjectCommands{Up: "make up"},
		},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image"}},
			"web": {Source: config.SourceConfig{Kind: "image"}},
		},
		Infra: map[string]config.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
				// Remove api, keep web
				filtered := *deps
				filtered.Services = map[string]config.Service{
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

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "cmd-only",
			Commands: &config.ProjectCommands{
				Up: "make start",
			},
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error { return nil },
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
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
