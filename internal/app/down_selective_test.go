package app

import (
	"strings"
	"testing"

	"raioz/internal/config"
)

// declaredServiceNames / declaredInfraNames / hasService / hasInfra are
// trivial but they're the resolution surface for issue 012's "is this a
// known target?" check. Pinning them prevents a future refactor from
// silently dropping a kind.
func TestHasServiceAndInfra(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{"api": {}, "web": {}},
		Infra:    map[string]config.InfraEntry{"postgres": {}, "redis": {}},
	}

	if !hasService(deps, "api") || !hasService(deps, "web") {
		t.Errorf("hasService missed a declared service")
	}
	if hasService(deps, "postgres") {
		t.Errorf("hasService matched a dep")
	}
	if !hasInfra(deps, "postgres") || !hasInfra(deps, "redis") {
		t.Errorf("hasInfra missed a declared dep")
	}
	if hasInfra(deps, "api") {
		t.Errorf("hasInfra matched a service")
	}
}

func TestDeclaredNamesIncludeEverything(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{"api": {}, "web": {}},
		Infra:    map[string]config.InfraEntry{"postgres": {}},
	}

	svcs := declaredServiceNames(deps)
	if len(svcs) != 2 {
		t.Errorf("declaredServiceNames len = %d, want 2", len(svcs))
	}
	deps2Joined := strings.Join(declaredInfraNames(deps), ",")
	if deps2Joined != "postgres" {
		t.Errorf("declaredInfraNames = %q, want postgres", deps2Joined)
	}
}
