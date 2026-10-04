package upcase

import (
	"testing"

	"raioz/internal/domain/models"
)

func TestServiceResources(t *testing.T) {
	res := &models.Resources{Memory: "128m"}
	tests := []struct {
		name      string
		inherited bool
		runtime   models.Runtime
		capped    bool
	}{
		{"own block on a Dockerfile service", false, models.RuntimeDockerfile, true},
		{"own block on a host service", false, models.RuntimeNPM, true},
		{"own block on a compose service", false, models.RuntimeCompose, true},
		{"root default on a Dockerfile service", true, models.RuntimeDockerfile, true},
		{"root default skips a host service", true, models.RuntimeNPM, false},
		{"root default skips a compose service", true, models.RuntimeCompose, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := models.Service{Resources: res, ResourcesInherited: tt.inherited}
			got := serviceResources(svc, models.DetectResult{Runtime: tt.runtime})
			if (got != nil) != tt.capped {
				t.Errorf("capped = %v, want %v", got != nil, tt.capped)
			}
		})
	}
}
