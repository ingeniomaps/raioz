package cli

import (
	"context"
	"fmt"
	"os"

	"raioz/internal/app"
	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/state"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:          "check",
	Short:        "Validate raioz.yaml without starting anything",
	SilenceUsage: true,
	Long:         "Validate the configuration without starting anything.",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz check"); panicErr != nil {
				err = panicErr
			}
		}()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		if configPath == "" && projectName != "" {
			resolved, resolveErr := ResolveProjectConfigPath(configPath, projectName)
			if resolveErr != nil {
				return resolveErr
			}
			configPath = resolved
		}

		// A meta config has no services of its own: check what it names.
		if path := ResolveConfigPath(configPath); path != AutoDetectMarker {
			if meta, isMeta, metaErr := config.LoadMetaConfig(path); isMeta {
				if metaErr != nil {
					return errors.New(errors.ErrCodeInvalidConfig, metaErr.Error())
				}
				return app.CheckMeta(meta)
			}
		}

		deps := newDependencies()
		checkUseCase := app.NewCheckUseCase(deps)

		result, err := checkUseCase.Execute(ctx, app.CheckOptions{
			ProjectName: projectName,
			ConfigPath:  configPath,
		})
		if err != nil {
			return err
		}

		displayCheckResult(result)
		return nil
	},
}

func displayCheckResult(result *app.CheckResult) {
	// YAML mode: CheckYAML already printed the section header, per-service
	// runtime, proxy/port errors, and the final "All checks passed" or
	// "N issue(s) found" summary. The only thing left for the CLI wrapper
	// to do is propagate the exit code — no extra "valid" banner, no legacy
	// state-alignment hints.
	if result.YAMLMode {
		if result.HasIssues {
			os.Exit(1)
		}
		return
	}

	// Show validation results
	if !result.ConfigValid {
		fmt.Println(i18n.T("check.config_invalid"))
		for _, e := range result.ValidationErrors {
			fmt.Printf("  • %s\n", e)
		}
		fmt.Println()
	} else {
		fmt.Println(i18n.T("check.config_valid"))
	}

	// Handle no state
	if result.NoState {
		fmt.Println(i18n.T("check.no_state_found"))
		fmt.Println(i18n.T("check.run_up_hint"))
		if result.HasIssues {
			os.Exit(1)
		}
		return
	}

	// Show alignment results
	fmt.Println(i18n.T("check.checking_alignment"))
	fmt.Println(state.FormatIssues(result.AlignmentIssues))

	if result.HasIssues {
		os.Exit(1)
	}
}

func init() {
	checkCmd.Flags().StringVarP(&configPath, "file", "f", "", "Path to config file")
	checkCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (alternative to --file)")
}
