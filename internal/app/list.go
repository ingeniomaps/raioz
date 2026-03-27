package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/state"
)

// ListOptions holds options for the list use case
type ListOptions struct {
	JSONOutput bool
	Filter     string
	Status     string
}

// ListUseCase handles listing active projects
type ListUseCase struct {
	deps *Dependencies
}

// NewListUseCase creates a new ListUseCase
func NewListUseCase(deps *Dependencies) *ListUseCase {
	return &ListUseCase{deps: deps}
}

// Execute runs the list use case
func (uc *ListUseCase) Execute(opts ListOptions) error {
	globalState, err := uc.deps.StateManager.LoadGlobalState()
	if err != nil {
		return fmt.Errorf("failed to load global state: %w", err)
	}

	// Apply filters
	filteredState := uc.applyFilters(globalState, opts)

	if opts.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(filteredState); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
		return nil
	}

	// Table output
	if len(filteredState.ActiveProjects) == 0 {
		if opts.Filter != "" || opts.Status != "" {
			output.PrintInfo(i18n.T("output.no_projects_match_filters"))
		} else {
			output.PrintInfo(i18n.T("output.no_active_projects"))
		}
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.active_projects_header"))

	for i, projectName := range filteredState.ActiveProjects {
		projectState, exists := filteredState.Projects[projectName]
		if !exists {
			continue
		}

		if i > 0 {
			fmt.Println()
		}

		output.PrintSubsection(projectState.Name)
		output.PrintKeyValue("Workspace", projectState.Workspace)
		output.PrintKeyValue("Last Execution", formatTime(projectState.LastExecution))
		output.PrintKeyValue("Active Services", fmt.Sprintf("%d", len(projectState.Services)))

		// Show service summary
		if len(projectState.Services) > 0 {
			runningCount := 0
			for _, svc := range projectState.Services {
				if svc.Status == "running" {
					runningCount++
				}
			}
			output.PrintKeyValue("Running", fmt.Sprintf("%d/%d", runningCount, len(projectState.Services)))

			if len(projectState.Services) <= 5 {
				serviceNames := make([]string, 0, len(projectState.Services))
				for _, svc := range projectState.Services {
					statusIndicator := "●"
					if svc.Status == "running" {
						statusIndicator = "✓"
					}
					serviceNames = append(serviceNames, fmt.Sprintf("%s %s", statusIndicator, svc.Name))
				}
				output.PrintKeyValue("Services", strings.Join(serviceNames, ", "))
			}
		}
	}

	return nil
}

// applyFilters applies name and status filters to the global state
func (uc *ListUseCase) applyFilters(globalState *state.GlobalState, opts ListOptions) *state.GlobalState {
	if opts.Filter == "" && opts.Status == "" {
		return globalState
	}

	filtered := &state.GlobalState{
		ActiveProjects: []string{},
		Projects:       make(map[string]state.ProjectState),
	}

	for _, projectName := range globalState.ActiveProjects {
		projectState, exists := globalState.Projects[projectName]
		if !exists {
			continue
		}

		// Apply name filter
		if opts.Filter != "" && !strings.Contains(strings.ToLower(projectName), strings.ToLower(opts.Filter)) {
			continue
		}

		// Apply status filter
		if opts.Status != "" {
			hasMatchingStatus := false
			for _, svc := range projectState.Services {
				if strings.EqualFold(svc.Status, opts.Status) {
					hasMatchingStatus = true
					break
				}
			}
			if !hasMatchingStatus {
				continue
			}
		}

		filtered.ActiveProjects = append(filtered.ActiveProjects, projectName)
		filtered.Projects[projectName] = projectState
	}

	return filtered
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		minutes := int(diff.Minutes())
		return fmt.Sprintf("%d minute(s) ago", minutes)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hour(s) ago", hours)
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d day(s) ago", days)
	}

	return t.Format("2006-01-02 15:04:05")
}
