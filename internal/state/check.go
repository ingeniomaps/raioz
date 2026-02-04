package state

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/git"
	"raioz/internal/workspace"
)

// AlignmentIssue represents a detected alignment issue
type AlignmentIssue struct {
	Type        string // "branch_drift", "config_change", "port_conflict", "env_change"
	Severity    string // "info", "warning", "critical"
	Service     string // Service name (if applicable)
	Description string // Human-readable description
	Suggestion  string // Suggested action
}

// CheckAlignment checks if the current state aligns with the configuration
// Returns a list of alignment issues
func CheckAlignment(ws *workspace.Workspace, currentDeps *config.Deps) ([]AlignmentIssue, error) {
	var issues []AlignmentIssue

	// Load saved state
	savedDeps, err := Load(ws)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if savedDeps == nil {
		// No saved state, no alignment issues
		return issues, nil
	}

	// Check config changes
	configChanges, err := CompareDeps(savedDeps, currentDeps)
	if err != nil {
		return nil, fmt.Errorf("failed to compare configs: %w", err)
	}

	// Convert config changes to alignment issues
	for _, change := range configChanges {
		severity := "warning"
		suggestion := "Run 'raioz up' to apply changes"

		// Critical changes
		if change.Field == "docker.ports" || change.Field == "ports" {
			severity = "critical"
			suggestion = "Port changes detected. Run 'raioz down' then 'raioz up' to apply"
		} else if change.Field == "dependsOn" || change.Field == "docker.dependsOn" || change.Field == "removed" {
			severity = "critical"
			suggestion = "Dependencies changed. Run 'raioz down' then 'raioz up' to apply"
		} else if change.Field == "added" {
			severity = "warning"
			suggestion = "New service/infra added. Run 'raioz up' to start it"
		}

		// Branch and tag changes are handled separately for drift detection
		if change.Field == "source.branch" || change.Field == "source.tag" ||
			change.Field == "image" || change.Field == "tag" {
			// Skip here, will be handled in drift detection
			continue
		}

		issues = append(issues, AlignmentIssue{
			Type:     "config_change",
			Severity: severity,
			Service:  change.Name,
			Description: fmt.Sprintf(
				"%s.%s changed: %s -> %s",
				change.Name, change.Field, change.OldValue, change.NewValue,
			),
			Suggestion: suggestion,
		})
	}

	// Check branch drift for git-based services
	for name, svc := range currentDeps.Services {
		if svc.Source.Kind != "git" {
			continue
		}

		// Use GetServicePath to get correct path based on access mode
		repoPath := workspace.GetServicePath(ws, name, svc)

		// Check if repo exists (check for .git directory)
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// Check for branch drift (manual branch changes)
			ctx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
			defer cancel()
			hasDrift, currentBranch, err := git.DetectBranchDrift(ctx, repoPath, svc.Source.Branch)
			if err != nil {
				// Repo might not exist yet or not be a git repo
				continue
			}

			if hasDrift {
				// Check if branch also changed in config
				oldSvc, exists := savedDeps.Services[name]
				if exists && oldSvc.Source.Branch == svc.Source.Branch {
					// Branch drift detected (manual change, not in config)
					issues = append(issues, AlignmentIssue{
						Type:     "branch_drift",
						Severity: "info",
						Service:  name,
						Description: fmt.Sprintf(
							"Branch drift: repository is on '%s', but config expects '%s'",
							currentBranch, svc.Source.Branch,
						),
						Suggestion: fmt.Sprintf(
							"Repository was manually switched. Run 'raioz up' to switch to '%s' or update .raioz.json",
							svc.Source.Branch,
						),
					})
				}
				// If branch changed in config, it's already handled in config changes above
			}
		}
	}

	// Check for tag/image changes (if image-based)
	for name, svc := range currentDeps.Services {
		if svc.Source.Kind != "image" {
			continue
		}

		oldSvc, exists := savedDeps.Services[name]
		if !exists {
			continue
		}

		// Check if tag changed
		if oldSvc.Source.Tag != svc.Source.Tag {
			issues = append(issues, AlignmentIssue{
				Type:     "config_change",
				Severity: "warning",
				Service:  name,
				Description: fmt.Sprintf(
					"Image tag changed: %s -> %s",
					oldSvc.Source.Tag, svc.Source.Tag,
				),
				Suggestion: "Run 'raioz up' to pull and use the new tag",
			})
		}
	}

	// Check for infra image/tag changes
	for name, infra := range currentDeps.Infra {
		oldInfra, exists := savedDeps.Infra[name]
		if !exists {
			continue
		}

		if oldInfra.Tag != infra.Tag {
			issues = append(issues, AlignmentIssue{
				Type:     "config_change",
				Severity: "warning",
				Service:  name,
				Description: fmt.Sprintf(
					"Infra tag changed: %s -> %s",
					oldInfra.Tag, infra.Tag,
				),
				Suggestion: "Run 'raioz up' to pull and use the new tag",
			})
		}
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
