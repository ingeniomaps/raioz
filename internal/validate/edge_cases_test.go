package validate

import (
	"testing"

	"raioz/internal/config"
)

// TestInvalidConfigMissingRequiredFields tests validation with missing required fields
func TestInvalidConfigMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		deps *config.Deps
		want bool // want error
	}{
		{
			name: "missing project name",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Project: config.Project{
						},
				Services: map[string]config.Service{},
				Infra:    map[string]config.InfraEntry{},
				Env:      config.EnvConfig{},
			},
			want: true,
		},
		{
			name: "missing network",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Project: config.Project{
					Name: "test-project",
				},
				Services: map[string]config.Service{},
				Infra:    map[string]config.InfraEntry{},
				Env:      config.EnvConfig{},
			},
			want: true,
		},
		{
			name: "missing schema version",
			deps: &config.Deps{
				Project: config.Project{
					Name:    "test-project",
						},
				Services: map[string]config.Service{},
				Infra:    map[string]config.InfraEntry{},
				Env:      config.EnvConfig{},
			},
			want: true,
		},
		{
			name: "invalid schema version",
			deps: &config.Deps{
				SchemaVersion: "2.0", // Invalid, must be 1.0
				Project: config.Project{
					Name:    "test-project",
						},
				Services: map[string]config.Service{},
				Infra:    map[string]config.InfraEntry{},
				Env:      config.EnvConfig{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := All(tt.deps)
			hasError := err != nil

			if hasError != tt.want {
				t.Errorf("All() error = %v, want error = %v", err, tt.want)
			}
		})
	}
}

// TestInvalidServiceConfig tests validation with invalid service configurations
func TestInvalidServiceConfig(t *testing.T) {
	tests := []struct {
		name string
		deps *config.Deps
		want bool // want error
	}{
		{
			name: "git service missing repo",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Project: config.Project{
					Name:    "test",
						},
				Services: map[string]config.Service{
					"test": {
						Source: config.SourceConfig{
							Kind:   "git",
							Branch: "main",
							Path:   "services/test",
							// Missing repo
						},
						Docker: &config.DockerConfig{
							Mode: "dev",
						},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{},
			},
			want: true,
		},
		{
			name: "git service missing branch",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Project: config.Project{
					Name:    "test",
						},
				Services: map[string]config.Service{
					"test": {
						Source: config.SourceConfig{
							Kind: "git",
							Repo: "git@github.com:test/repo.git",
							Path: "services/test",
							// Missing branch
						},
						Docker: &config.DockerConfig{
							Mode: "dev",
						},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{},
			},
			want: true,
		},
		{
			name: "image service missing image",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Project: config.Project{
					Name:    "test",
						},
				Services: map[string]config.Service{
					"test": {
						Source: config.SourceConfig{
							Kind: "image",
							Tag:  "latest",
							// Missing image
						},
						Docker: &config.DockerConfig{
							Mode: "dev",
						},
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{},
			},
			want: true,
		},
		{
			name: "service missing docker config",
			deps: &config.Deps{
				SchemaVersion: "1.0",
				Project: config.Project{
					Name:    "test",
						},
				Services: map[string]config.Service{
					"test": {
						Source: config.SourceConfig{
							Kind:  "image",
							Image: "test/image",
							Tag:   "latest",
						},
						// Missing docker config
					},
				},
				Infra: map[string]config.InfraEntry{},
				Env:   config.EnvConfig{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := All(tt.deps)
			hasError := err != nil

			if hasError != tt.want {
				t.Errorf("All() error = %v, want error = %v", err, tt.want)
			}
		})
	}
}

// TestEdgeCaseCircularDependencies tests circular dependency detection
func TestEdgeCaseCircularDependencies(t *testing.T) {
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "test",
		},
		Services: map[string]config.Service{
			"service1": {
				Source: config.SourceConfig{
					Kind:  "image",
					Image: "test/image1",
					Tag:   "latest",
				},
				Docker: &config.DockerConfig{
					Mode:      "dev",
					DependsOn: []string{"service2"},
				},
			},
			"service2": {
				Source: config.SourceConfig{
					Kind:  "image",
					Image: "test/image2",
					Tag:   "latest",
				},
				Docker: &config.DockerConfig{
					Mode:      "dev",
					DependsOn: []string{"service1"}, // Circular!
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
		Env:   config.EnvConfig{},
	}

	err := All(deps)
	if err == nil {
		t.Error("Expected error for circular dependencies, got nil")
	}
}

// TestEdgeCaseInvalidPortFormat tests invalid port format
func TestEdgeCaseInvalidPortFormat(t *testing.T) {
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "test",
		},
		Services: map[string]config.Service{
			"test": {
				Source: config.SourceConfig{
					Kind:  "image",
					Image: "test/image",
					Tag:   "latest",
				},
				Docker: &config.DockerConfig{
					Mode:  "dev",
					Ports: []string{"invalid-port"}, // Invalid format
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
		Env:   config.EnvConfig{},
	}

	// This should fail schema validation
	err := All(deps)
	if err == nil {
		t.Error("Expected error for invalid port format, got nil")
	}
}

// TestEdgeCaseInvalidProjectName tests invalid project name format
func TestEdgeCaseInvalidProjectName(t *testing.T) {
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Project: config.Project{
			Name: "Invalid Name!", // Invalid: contains spaces and special chars
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
		Env:      config.EnvConfig{},
	}

	err := All(deps)
	if err == nil {
		t.Error("Expected error for invalid project name, got nil")
	}
}

// TestEdgeCaseInvalidNetworkName tests invalid network name format
func TestEdgeCaseInvalidNetworkName(t *testing.T) {
	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network: config.NetworkConfig{Name: "Invalid Network!"}, // Invalid: contains spaces
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
		Env:      config.EnvConfig{},
	}

	err := All(deps)
	if err == nil {
		t.Error("Expected error for invalid network name, got nil")
	}
}
