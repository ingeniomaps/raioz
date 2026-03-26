package cmd

import (
	"raioz/internal/app"

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
		deps := app.NewDependencies()
		useCase := app.NewListUseCase(deps)
		return useCase.Execute(app.ListOptions{
			JSONOutput: listJSON,
			Filter:     listFilter,
			Status:     listStatus,
		})
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")
	listCmd.Flags().StringVar(&listFilter, "filter", "", "Filter projects by name (partial match, case-insensitive)")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter projects by service status (running, stopped)")
}
