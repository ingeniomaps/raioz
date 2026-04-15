package state

import (
	"testing"
)

func TestFormatIssues(t *testing.T) {
	tests := []struct {
		name   string
		issues []AlignmentIssue
		want   string
	}{
		{
			name:   "no issues",
			issues: []AlignmentIssue{},
			want:   "✔ All checks passed - configuration is aligned with state",
		},
		{
			name: "single critical issue",
			issues: []AlignmentIssue{
				{
					Type:        "config_change",
					Severity:    "critical",
					Service:     "service1",
					Description: "Port changed",
					Suggestion:  "Run 'raioz up'",
				},
			},
			want: "🔴 Critical Issues:",
		},
		{
			name: "branch drift info",
			issues: []AlignmentIssue{
				{
					Type:        "branch_drift",
					Severity:    "info",
					Service:     "service1",
					Description: "Branch drift detected",
					Suggestion:  "Run 'raioz up'",
				},
			},
			want: "ℹ️  Info:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatIssues(tt.issues)
			if tt.name == "no issues" && !contains(result, "All checks passed") {
				t.Errorf("FormatIssues() should contain 'All checks passed' for no issues")
			}
			if tt.name != "no issues" && !contains(result, tt.want) {
				t.Errorf("FormatIssues() should contain '%s'", tt.want)
			}
		})
	}
}

func TestHasCriticalIssues(t *testing.T) {
	tests := []struct {
		name   string
		issues []AlignmentIssue
		want   bool
	}{
		{
			name:   "no issues",
			issues: []AlignmentIssue{},
			want:   false,
		},
		{
			name: "has critical",
			issues: []AlignmentIssue{
				{
					Severity: "critical",
				},
			},
			want: true,
		},
		{
			name: "only warning",
			issues: []AlignmentIssue{
				{
					Severity: "warning",
				},
			},
			want: false,
		},
		{
			name: "only info",
			issues: []AlignmentIssue{
				{
					Severity: "info",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasCriticalIssues(tt.issues)
			if got != tt.want {
				t.Errorf("HasCriticalIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasWarningOrCriticalIssues(t *testing.T) {
	tests := []struct {
		name   string
		issues []AlignmentIssue
		want   bool
	}{
		{
			name:   "no issues",
			issues: []AlignmentIssue{},
			want:   false,
		},
		{
			name: "has critical",
			issues: []AlignmentIssue{
				{
					Severity: "critical",
				},
			},
			want: true,
		},
		{
			name: "has warning",
			issues: []AlignmentIssue{
				{
					Severity: "warning",
				},
			},
			want: true,
		},
		{
			name: "only info",
			issues: []AlignmentIssue{
				{
					Severity: "info",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasWarningOrCriticalIssues(tt.issues)
			if got != tt.want {
				t.Errorf("HasWarningOrCriticalIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlignmentIssue(t *testing.T) {
	issue := AlignmentIssue{
		Type:        "branch_drift",
		Severity:    "info",
		Service:     "test-service",
		Description: "Test description",
		Suggestion:  "Test suggestion",
	}

	if issue.Type != "branch_drift" {
		t.Errorf("AlignmentIssue.Type = %s, want branch_drift", issue.Type)
	}

	if issue.Severity != "info" {
		t.Errorf("AlignmentIssue.Severity = %s, want info", issue.Severity)
	}

	if issue.Service != "test-service" {
		t.Errorf("AlignmentIssue.Service = %s, want test-service", issue.Service)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
