package validate

import (
	"fmt"
	"strings"

	"raioz/internal/config"
)

// CompatibilityIssue represents a compatibility issue between services
type CompatibilityIssue struct {
	Type        string // "version_mismatch", "missing_dependency"
	Severity    string // "warning", "error"
	Service     string
	Description string
	Suggestion  string
}

// ValidateCompatibility validates service compatibility and transitive dependencies
func ValidateCompatibility(deps *config.Deps) ([]CompatibilityIssue, error) {
	var issues []CompatibilityIssue

	// Validate transitive dependencies
	transitiveIssues, err := validateTransitiveDependencies(deps)
	if err != nil {
		return nil, fmt.Errorf("failed to validate transitive dependencies: %w", err)
	}
	issues = append(issues, transitiveIssues...)

	// Validate version compatibility for communicating services
	versionIssues, err := validateVersionCompatibility(deps)
	if err != nil {
		return nil, fmt.Errorf("failed to validate version compatibility: %w", err)
	}
	issues = append(issues, versionIssues...)

	return issues, nil
}

// validateTransitiveDependencies checks that all transitive dependencies are defined
func validateTransitiveDependencies(deps *config.Deps) ([]CompatibilityIssue, error) {
	var issues []CompatibilityIssue

	// Collect all defined service and infra names
	definedNames := make(map[string]bool)
	for name := range deps.Services {
		definedNames[name] = true
	}
	for name := range deps.Infra {
		definedNames[name] = true
	}

	// Track all referenced dependencies (direct and transitive)
	referencedNames := make(map[string]bool)

	// Function to collect transitive dependencies recursively
	var collectTransitive func(serviceName string, visited map[string]bool) error
	collectTransitive = func(serviceName string, visited map[string]bool) error {
		// Prevent infinite loops
		if visited[serviceName] {
			return nil
		}
		visited[serviceName] = true

		// Check if service exists
		svc, exists := deps.Services[serviceName]
		if !exists {
			// Check if it's an infra
			_, infraExists := deps.Infra[serviceName]
			if !infraExists {
				return fmt.Errorf("service/infra '%s' is referenced but not defined", serviceName)
			}
			return nil // Infra doesn't have dependencies
		}

		// Collect direct dependencies (service-level and docker-level)
		for _, depName := range svc.GetDependsOn() {
			referencedNames[depName] = true

			if !definedNames[depName] {
				return fmt.Errorf("service/infra '%s' is referenced but not defined", depName)
			}

			newVisited := make(map[string]bool)
			for k, v := range visited {
				newVisited[k] = v
			}
			if err := collectTransitive(depName, newVisited); err != nil {
				return err
			}
		}

		return nil
	}

	// Collect all transitive dependencies for each service
	for name := range deps.Services {
		visited := make(map[string]bool)
		if err := collectTransitive(name, visited); err != nil {
			issues = append(issues, CompatibilityIssue{
				Type:        "missing_dependency",
				Severity:    "error",
				Service:     name,
				Description: err.Error(),
				Suggestion:  fmt.Sprintf("Add missing service/infra '%s' to .raioz.json", name),
			})
		}
	}

	// Check if all referenced dependencies are defined
	for refName := range referencedNames {
		if !definedNames[refName] {
			issues = append(issues, CompatibilityIssue{
				Type:        "missing_dependency",
				Severity:    "error",
				Service:     "",
				Description: fmt.Sprintf("Referenced dependency '%s' is not defined", refName),
				Suggestion:  fmt.Sprintf("Add service/infra '%s' to .raioz.json", refName),
			})
		}
	}

	return issues, nil
}

// validateVersionCompatibility checks version compatibility for services that communicate
func validateVersionCompatibility(deps *config.Deps) ([]CompatibilityIssue, error) {
	var issues []CompatibilityIssue

	// Build communication graph (services that depend on each other)
	communicationGraph := make(map[string][]string)
	for name, svc := range deps.Services {
		communicationGraph[name] = svc.GetDependsOn()
	}

	// Extract version information from tags/branches
	getVersion := func(svc config.Service) string {
		if svc.Source.Kind == "git" {
			return svc.Source.Branch // Use branch as version indicator
		} else if svc.Source.Kind == "image" {
			return svc.Source.Tag // Use tag as version indicator
		}
		return ""
	}

	// Check version compatibility for communicating services
	for serviceName, dependencies := range communicationGraph {
		serviceVersion := getVersion(deps.Services[serviceName])
		if serviceVersion == "" {
			continue // No version info available
		}

		// Normalize version for comparison (extract major version if possible)
		serviceMajor := extractMajorVersion(serviceVersion)

		// Check compatibility with dependencies
		for _, depName := range dependencies {
			depSvc, exists := deps.Services[depName]
			if !exists {
				// Dependency might be infra, skip version check
				continue
			}

			depVersion := getVersion(depSvc)
			if depVersion == "" {
				continue // No version info available
			}

			depMajor := extractMajorVersion(depVersion)

			// Compare versions (simple heuristic: same major version = compatible)
			if serviceMajor != "" && depMajor != "" && serviceMajor != depMajor {
				// Different major versions - potential incompatibility
				issues = append(issues, CompatibilityIssue{
					Type:     "version_mismatch",
					Severity: "warning",
					Service:  serviceName,
					Description: fmt.Sprintf(
						"Version mismatch with dependency '%s': %s vs %s (different major versions)",
						depName, serviceVersion, depVersion,
					),
					Suggestion: fmt.Sprintf(
						"Consider using compatible versions. Service '%s' uses %s, dependency '%s' uses %s",
						serviceName, serviceVersion, depName, depVersion,
					),
				})
			}
		}
	}

	return issues, nil
}

// extractMajorVersion extracts major version from a version string
// Supports formats like: v1.2.3, 1.2.3, main, develop, feature-1.0
func extractMajorVersion(version string) string {
	if version == "" {
		return ""
	}

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split by '.' or '-' to find numeric major version
	parts := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '-'
	})

	if len(parts) > 0 {
		// Check if first part starts with a digit (numeric version)
		firstPart := parts[0]
		if len(firstPart) > 0 && firstPart[0] >= '0' && firstPart[0] <= '9' {
			// Extract first digit(s) as major version
			var major strings.Builder
			for _, char := range firstPart {
				if char >= '0' && char <= '9' {
					major.WriteRune(char)
				} else {
					break
				}
			}
			if major.Len() > 0 {
				return major.String()
			}
		}

		// If first part doesn't start with digit, use it as identifier
		// This handles branch names like "main", "develop", "feature-xyz", "feature-1.0"
		return firstPart
	}

	return ""
}

// FormatCompatibilityIssues formats compatibility issues for display
func FormatCompatibilityIssues(issues []CompatibilityIssue) string {
	if len(issues) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("⚠️  Found %d compatibility issue(s):\n\n", len(issues)))

	// Group by severity
	errors := []CompatibilityIssue{}
	warnings := []CompatibilityIssue{}

	for _, issue := range issues {
		if issue.Severity == "error" {
			errors = append(errors, issue)
		} else {
			warnings = append(warnings, issue)
		}
	}

	// Display errors first
	if len(errors) > 0 {
		result.WriteString("🔴 Errors:\n")
		for _, issue := range errors {
			result.WriteString(fmt.Sprintf("  • [%s] %s\n", issue.Service, issue.Description))
			if issue.Suggestion != "" {
				result.WriteString(fmt.Sprintf("    → %s\n", issue.Suggestion))
			}
		}
		result.WriteString("\n")
	}

	// Then warnings
	if len(warnings) > 0 {
		result.WriteString("🟡 Warnings:\n")
		for _, issue := range warnings {
			result.WriteString(fmt.Sprintf("  • [%s] %s\n", issue.Service, issue.Description))
			if issue.Suggestion != "" {
				result.WriteString(fmt.Sprintf("    → %s\n", issue.Suggestion))
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

// HasCompatibilityErrors checks if there are any compatibility errors
func HasCompatibilityErrors(issues []CompatibilityIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}
