package cli

import (
	"errors"
	"fmt"

	"raioz/internal/app/proxycase"

	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:          "proxy",
	Short:        "Manage the reverse proxy",
	SilenceUsage: true,
}

var proxyStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show proxy status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := newDependencies()
		uc := proxycase.StatusUseCase{Deps: &proxycase.Dependencies{
			ConfigLoader: deps.ConfigLoader,
			ProxyManager: deps.ProxyManager,
		}}
		running, err := uc.Execute(cmd.Context(), proxycase.StatusOptions{})
		if errors.Is(err, proxycase.ErrProxyNotConfigured) {
			fmt.Println("Proxy is not configured")
			return nil
		}
		if err != nil {
			return err
		}
		if running {
			fmt.Println("Proxy: running")
		} else {
			fmt.Println("Proxy: stopped")
		}
		return nil
	},
}

var proxyStopCmd = &cobra.Command{
	Use:          "stop",
	Short:        "Stop the proxy",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := newDependencies()
		uc := proxycase.StopUseCase{Deps: &proxycase.Dependencies{
			ConfigLoader: deps.ConfigLoader,
			ProxyManager: deps.ProxyManager,
		}}
		err := uc.Execute(cmd.Context(), proxycase.StopOptions{})
		if errors.Is(err, proxycase.ErrProxyNotConfigured) {
			fmt.Println("Proxy is not configured")
			return nil
		}
		return err
	},
}

func init() {
	proxyCmd.AddCommand(proxyStatusCmd)
	proxyCmd.AddCommand(proxyStopCmd)
}
