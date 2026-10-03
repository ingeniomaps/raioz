package upcase

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

func TestApplyInternalDepPort(t *testing.T) {
	prev := imageExposedPortFn
	imageExposedPortFn = func(_ context.Context, image string) (int, error) {
		if image == "redis:7" {
			return 6379, nil
		}
		return 0, errors.New("unknown image")
	}
	t.Cleanup(func() { imageExposedPortFn = prev })

	tests := []struct {
		name     string
		infra    *models.Infra
		ep       interfaces.ServiceEndpoint
		wantPort int
		wantOnly bool
	}{
		{"declared expose", &models.Infra{Image: "nginx", Expose: []int{8080}}, interfaces.ServiceEndpoint{}, 8080, true},
		{"port the image exposes", &models.Infra{Image: "redis", Tag: "7"}, interfaces.ServiceEndpoint{}, 6379, true},
		{"image that exposes nothing known", &models.Infra{Image: "scratch"}, interfaces.ServiceEndpoint{}, 0, false},
		{
			"published dependency keeps its mapping",
			&models.Infra{Image: "redis", Tag: "7"},
			interfaces.ServiceEndpoint{Port: 6379, HostPort: 36379}, 6379, false,
		},
		{"compose dependency", &models.Infra{Compose: []string{"./x.yml"}}, interfaces.ServiceEndpoint{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &models.Deps{Infra: map[string]models.InfraEntry{"dep": {Inline: tt.infra}}}
			ep := tt.ep
			applyInternalDepPort(context.Background(), &ep, "dep", deps)
			if ep.Port != tt.wantPort || ep.ContainerOnly != tt.wantOnly {
				t.Errorf("Port=%d ContainerOnly=%v, want %d/%v", ep.Port, ep.ContainerOnly, tt.wantPort, tt.wantOnly)
			}
		})
	}
}
