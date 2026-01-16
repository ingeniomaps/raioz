package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"raioz/internal/output"
	"raioz/internal/state"

	"github.com/spf13/cobra"
)

var (
	listJSON   bool
	listFilter string
	listStatus string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active projects",
	Long: `List all active projects tracked in the global state.

This command shows:
- Project names
- Last execution time
- Number of active services per project
- Workspace paths

Filters:
- Use --filter to show only projects matching a name pattern
- Use --status to filter by service status (running, stopped)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		globalState, err := state.LoadGlobalState()
		if err != nil {
			return fmt.Errorf("failed to load global state: %w", err)
		}

		// Apply filters
		filteredState := applyFilters(globalState)

		if listJSON {
			// JSON output
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(filteredState); err != nil {
				return fmt.Errorf("failed to encode JSON: %w", err)
			}
			return nil
		}

		// Table output
		if len(filteredState.ActiveProjects) == 0 {
			if listFilter != "" || listStatus != "" {
				output.PrintInfo("No projects match the specified filters")
			} else {
				output.PrintInfo("No active projects found")
			}
			return nil
		}

		output.PrintSectionHeader("Active Projects")

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

				// Show service names if requested or if there are few services
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
	},
}

// applyFilters applies name and status filters to the global state
func applyFilters(globalState *state.GlobalState) *state.GlobalState {
	if listFilter == "" && listStatus == "" {
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
		if listFilter != "" && !strings.Contains(strings.ToLower(projectName), strings.ToLower(listFilter)) {
			continue
		}

		// Apply status filter
		if listStatus != "" {
			hasMatchingStatus := false
			for _, svc := range projectState.Services {
				if strings.EqualFold(svc.Status, listStatus) {
					hasMatchingStatus = true
					break
				}
			}
			if !hasMatchingStatus {
				continue
			}
		}

		// Project matches all filters
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

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	listCmd.Flags().StringVar(&listFilter, "filter", "", "Filter projects by name (partial match, case-insensitive)")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter projects by service status (running, stopped)")
}
