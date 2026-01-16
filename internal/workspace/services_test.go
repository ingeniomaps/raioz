package workspace

import (
	"testing"

	"raioz/internal/config"
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
		svc      config.Service
		expected string
	}{
		{
			name: "readonly git service",
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "readonly",
				},
			},
			expected: "/tmp/workspace/readonly",
		},
		{
			name: "editable git service",
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "editable",
				},
			},
			expected: "/tmp/workspace/local",
		},
		{
			name: "default git service (editable)",
			svc: config.Service{
				Source: config.SourceConfig{
					Kind: "git",
				},
			},
			expected: "/tmp/workspace/local",
		},
		{
			name: "image service",
			svc: config.Service{
				Source: config.SourceConfig{
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
	ws := &Workspace{
		Root:                "/tmp/workspace",
		ServicesDir:         "/tmp/services",
		LocalServicesDir:    "/tmp/workspace/local",
		ReadonlyServicesDir: "/tmp/workspace/readonly",
		EnvDir:              "/tmp/env",
	}

	tests := []struct {
		name     string
		svc      config.Service
		expected string
	}{
		{
			name: "readonly git service",
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "readonly",
					Path:   "services/auth",
				},
			},
			expected: "/tmp/workspace/readonly/services/auth",
		},
		{
			name: "editable git service",
			svc: config.Service{
				Source: config.SourceConfig{
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
