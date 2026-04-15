package docker

import (
	"testing"

	"raioz/internal/config"
)

func TestValidateDependencyCycle(t *testing.T) {
	tests := []struct {
		name    string
		deps    *config.Deps
		wantErr bool
		errMsg  string
	}{
		{
			name: "no dependencies",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {},
					"service2": {},
				},
			},
			wantErr: false,
		},
		{
			name: "valid dependencies",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {},
					"service2": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service1"},
						},
					},
					"service3": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "circular dependency - two services",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
					"service2": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service1"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "circular dependency - three services",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service3"},
						},
					},
					"service2": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service1"},
						},
					},
					"service3": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "self-dependency",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service1"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "multiple dependencies",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {},
					"service2": {},
					"service3": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service1", "service2"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "diamond dependency (valid)",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"base": {},
					"service1": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"base"},
						},
					},
					"service2": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"base"},
						},
					},
					"service3": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service1", "service2"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDependencyCycle(tt.deps)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDependencyCycle() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateDependencyCycle() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDependencyCycle() unexpected error = %v", err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 ||
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestGetAllServiceNames(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"service1": {},
			"service2": {},
		},
		Infra: map[string]config.InfraEntry{
			"infra1": {Inline: &config.Infra{}},
			"infra2": {Inline: &config.Infra{}},
		},
	}

	names := GetAllServiceNames(deps)

	expected := map[string]bool{
		"service1": true,
		"service2": true,
		"infra1":   true,
		"infra2":   true,
	}

	if len(names) != len(expected) {
		t.Errorf("GetAllServiceNames() returned %d names, want %d", len(names), len(expected))
	}

	for name := range expected {
		if !names[name] {
			t.Errorf("GetAllServiceNames() missing name: %s", name)
		}
	}
}
