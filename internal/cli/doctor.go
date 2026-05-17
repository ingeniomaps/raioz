package cli

import (
	"context"
	"os"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var doctorPrintSpawnEnv bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system requirements and environment health",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// `raioz doctor --print-spawn-env` short-circuits the regular
		// checks and dumps the env raioz would pass to sub-spawns
		// (hooks / sibling / meta). Secret-shaped keys redact values.
		if doctorPrintSpawnEnv {
			app.PrintSpawnEnv(os.Stdout)
			return nil
		}

		useCase := app.NewDoctorUseCase()
		useCase.DevBuild = IsDevBuild()
		return useCase.Execute(ctx)
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorPrintSpawnEnv, "print-spawn-env", false,
		"Print the env raioz would inherit when spawning a sub-process "+
			"(hooks / sibling / meta). Secret-shaped keys are listed but "+
			"their values are redacted.")
}
