package cli

import (
	"raioz/internal/app"
	"raioz/internal/errors"

	"github.com/spf13/cobra"
)

var initOutputPath string

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Scan and generate raioz.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz init"); panicErr != nil {
				err = panicErr
			}
		}()

		useCase := app.NewInitScanUseCase()
		return useCase.Execute(app.InitScanOptions{
			OutputPath: initOutputPath,
		})
	},
}

func init() {
	initCmd.Flags().StringVarP(
		&initOutputPath,
		"output", "o", "raioz.yaml",
		"Output path for generated config",
	)
}
