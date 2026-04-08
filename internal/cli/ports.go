package cli

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List all ports in use by active projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		deps := app.NewDependencies()
		portsUseCase := app.NewPortsUseCase(deps)

		return portsUseCase.Execute(ctx, app.PortsOptions{
			ProjectName: projectName,
		})
	},
}

func init() {
	portsCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (optional)")
}
