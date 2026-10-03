package cli

import (
	"os"

	"raioz/internal/errors"
	"raioz/internal/graph"
	"raioz/internal/i18n"

	"github.com/spf13/cobra"
)

var graphFormat string
var graphConfigPath string

var graphCmd = &cobra.Command{
	Use:          "graph",
	Short:        "Visualize service dependency graph",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := newDependencies()
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
		case "ascii", "":
			graph.RenderASCII(g, os.Stdout)
		default:
			return errors.New(
				errors.ErrCodeInvalidField,
				i18n.T("error.graph_unknown_format", graphFormat),
			)
		}
		return nil
	},
}

func init() {
	graphCmd.Flags().StringVar(&graphFormat, "format", "ascii", "Output format: ascii, dot, json")
	graphCmd.Flags().StringVarP(&graphConfigPath, "file", "f", "", "Path to config file")
}
