package cli

import (
	"fmt"

	"raioz/internal/output"
	"raioz/internal/tunnel"

	"github.com/spf13/cobra"
)

var tunnelPort int

var tunnelCmd = &cobra.Command{
	Use:          "tunnel [service]",
	Short:        "Expose a local service to the internet",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]
		ctx := cmd.Context()

		mgr := tunnel.NewManager()
		port := tunnelPort
		if port == 0 {
			port = 3000 // default
		}

		info, err := mgr.Start(ctx, serviceName, port)
		if err != nil {
			return err
		}

		output.PrintSuccess(fmt.Sprintf("Tunnel active for %s", serviceName))
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
		mgr := tunnel.NewManager()
		tunnels := mgr.List()

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
		mgr := tunnel.NewManager()
		if err := mgr.Stop(args[0]); err != nil {
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
		mgr := tunnel.NewManager()
		mgr.StopAll()
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
