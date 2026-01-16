package docker

import (
	"raioz/internal/config"
	"testing"
)

func TestParsePort(t *testing.T) {
	tests := []struct {
		name      string
		port      string
		wantPort  int
		wantError bool
	}{
		{
			name:      "simple port",
			port:      "3000",
			wantPort:  3000,
			wantError: false,
		},
		{
			name:      "host:container port",
			port:      "3000:8080",
			wantPort:  3000,
			wantError: false,
		},
		{
			name:      "invalid format",
			port:      "invalid",
			wantPort:  0,
			wantError: true,
		},
		{
			name:      "empty string",
			port:      "",
			wantPort:  0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := ParsePort(tt.port)

			if tt.wantError {
				if err == nil {
					t.Errorf("ParsePort() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParsePort() error = %v, want nil", err)
				return
			}

			if port != tt.wantPort {
				t.Errorf("ParsePort() = %v, want %v", port, tt.wantPort)
			}
		})
	}
}

func TestValidatePorts(t *testing.T) {
	// Create a test deps with ports
	deps := &config.Deps{
		Project: config.Project{
			Name: "test-project",
		},
		Services: map[string]config.Service{
			"service1": {
				Docker: config.DockerConfig{
					Ports: []string{"3000:8080"},
				},
			},
		},
		Infra: map[string]config.Infra{
			"infra1": {
				Ports: []string{"5432:5432"},
			},
		},
	}

	// Test with empty baseDir (no conflicts should be found)
	conflicts, err := ValidatePorts(deps, "/tmp", "test-project")
	if err != nil {
		t.Errorf("ValidatePorts() error = %v, want nil", err)
	}

	// Should not have conflicts with itself
	if len(conflicts) > 0 {
		for _, conflict := range conflicts {
			if conflict.Project == "test-project" {
				t.Errorf("ValidatePorts() found conflict with self: %v", conflict)
			}
		}
	}
}

func TestGetAllActivePorts(t *testing.T) {
	// Test with non-existent directory
	ports, err := GetAllActivePorts("/tmp/nonexistent")
	if err != nil {
		t.Errorf("GetAllActivePorts() error = %v, want nil", err)
	}

	// Should return empty list for non-existent directory
	if len(ports) != 0 {
		t.Errorf("GetAllActivePorts() = %v ports, want 0", len(ports))
	}
}
