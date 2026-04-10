package cli

import (
	"fmt"

	"raioz/internal/app"

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
		ctx := cmd.Context()
		deps := app.NewDependencies()

		if deps.ProxyManager == nil {
			fmt.Println("Proxy is not configured")
			return nil
		}

		if proj := app.ResolveYAMLProject(deps, ""); proj != nil {
			deps.ProxyManager.SetProjectName(proj.ProjectName)
		}

		running, err := deps.ProxyManager.Status(ctx)
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
		ctx := cmd.Context()
		deps := app.NewDependencies()

		if deps.ProxyManager == nil {
			fmt.Println("Proxy is not configured")
			return nil
		}

		if proj := app.ResolveYAMLProject(deps, ""); proj != nil {
			deps.ProxyManager.SetProjectName(proj.ProjectName)
		}

		return deps.ProxyManager.Stop(ctx)
	},
}

func init() {
	proxyCmd.AddCommand(proxyStatusCmd)
	proxyCmd.AddCommand(proxyStopCmd)
}
