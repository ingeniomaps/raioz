package cmd

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
	logsAll    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [service...]",
	Short: "View logs for services",
	Long:  "View logs for one or more services.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		configPath = ResolveConfigPath(configPath)

		deps := app.NewDependencies()
		logsUseCase := app.NewLogsUseCase(deps)

		return logsUseCase.Execute(ctx, app.LogsOptions{
			ConfigPath:  configPath,
			ProjectName: projectName,
			Follow:      logsFollow,
			Tail:        logsTail,
			All:         logsAll,
			Services:    args,
		})
	},
}

func init() {
	logsCmd.Flags().StringVarP(&configPath, "file", "f", ".raioz.json", "Path to config file")
	logsCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
	logsCmd.Flags().BoolVar(&logsFollow, "follow", false, "Follow log output")
	logsCmd.Flags().IntVar(&logsTail, "tail", 0, "Number of lines to show from the end of logs (0 = all)")
	logsCmd.Flags().BoolVar(&logsAll, "all", false, "Show logs for all services")
}
