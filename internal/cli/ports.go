package cli

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var portsConflicting bool

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
			ConfigPath:  ResolveConfigPath(configPath),
			Conflicting: portsConflicting,
		})
	},
}

func init() {
	portsCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (optional)")
	portsCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file (used with --conflicting)")
	portsCmd.Flags().BoolVar(&portsConflicting, "conflicting", false,
		"Show sibling raioz projects holding host ports declared in this project's raioz.yaml (read-only)")
}
