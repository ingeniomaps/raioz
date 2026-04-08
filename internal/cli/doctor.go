package cli

import (
	"context"

	"raioz/internal/app"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system requirements and environment health",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		useCase := app.NewDoctorUseCase()
		return useCase.Execute(ctx)
	},
}
