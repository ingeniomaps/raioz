package production

import (
	"testing"

	"raioz/internal/domain/models"
)

func localDeps(services map[string]models.Service, infra map[string]models.InfraEntry) *models.Deps {
	return &models.Deps{Services: services, Infra: infra}
}

// `raioz compare` used to panic here: a service declared with `command:`
// and no `ports:` carries no docker block, and the port comparison
// dereferenced it. That is the recommended shape for Next/Vite services,
// so the crash hit the common case.
func TestCompareConfigsServiceWithoutDockerBlock(t *testing.T) {
	local := localDeps(map[string]models.Service{
		"api": {Source: models.SourceConfig{Kind: "local", Command: "npm run dev"}},
	}, nil)
	prod := &ProductionConfig{Services: map[string]ProductionService{
		"api": {Image: "registry/api:1.2.3", Ports: []string{"8080:8080"}},
	}}

	result := CompareConfigs(local, prod)

	if len(result.ServiceDifferences) != 1 {
		t.Fatalf("expected one difference, got %+v", result.ServiceDifferences)
	}
	diff := result.ServiceDifferences[0]
	if diff.PortMismatch == nil {
		t.Error("production publishes a port the local service does not; expected a port mismatch")
	}
}
