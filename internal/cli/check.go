package cli

import (
	"context"
	"fmt"
	"os"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/state"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:          "check",
	Short:        "Check for alignment issues between config and state",
	SilenceUsage: true,
	Long:         "Check if the current configuration aligns with the saved state.",
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

		deps := app.NewDependencies()
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
