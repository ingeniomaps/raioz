package cli

import (
	"errors"

	"raioz/internal/app/proxycase"
	"raioz/internal/i18n"
	"raioz/internal/output"

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
		running, err := uc.Execute(cmd.Context(), proxycase.StatusOptions{
			ConfigPath: ResolveConfigPath(""),
		})
		if errors.Is(err, proxycase.ErrProxyNotConfigured) {
			output.PrintInfo(i18n.T("proxy.not_configured"))
			return nil
		}
		if err != nil {
			return err
		}
		if running {
			output.PrintSuccess(i18n.T("proxy.status_running"))
		} else {
			output.PrintInfo(i18n.T("proxy.status_stopped"))
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
		err := uc.Execute(cmd.Context(), proxycase.StopOptions{
			ConfigPath: ResolveConfigPath(""),
		})
		if errors.Is(err, proxycase.ErrProxyNotConfigured) {
			output.PrintInfo(i18n.T("proxy.not_configured"))
			return nil
		}
		return err
	},
}

func init() {
	proxyCmd.AddCommand(proxyStatusCmd)
	proxyCmd.AddCommand(proxyStopCmd)
}
