package cli

import (
	"os"

	"raioz/internal/app"
	"raioz/internal/graph"

	"github.com/spf13/cobra"
)

var graphFormat string
var graphConfigPath string

var graphCmd = &cobra.Command{
	Use:          "graph",
	Short:        "Visualize service dependency graph",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		configPath := ResolveConfigPath(graphConfigPath)

		cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
		if err != nil {
			return err
		}

		g := graph.Build(cfgDeps)

		switch graphFormat {
		case "dot":
			graph.RenderDOT(g, os.Stdout)
		case "json":
			return graph.RenderJSON(g, os.Stdout)
		default:
			graph.RenderASCII(g, os.Stdout)
		}
		return nil
	},
}

func init() {
	graphCmd.Flags().StringVar(&graphFormat, "format", "ascii", "Output format: ascii, dot, json")
	graphCmd.Flags().StringVarP(&graphConfigPath, "file", "f", "", "Path to config file")
}
