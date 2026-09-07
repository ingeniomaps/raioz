package docker

import (
	"testing"

	"raioz/internal/domain/models"
)

func TestBuildImageName(t *testing.T) {
	tests := []struct {
		image string
		tag   string
		want  string
	}{
		{"nginx", "latest", "nginx:latest"},
		{"nginx", "", "nginx"},
		{"myapp", "v1.0.0", "myapp:v1.0.0"},
		{"registry.io/app", "dev", "registry.io/app:dev"},
	}

	for _, tt := range tests {
		t.Run(tt.image+":"+tt.tag, func(t *testing.T) {
			got := BuildImageName(tt.image, tt.tag)
			if got != tt.want {
				t.Errorf("BuildImageName(%s, %s) = %v, want %v", tt.image, tt.tag, got, tt.want)
			}
		})
	}
}

func TestValidateAllImages(t *testing.T) {
	// Test with no images (should pass)
	deps := &models.Deps{
		Services: map[string]models.Service{
			"service1": {
				Source: models.SourceConfig{
					Kind: "git",
				},
			},
		},
		Infra: map[string]models.InfraEntry{},
	}

	if err := ValidateAllImages(deps); err != nil {
		t.Errorf("ValidateAllImages() error = %v, want nil", err)
	}
}
