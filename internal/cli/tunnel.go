package cli

import (
	"fmt"

	"raioz/internal/app"
	"raioz/internal/app/tunnelcase"
	"raioz/internal/output"

	"github.com/spf13/cobra"
)

const defaultTunnelPort = 3000

var tunnelPort int

var tunnelCmd = &cobra.Command{
	Use:          "tunnel [service]",
	Short:        "Expose a local service to the internet",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		port := tunnelPort
		if port == 0 {
			port = defaultTunnelPort
		}
		deps := app.NewDependencies()
		uc := tunnelcase.StartUseCase{Deps: &tunnelcase.Dependencies{TunnelManager: deps.TunnelManager}}
		info, err := uc.Execute(cmd.Context(), tunnelcase.StartOptions{
			ServiceName: args[0],
			LocalPort:   port,
		})
		if err != nil {
			return err
		}
		output.PrintSuccess(fmt.Sprintf("Tunnel active for %s", args[0]))
		fmt.Printf("  URL:   %s\n", info.PublicURL)
		fmt.Printf("  Local: http://localhost:%d\n", info.LocalPort)
		fmt.Printf("  PID:   %d\n", info.PID)
		return nil
	},
}

var tunnelListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List active tunnels",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		uc := tunnelcase.ListUseCase{Deps: &tunnelcase.Dependencies{TunnelManager: deps.TunnelManager}}
		tunnels := uc.Execute(cmd.Context())
		if len(tunnels) == 0 {
			output.PrintInfo("No active tunnels")
			return nil
		}
		for _, t := range tunnels {
			fmt.Printf("  %-20s %s  (port %d, pid %d)\n",
				t.ServiceName, t.PublicURL, t.LocalPort, t.PID)
		}
		return nil
	},
}

var tunnelStopCmd = &cobra.Command{
	Use:          "stop [service]",
	Short:        "Stop a tunnel",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		uc := tunnelcase.StopUseCase{Deps: &tunnelcase.Dependencies{TunnelManager: deps.TunnelManager}}
		if err := uc.Execute(cmd.Context(), tunnelcase.StopOptions{ServiceName: args[0]}); err != nil {
			return err
		}
		output.PrintSuccess(fmt.Sprintf("Tunnel stopped for %s", args[0]))
		return nil
	},
}

var tunnelStopAllCmd = &cobra.Command{
	Use:          "stop-all",
	Short:        "Stop all tunnels",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		uc := tunnelcase.StopAllUseCase{Deps: &tunnelcase.Dependencies{TunnelManager: deps.TunnelManager}}
		if err := uc.Execute(cmd.Context()); err != nil {
			return err
		}
		output.PrintSuccess("All tunnels stopped")
		return nil
	},
}

func init() {
	tunnelCmd.Flags().IntVar(&tunnelPort, "port", 0, "Local port to tunnel")
	tunnelCmd.AddCommand(tunnelListCmd)
	tunnelCmd.AddCommand(tunnelStopCmd)
	tunnelCmd.AddCommand(tunnelStopAllCmd)
}
