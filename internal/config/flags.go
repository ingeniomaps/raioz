package config

import (
	"os"
)

// MockConfig defines configuration for a mock service
type MockConfig struct {
	Enabled bool     `json:"enabled,omitempty"` // Whether to use mock
	Image   string   `json:"image,omitempty"`   // Docker image for mock
	Tag     string   `json:"tag,omitempty"`     // Tag for mock image
	Ports   []string `json:"ports,omitempty"`   // Ports for mock (overrides service ports)
	Env     []string `json:"env,omitempty"`     // Env vars for mock
}

// FeatureFlagConfig defines feature flag configuration
type FeatureFlagConfig struct {
	Enabled  bool     `json:"enabled,omitempty"`  // Direct enable/disable
	EnvVar   string   `json:"envVar,omitempty"`   // Environment variable to check
	EnvValue string   `json:"envValue,omitempty"` // Required value for envVar
	Profiles []string `json:"profiles,omitempty"` // Enabled for specific profiles
	Disabled bool     `json:"disabled,omitempty"` // Direct disable (takes precedence)
}

// IsEnabled checks if a feature flag is enabled based on configuration and context
func (f *FeatureFlagConfig) IsEnabled(profile string, envVars map[string]string) bool {
	// Disabled takes precedence
	if f.Disabled {
		return false
	}

	// Check direct enable flag
	if f.Enabled {
		// But verify profile if specified
		if len(f.Profiles) > 0 {
			for _, p := range f.Profiles {
				if p == profile {
					return true
				}
			}
			return false // Enabled but profile doesn't match
		}
		return true
	}

	// Check environment variable
	if f.EnvVar != "" {
		envValue, exists := envVars[f.EnvVar]
		if !exists {
			// Try to get from actual environment
			envValue = os.Getenv(f.EnvVar)
		}
		if f.EnvValue != "" {
			return envValue == f.EnvValue
		}
		return envValue != "" && envValue != "false" && envValue != "0"
	}

	// Check profile-based enable
	if len(f.Profiles) > 0 {
		for _, p := range f.Profiles {
			if p == profile {
				return true
			}
		}
	}

	// Default: enabled if no configuration (backward compatibility)
	return true
}

// ShouldUseMock checks if a service should use mock instead of real service
func ShouldUseMock(
	svc Service,
	profile string,
	envVars map[string]string,
) bool {
	if svc.Mock == nil {
		return false
	}
	return svc.Mock.Enabled && svc.Mock.Image != ""
}

// IsServiceEnabled checks if a service is enabled
// Checks both the explicit 'enabled' field and feature flags
// Priority: enabled field > feature flags
func IsServiceEnabled(
	svc Service,
	profile string,
	envVars map[string]string,
) bool {
	// Explicit enabled field takes precedence
	if svc.Enabled != nil {
		return *svc.Enabled
	}

	// Then check feature flags
	if svc.FeatureFlag == nil {
		return true // Default: enabled if no feature flag config
	}
	return svc.FeatureFlag.IsEnabled(profile, envVars)
}

// ShouldEnableService is an alias for IsServiceEnabled for backward compatibility
func ShouldEnableService(
	svc Service,
	profile string,
	envVars map[string]string,
) bool {
	return IsServiceEnabled(svc, profile, envVars)
}
