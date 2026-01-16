package git

import (
	"testing"

	"raioz/internal/config"
)

func TestIsReadonly(t *testing.T) {
	tests := []struct {
		name     string
		source   config.SourceConfig
		expected bool
	}{
		{
			name: "readonly access",
			source: config.SourceConfig{
				Kind:   "git",
				Access: "readonly",
			},
			expected: true,
		},
		{
			name: "editable access",
			source: config.SourceConfig{
				Kind:   "git",
				Access: "editable",
			},
			expected: false,
		},
		{
			name: "empty access (default editable)",
			source: config.SourceConfig{
				Kind: "git",
			},
			expected: false,
		},
		{
			name: "image source (not applicable)",
			source: config.SourceConfig{
				Kind:   "image",
				Access: "readonly",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReadonly(tt.source)
			if result != tt.expected {
				t.Errorf("IsReadonly() = %v, want %v", result, tt.expected)
			}
		})
	}
}
