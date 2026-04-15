package validate

import (
	"testing"

	"raioz/internal/config"
)

func TestValidateTransitiveDependencies(t *testing.T) {
	tests := []struct {
		name       string
		deps       *config.Deps
		wantIssues bool
		wantError  bool
	}{
		{
			name: "valid transitive dependencies",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
					"service2": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service3"},
						},
					},
					"service3": {},
				},
			},
			wantIssues: false,
			wantError:  false,
		},
		{
			name: "missing transitive dependency",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
					"service2": {
						Docker: &config.DockerConfig{
							DependsOn: []string{"missing-service"},
						},
					},
				},
			},
			wantIssues: true,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := validateTransitiveDependencies(tt.deps)
			if (err != nil) != tt.wantError {
				t.Errorf("validateTransitiveDependencies() error = %v, wantError %v", err, tt.wantError)
			}
			hasIssues := len(issues) > 0
			if hasIssues != tt.wantIssues {
				t.Errorf("validateTransitiveDependencies() issues = %v, wantIssues %v", hasIssues, tt.wantIssues)
			}
			if tt.wantIssues {
				hasMissingDep := false
				for _, issue := range issues {
					if issue.Type == "missing_dependency" {
						hasMissingDep = true
						break
					}
				}
				if !hasMissingDep {
					t.Error("validateTransitiveDependencies() should return missing_dependency issues")
				}
			}
		})
	}
}

func TestExtractMajorVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"v1.2.3", "1"},
		{"1.2.3", "1"},
		{"2.0.0", "2"},
		{"main", "main"},
		{"develop", "develop"},
		{"feature-1.0", "feature"},
		{"v10.20.30", "10"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := extractMajorVersion(tt.version)
			if got != tt.want {
				t.Errorf("extractMajorVersion(%s) = %s, want %s", tt.version, got, tt.want)
			}
		})
	}
}

func TestValidateVersionCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		deps        *config.Deps
		wantIssues  bool
		wantVersion bool
	}{
		{
			name: "compatible versions (same major)",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Source: config.SourceConfig{
							Kind: "image",
							Tag:  "v1.2.3",
						},
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
					"service2": {
						Source: config.SourceConfig{
							Kind: "image",
							Tag:  "v1.3.0",
						},
					},
				},
			},
			wantIssues:  false,
			wantVersion: true,
		},
		{
			name: "incompatible versions (different major)",
			deps: &config.Deps{
				Services: map[string]config.Service{
					"service1": {
						Source: config.SourceConfig{
							Kind: "image",
							Tag:  "v1.2.3",
						},
						Docker: &config.DockerConfig{
							DependsOn: []string{"service2"},
						},
					},
					"service2": {
						Source: config.SourceConfig{
							Kind: "image",
							Tag:  "v2.0.0",
						},
					},
				},
			},
			wantIssues:  true,
			wantVersion: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := validateVersionCompatibility(tt.deps)
			if err != nil {
				t.Errorf("validateVersionCompatibility() error = %v", err)
				return
			}

			hasVersionIssues := false
			for _, issue := range issues {
				if issue.Type == "version_mismatch" {
					hasVersionIssues = true
					break
				}
			}

			if hasVersionIssues != tt.wantIssues {
				t.Errorf(
					"validateVersionCompatibility() version issues = %v, want %v",
					hasVersionIssues, tt.wantIssues,
				)
			}
		})
	}
}

func TestHasCompatibilityErrors(t *testing.T) {
	tests := []struct {
		name   string
		issues []CompatibilityIssue
		want   bool
	}{
		{
			name:   "no issues",
			issues: []CompatibilityIssue{},
			want:   false,
		},
		{
			name: "has errors",
			issues: []CompatibilityIssue{
				{
					Severity: "error",
				},
			},
			want: true,
		},
		{
			name: "only warnings",
			issues: []CompatibilityIssue{
				{
					Severity: "warning",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasCompatibilityErrors(tt.issues)
			if got != tt.want {
				t.Errorf("HasCompatibilityErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCompatibilityIssues(t *testing.T) {
	issues := []CompatibilityIssue{
		{
			Type:        "missing_dependency",
			Severity:    "error",
			Service:     "service1",
			Description: "Missing dependency",
			Suggestion:  "Add it",
		},
		{
			Type:        "version_mismatch",
			Severity:    "warning",
			Service:     "service2",
			Description: "Version mismatch",
			Suggestion:  "Fix versions",
		},
	}

	result := FormatCompatibilityIssues(issues)
	if result == "" {
		t.Error("FormatCompatibilityIssues() should return formatted string")
	}

	if !contains(result, "Errors:") {
		t.Error("FormatCompatibilityIssues() should contain 'Errors:' section")
	}

	if !contains(result, "Warnings:") {
		t.Error("FormatCompatibilityIssues() should contain 'Warnings:' section")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
