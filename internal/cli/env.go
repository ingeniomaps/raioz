package cli

import (
	"context"
	"fmt"
	"os"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/i18n"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env <service>",
	Short: i18n.T("cmd.env.short"),
	Long: "Display all environment variables that a service would receive,\n" +
		"including variables from .env files and auto-injected service discovery variables.\n\n" +
		"Example:\n  raioz env api\n  raioz env frontend -f raioz.yaml",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz env"); panicErr != nil {
				err = panicErr
			}
		}()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		cfgPath, _ := cmd.Flags().GetString("file")
		cfgPath = ResolveConfigPath(cfgPath)

		deps := app.NewDependencies()
		uc := app.NewEnvShowUseCase(deps)

		entries, err := uc.Execute(ctx, app.EnvShowOptions{
			ConfigPath:  cfgPath,
			ServiceName: args[0],
		})
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Fprintln(os.Stderr, i18n.T("env.no_vars", args[0]))
			return nil
		}

		// Group by source
		var fileVars, discoveryVars []app.EnvEntry
		for _, e := range entries {
			switch e.Source {
			case "file":
				fileVars = append(fileVars, e)
			default:
				discoveryVars = append(discoveryVars, e)
			}
		}

		if len(fileVars) > 0 {
			fmt.Printf("\n  %s\n", i18n.T("env.section_file"))
			for _, e := range fileVars {
				fmt.Printf("  %s=%s\n", e.Key, e.Value)
			}
		}

		if len(discoveryVars) > 0 {
			fmt.Printf("\n  %s\n", i18n.T("env.section_discovery"))
			for _, e := range discoveryVars {
				fmt.Printf("  %s=%s\n", e.Key, e.Value)
			}
		}

		fmt.Println()
		return nil
	},
}

func init() {
	envCmd.Flags().StringP(
		"file", "f", "",
		"Path to config file (auto-detects if omitted)",
	)
}
