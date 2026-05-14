package workspace

import (
	"runtime"
	"testing"

	"raioz/internal/domain/models"
)

func TestGetServiceDir(t *testing.T) {
	ws := &Workspace{
		Root:                "/tmp/workspace",
		ServicesDir:         "/tmp/services",
		LocalServicesDir:    "/tmp/workspace/local",
		ReadonlyServicesDir: "/tmp/workspace/readonly",
		EnvDir:              "/tmp/env",
	}

	tests := []struct {
		name     string
		svc      models.Service
		expected string
	}{
		{
			name: "readonly git service",
			svc: models.Service{
				Source: models.SourceConfig{
					Kind:   "git",
					Access: "readonly",
				},
			},
			expected: "/tmp/workspace/readonly",
		},
		{
			name: "editable git service",
			svc: models.Service{
				Source: models.SourceConfig{
					Kind:   "git",
					Access: "editable",
				},
			},
			expected: "/tmp/workspace/local",
		},
		{
			name: "default git service (editable)",
			svc: models.Service{
				Source: models.SourceConfig{
					Kind: "git",
				},
			},
			expected: "/tmp/workspace/local",
		},
		{
			name: "image service",
			svc: models.Service{
				Source: models.SourceConfig{
					Kind: "image",
				},
			},
			expected: "/tmp/services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetServiceDir(ws, tt.svc)
			if result != tt.expected {
				t.Errorf("GetServiceDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetServicePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("docs/issues/068: assertions use Unix path separators")
	}
	ws := &Workspace{
		Root:                "/tmp/workspace",
		ServicesDir:         "/tmp/services",
		LocalServicesDir:    "/tmp/workspace/local",
		ReadonlyServicesDir: "/tmp/workspace/readonly",
		EnvDir:              "/tmp/env",
	}

	tests := []struct {
		name     string
		svc      models.Service
		expected string
	}{
		{
			name: "readonly git service",
			svc: models.Service{
				Source: models.SourceConfig{
					Kind:   "git",
					Access: "readonly",
					Path:   "services/auth",
				},
			},
			expected: "/tmp/workspace/readonly/services/auth",
		},
		{
			name: "editable git service",
			svc: models.Service{
				Source: models.SourceConfig{
					Kind:   "git",
					Access: "editable",
					Path:   "services/users",
				},
			},
			expected: "/tmp/workspace/local/services/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetServicePath(ws, "test-service", tt.svc)
			if result != tt.expected {
				t.Errorf("GetServicePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}
