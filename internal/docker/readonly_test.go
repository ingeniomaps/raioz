package docker

import (
	"testing"

	"raioz/internal/config"
	"raioz/internal/git"
)

func TestApplyReadonlyToVolumes(t *testing.T) {
	tests := []struct {
		name     string
		volumes  []string
		svc      config.Service
		expected []string
	}{
		{
			name:    "readonly service with bind mount",
			volumes: []string{"./service:/app"},
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "readonly",
				},
			},
			expected: []string{"./service:/app:ro"},
		},
		{
			name:    "editable service with bind mount",
			volumes: []string{"./service:/app"},
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "editable",
				},
			},
			expected: []string{"./service:/app"},
		},
		{
			name:    "readonly service with named volume",
			volumes: []string{"data:/data"},
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "readonly",
				},
			},
			expected: []string{"data:/data"},
		},
		{
			name:    "readonly service with explicit :rw",
			volumes: []string{"./service:/app:rw"},
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "readonly",
				},
			},
			expected: []string{"./service:/app:ro"},
		},
		{
			name:    "readonly service with already :ro",
			volumes: []string{"./service:/app:ro"},
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "readonly",
				},
			},
			expected: []string{"./service:/app:ro"},
		},
		{
			name:    "image service (not applicable)",
			volumes: []string{"./service:/app"},
			svc: config.Service{
				Source: config.SourceConfig{
					Kind: "image",
				},
			},
			expected: []string{"./service:/app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyReadonlyToVolumes(tt.volumes, tt.svc)
			if len(result) != len(tt.expected) {
				t.Errorf("ApplyReadonlyToVolumes() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("ApplyReadonlyToVolumes()[%d] = %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetVolumeMountMode(t *testing.T) {
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
			expected: ":ro",
		},
		{
			name: "editable git service",
			svc: config.Service{
				Source: config.SourceConfig{
					Kind:   "git",
					Access: "editable",
				},
			},
			expected: "",
		},
		{
			name: "image service",
			svc: config.Service{
				Source: config.SourceConfig{
					Kind: "image",
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetVolumeMountMode(tt.svc)
			if result != tt.expected {
				t.Errorf("GetVolumeMountMode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test that readonly services are detected correctly
func TestReadonlyServiceDetection(t *testing.T) {
	readonlySvc := config.Service{
		Source: config.SourceConfig{
			Kind:   "git",
			Access: "readonly",
		},
	}

	if !git.IsReadonly(readonlySvc.Source) {
		t.Error("Expected readonly service to be detected as readonly")
	}

	editableSvc := config.Service{
		Source: config.SourceConfig{
			Kind:   "git",
			Access: "editable",
		},
	}

	if git.IsReadonly(editableSvc.Source) {
		t.Error("Expected editable service to not be detected as readonly")
	}
}
