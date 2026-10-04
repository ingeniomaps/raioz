package app

import (
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

func TestServiceHostPort(t *testing.T) {
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(string) (*models.Deps, []string, error) {
			return &models.Deps{
				Project:      models.Project{Name: "p"},
				SourceFormat: models.SourceFormatYAML,
				Services: map[string]models.Service{
					"web": {Source: models.SourceConfig{Kind: "local", Path: ".", Command: "run"}, Port: 4321},
				},
				Infra: map[string]models.InfraEntry{},
			}, nil, nil
		},
	}

	if port, ok := ServiceHostPort(deps, "raioz.yaml", "web"); !ok || port != 4321 {
		t.Errorf("web = %d, %v; want 4321, true", port, ok)
	}
	if _, ok := ServiceHostPort(deps, "raioz.yaml", "nope"); ok {
		t.Error("an undeclared name has no port")
	}
	if _, ok := ServiceHostPort(deps, "", "web"); ok {
		t.Error("no config, no port")
	}
}
