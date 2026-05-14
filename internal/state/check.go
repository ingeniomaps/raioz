package state

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/domain/models"
	exectimeout "raioz/internal/exec"
	"raioz/internal/git"
	"raioz/internal/workspace"
)

// AlignmentIssue lives in internal/domain/models; alias kept for callers (ADR-009).
type AlignmentIssue = models.AlignmentIssue

// CheckAlignment checks if the current state aligns with the configuration.
//
// ADR-011 Phase 3: the function used to load the legacy .state.json
// snapshot and diff it against currentDeps to surface config drift
// since the last `raioz up`. Without that snapshot only the git
// branch-drift case still has signal — it compares the on-disk repo's
// branch against `currentDeps.Services[name].Source.Branch`, which
// doesn't depend on state at all. Image-tag drift required the
// snapshot's old tag, so that branch is dropped.
func CheckAlignment(ws *workspace.Workspace, currentDeps *models.Deps) ([]AlignmentIssue, error) {
	var issues []AlignmentIssue

	for name, svc := range currentDeps.Services {
		if svc.Source.Kind != "git" {
			continue
		}
		repoPath := workspace.GetServicePath(ws, name, svc)
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			continue
		}
		ctx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
		defer cancel()
		hasDrift, currentBranch, err := git.DetectBranchDrift(ctx, repoPath, svc.Source.Branch)
		if err != nil || !hasDrift {
			continue
		}
		issues = append(issues, AlignmentIssue{
			Type:     "branch_drift",
			Severity: "info",
			Service:  name,
			Description: fmt.Sprintf(
				"Branch drift: repository is on '%s', but config expects '%s'",
				currentBranch, svc.Source.Branch,
			),
			Suggestion: fmt.Sprintf(
				"Repository was manually switched. Run 'raioz up' to switch to '%s' or update raioz.yaml",
				svc.Source.Branch,
			),
		})
	}

	return issues, nil
}

// FormatIssues formats alignment issues for display
func FormatIssues(issues []AlignmentIssue) string {
	if len(issues) == 0 {
		return "✔ All checks passed - configuration is aligned with state"
	}

	var result string
	result += fmt.Sprintf("⚠️  Found %d alignment issue(s):\n\n", len(issues))

	// Group by severity
	critical := []AlignmentIssue{}
	warning := []AlignmentIssue{}
	info := []AlignmentIssue{}

	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			critical = append(critical, issue)
		case "warning":
			warning = append(warning, issue)
		case "info":
			info = append(info, issue)
		}
	}

	// Display critical first
	if len(critical) > 0 {
		result += "🔴 Critical Issues:\n"
		for _, issue := range critical {
			result += fmt.Sprintf("  • [%s] %s\n", issue.Service, issue.Description)
			if issue.Suggestion != "" {
				result += fmt.Sprintf("    → %s\n", issue.Suggestion)
			}
		}
		result += "\n"
	}

	// Then warnings
	if len(warning) > 0 {
		result += "🟡 Warnings:\n"
		for _, issue := range warning {
			result += fmt.Sprintf("  • [%s] %s\n", issue.Service, issue.Description)
			if issue.Suggestion != "" {
				result += fmt.Sprintf("    → %s\n", issue.Suggestion)
			}
		}
		result += "\n"
	}

	// Finally info
	if len(info) > 0 {
		result += "ℹ️  Info:\n"
		for _, issue := range info {
			result += fmt.Sprintf("  • [%s] %s\n", issue.Service, issue.Description)
			if issue.Suggestion != "" {
				result += fmt.Sprintf("    → %s\n", issue.Suggestion)
			}
		}
		result += "\n"
	}

	return result
}

// HasCriticalIssues checks if there are any critical alignment issues
func HasCriticalIssues(issues []AlignmentIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "critical" {
			return true
		}
	}
	return false
}

// HasWarningOrCriticalIssues checks if there are warnings or critical issues
func HasWarningOrCriticalIssues(issues []AlignmentIssue) bool {
	for _, issue := range issues {
		if issue.Severity == "critical" || issue.Severity == "warning" {
			return true
		}
	}
	return false
}
