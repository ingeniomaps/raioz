package config

import (
	"testing"
)

func TestIsServiceEnabled(t *testing.T) {
	tests := []struct {
		name     string
		svc      Service
		profile  string
		envVars  map[string]string
		expected bool
	}{
		{
			name: "explicitly enabled",
			svc: Service{
				Enabled: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "explicitly disabled",
			svc: Service{
				Enabled: boolPtr(false),
			},
			expected: false,
		},
		{
			name: "enabled field takes precedence over feature flag",
			svc: Service{
				Enabled: boolPtr(false),
				FeatureFlag: &FeatureFlagConfig{
					Enabled: true,
				},
			},
			expected: false,
		},
		{
			name: "no enabled field, feature flag enabled",
			svc: Service{
				FeatureFlag: &FeatureFlagConfig{
					Enabled: true,
				},
			},
			expected: true,
		},
		{
			name: "no enabled field, feature flag disabled",
			svc: Service{
				FeatureFlag: &FeatureFlagConfig{
					Disabled: true,
				},
			},
			expected: false,
		},
		{
			name:     "no enabled field, no feature flag (default enabled)",
			svc:      Service{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsServiceEnabled(tt.svc, tt.profile, tt.envVars)
			if result != tt.expected {
				t.Errorf("IsServiceEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilterByProfileRespectsEnabled(t *testing.T) {
	deps := &Deps{
		SchemaVersion: "1.0",
		Project: Project{
			Name: "test",
		},
		Services: map[string]Service{
			"enabled-service": {
				Enabled: boolPtr(true),
			},
			"disabled-service": {
				Enabled: boolPtr(false),
			},
			"default-service": {
				// No enabled field, defaults to enabled
			},
		},
		Infra: map[string]InfraEntry{},
	}

	filtered := FilterByProfile(deps, "")

	if _, exists := filtered.Services["disabled-service"]; exists {
		t.Error("disabled-service should not be in filtered services")
	}
	if _, exists := filtered.Services["enabled-service"]; !exists {
		t.Error("enabled-service should be in filtered services")
	}
	if _, exists := filtered.Services["default-service"]; !exists {
		t.Error("default-service should be in filtered services")
	}
}

func TestFilterByFeatureFlagsRespectsEnabled(t *testing.T) {
	deps := &Deps{
		SchemaVersion: "1.0",
		Project: Project{
			Name: "test",
		},
		Services: map[string]Service{
			"enabled-service": {
				Enabled: boolPtr(true),
			},
			"disabled-service": {
				Enabled: boolPtr(false),
			},
			"feature-flag-disabled": {
				FeatureFlag: &FeatureFlagConfig{
					Disabled: true,
				},
			},
		},
		Infra: map[string]InfraEntry{},
	}

	envVars := make(map[string]string)
	filtered, _ := FilterByFeatureFlags(deps, "", envVars)

	if _, exists := filtered.Services["disabled-service"]; exists {
		t.Error("disabled-service should not be in filtered services")
	}
	if _, exists := filtered.Services["feature-flag-disabled"]; exists {
		t.Error("feature-flag-disabled should not be in filtered services")
	}
	if _, exists := filtered.Services["enabled-service"]; !exists {
		t.Error("enabled-service should be in filtered services")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
