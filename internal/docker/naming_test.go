package docker

import (
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple name",
			parts:    []string{"raioz", "billing", "api"},
			expected: "raioz-billing-api",
			wantErr:  false,
		},
		{
			name:     "uppercase conversion",
			parts:    []string{"Raioz", "BILLING", "API"},
			expected: "raioz-billing-api",
			wantErr:  false,
		},
		{
			name:     "special characters",
			parts:    []string{"raioz", "billing-platform", "api_v2"},
			expected: "raioz-billing-platform-api-v2",
			wantErr:  false,
		},
		{
			name:     "multiple dashes",
			parts:    []string{"raioz", "billing---platform", "api"},
			expected: "raioz-billing-platform-api",
			wantErr:  false,
		},
		{
			name:     "leading/trailing dashes",
			parts:    []string{"raioz", "-billing-", "api"},
			expected: "raioz-billing-api",
			wantErr:  false,
		},
		{
			name:     "empty parts",
			parts:    []string{},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "numbers and dashes",
			parts:    []string{"raioz", "project-1", "service-2"},
			expected: "raioz-project-1-service-2",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeName(tt.parts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("NormalizeName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizeContainerName(t *testing.T) {
	tests := []struct {
		name     string
		project  string
		service  string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple container name",
			project:  "billing",
			service:  "api",
			expected: "raioz-billing-api",
			wantErr:  false,
		},
		{
			name:     "uppercase conversion",
			project:  "BILLING",
			service:  "API",
			expected: "raioz-billing-api",
			wantErr:  false,
		},
		{
			name:     "special characters",
			project:  "billing-platform",
			service:  "api_v2",
			expected: "raioz-billing-platform-api-v2",
			wantErr:  false,
		},
		{
			name:     "long name truncation",
			project:  "very-long-project-name",
			service:  "very-long-service-name-that-exceeds-limit",
			expected: "raioz-very-long-project-name-very-long-service-name-that-ex", // Truncated
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeContainerName(tt.project, tt.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeContainerName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(result) > MaxContainerNameLength {
					t.Errorf("NormalizeContainerName() result length = %d, exceeds max %d", len(result), MaxContainerNameLength)
				}
				// Check that it starts with raioz-
				if !startsWith(result, "raioz-") {
					t.Errorf("NormalizeContainerName() result = %v, should start with 'raioz-'", result)
				}
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		wantErr   bool
	}{
		{
			name:      "valid name",
			input:     "raioz-billing-api",
			maxLength: 63,
			wantErr:   false,
		},
		{
			name:      "empty name",
			input:     "",
			maxLength: 63,
			wantErr:   true,
		},
		{
			name:      "too long",
			input:     "a" + string(make([]byte, 64)),
			maxLength: 63,
			wantErr:   true,
		},
		{
			name:      "uppercase",
			input:     "Raioz-Billing-Api",
			maxLength: 63,
			wantErr:   true,
		},
		{
			name:      "special characters",
			input:     "raioz_billing_api",
			maxLength: 63,
			wantErr:   true,
		},
		{
			name:      "leading dash",
			input:     "-raioz-billing-api",
			maxLength: 63,
			wantErr:   true,
		},
		{
			name:      "trailing dash",
			input:     "raioz-billing-api-",
			maxLength: 63,
			wantErr:   true,
		},
		{
			name:      "consecutive dashes",
			input:     "raioz--billing--api",
			maxLength: 63,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input, tt.maxLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContainerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid container name",
			input:   "raioz-billing-api",
			wantErr: false,
		},
		{
			name:    "too long",
			input:   string(make([]byte, 64)),
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "raioz_billing_api",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContainerName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
