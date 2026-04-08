package upcase

import (
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
)

func TestApplyFiltersNoProfile(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error {
				return nil
			},
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
				return deps, nil
			},
		},
	})

	result, err := uc.applyFilters(deps, "", nil)
	if err != nil {
		t.Fatalf("applyFilters() error: %v", err)
	}
	if len(result.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(result.Services))
	}
}

func TestApplyFiltersWithProfile(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()
	deps.Services["web"] = config.Service{
		Source:   config.SourceConfig{Kind: "image", Image: "nginx", Tag: "latest"},
		Docker:   &config.DockerConfig{Mode: "prod", Ports: []string{"80:80"}},
		Profiles: []string{"frontend"},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error {
				return nil
			},
			FilterByProfileFunc: func(deps *config.Deps, profile string) *config.Deps {
				filtered := *deps
				filtered.Services = make(map[string]config.Service)
				for name, svc := range deps.Services {
					if len(svc.Profiles) == 0 || contains(svc.Profiles, profile) {
						filtered.Services[name] = svc
					}
				}
				return &filtered
			},
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
				return deps, nil
			},
		},
	})

	result, err := uc.applyFilters(deps, "frontend", nil)
	if err != nil {
		t.Fatalf("applyFilters() error: %v", err)
	}

	// Should include web (has frontend profile) + api (no profile = always included)
	if _, ok := result.Services["web"]; !ok {
		t.Error("web service should be included for frontend profile")
	}
}

func TestApplyFiltersEmptyAfterProfile(t *testing.T) {
	initI18nUp(t)

	deps := validDeps()
	// All services have profiles that don't match

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error {
				return nil
			},
			FilterByProfileFunc: func(deps *config.Deps, profile string) *config.Deps {
				filtered := *deps
				filtered.Services = make(map[string]config.Service)
				filtered.Infra = make(map[string]config.InfraEntry)
				return &filtered
			},
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
				return deps, nil
			},
		},
	})

	_, err := uc.applyFilters(deps, "nonexistent-profile", nil)
	if err == nil {
		t.Error("expected error when no services match profile")
	}
}

func TestApplyFiltersNoServicesNoInfraNoCommands(t *testing.T) {
	initI18nUp(t)

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project:       config.Project{Name: "empty"},
		Services:      map[string]config.Service{},
		Infra:         map[string]config.InfraEntry{},
	}

	uc := NewUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			ValidateFeatureFlagsFunc: func(deps *config.Deps) error {
				return nil
			},
			FilterByFeatureFlagsFunc: func(deps *config.Deps, profile string, envVars map[string]string) (*config.Deps, []string) {
				return deps, nil
			},
		},
	})

	_, err := uc.applyFilters(deps, "", nil)
	if err == nil {
		t.Error("expected error when no services, infra, or commands")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
