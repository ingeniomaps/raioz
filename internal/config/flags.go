package config

import "raioz/internal/domain/models"

// MockConfig and FeatureFlagConfig live in internal/domain/models. The
// aliases keep `models.MockConfig` / `models.FeatureFlagConfig` callers
// compiling (see ADR-009 / issue 023).
type (
	MockConfig        = models.MockConfig
	FeatureFlagConfig = models.FeatureFlagConfig
)

// ShouldUseMock checks if a service should use mock instead of real service.
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

// IsServiceEnabled checks if a service is enabled.
// Checks both the explicit 'enabled' field and feature flags.
// Priority: enabled field > feature flags.
func IsServiceEnabled(
	svc Service,
	profile string,
	envVars map[string]string,
) bool {
	if svc.Enabled != nil {
		return *svc.Enabled
	}
	if svc.FeatureFlag == nil {
		return true
	}
	return svc.FeatureFlag.IsEnabled(profile, envVars)
}

// ShouldEnableService is an alias for IsServiceEnabled for backward compatibility.
func ShouldEnableService(
	svc Service,
	profile string,
	envVars map[string]string,
) bool {
	return IsServiceEnabled(svc, profile, envVars)
}
