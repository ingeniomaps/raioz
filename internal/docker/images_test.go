package docker

import (
	"raioz/internal/config"
	"testing"
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

func TestValidateServiceImages(t *testing.T) {
	// Test with no image services (should pass)
	deps := &config.Deps{
		Services: map[string]config.Service{
			"service1": {
				Source: config.SourceConfig{
					Kind: "git",
				},
			},
		},
	}

	if err := ValidateServiceImages(deps); err != nil {
		t.Errorf("ValidateServiceImages() error = %v, want nil", err)
	}

	// Test with image service (will try to check/pull, may fail if no docker)
	deps2 := &config.Deps{
		Services: map[string]config.Service{
			"service2": {
				Source: config.SourceConfig{
					Kind:  "image",
					Image: "nginx",
					Tag:   "alpine",
				},
			},
		},
	}

	// This may fail if docker is not available, but that's ok for test
	_ = ValidateServiceImages(deps2)
}

func TestValidateInfraImages(t *testing.T) {
	// Test with no infra (should pass)
	deps := &config.Deps{
		Infra: map[string]config.InfraEntry{},
	}

	if err := ValidateInfraImages(deps); err != nil {
		t.Errorf("ValidateInfraImages() error = %v, want nil", err)
	}

	// Test with infra (will try to check/pull, may fail if no docker)
	deps2 := &config.Deps{
		Infra: map[string]config.InfraEntry{
			"mongo": {Inline: &config.Infra{
				Image: "mongo",
				Tag:   "5.0",
			}},
		},
	}

	// This may fail if docker is not available, but that's ok for test
	_ = ValidateInfraImages(deps2)
}

func TestValidateAllImages(t *testing.T) {
	// Test with no images (should pass)
	deps := &config.Deps{
		Services: map[string]config.Service{
			"service1": {
				Source: config.SourceConfig{
					Kind: "git",
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
	}

	if err := ValidateAllImages(deps); err != nil {
		t.Errorf("ValidateAllImages() error = %v, want nil", err)
	}
}
