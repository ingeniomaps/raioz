package cli

import (
	"context"
	"fmt"

	"raioz/internal/app"
	"raioz/internal/detect"
	"raioz/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var dashboardConfigPath string

var dashboardCmd = &cobra.Command{
	Use:          "dashboard",
	Aliases:      []string{"tui"},
	Short:        "Interactive dashboard for monitoring services",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		deps := app.NewDependencies()
		cfgPath := ResolveConfigPath(dashboardConfigPath)

		// Try YAML mode first
		proj := app.ResolveYAMLProject(deps, cfgPath)
		if proj != nil {
			return runDashboardYAML(ctx, deps, proj)
		}

		// Legacy mode
		return runDashboardLegacy(ctx, deps, cfgPath)
	},
}

func runDashboardYAML(
	ctx context.Context,
	deps *app.Dependencies,
	proj *app.YAMLProject,
) error {
	var services []tui.ServiceRow

	for name, svc := range proj.Deps.Services {
		rt := "unknown"
		if svc.Source.Path != "" {
			result := detect.Detect(svc.Source.Path)
			rt = string(result.Runtime)
		}
		url := ""
		if deps.ProxyManager != nil {
			url = deps.ProxyManager.GetURL(name)
		}
		services = append(services, tui.ServiceRow{
			Name:    name,
			Runtime: rt,
			Status:  "unknown",
			URL:     url,
		})
	}
	for name, entry := range proj.Deps.Infra {
		label := "image"
		if entry.Inline != nil {
			label = entry.Inline.Image
			if entry.Inline.Tag != "" {
				label += ":" + entry.Inline.Tag
			}
		}
		services = append(services, tui.ServiceRow{
			Name:    name,
			Runtime: label,
			Status:  "unknown",
		})
	}

	cfg := tui.Config{
		Project:   proj.ProjectName,
		Workspace: proj.Deps.Workspace,
		Services:  services,
		Docker:    deps.DockerRunner,
		Proxy:     deps.ProxyManager,
		Ctx:       ctx,
		YAMLMode:  true,
	}

	model := tui.New(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runDashboardLegacy(
	ctx context.Context,
	deps *app.Dependencies,
	cfgPath string,
) error {
	cfgDeps, _, err := deps.ConfigLoader.LoadDeps(cfgPath)
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}

	var services []tui.ServiceRow
	for name, svc := range cfgDeps.Services {
		rt := "unknown"
		if svc.Source.Path != "" {
			result := detect.Detect(svc.Source.Path)
			rt = string(result.Runtime)
		}
		url := ""
		if deps.ProxyManager != nil {
			url = deps.ProxyManager.GetURL(name)
		}
		services = append(services, tui.ServiceRow{
			Name:    name,
			Runtime: rt,
			Status:  "unknown",
			URL:     url,
		})
	}
	for name, entry := range cfgDeps.Infra {
		label := "image"
		if entry.Inline != nil {
			label = entry.Inline.Image
			if entry.Inline.Tag != "" {
				label += ":" + entry.Inline.Tag
			}
		}
		services = append(services, tui.ServiceRow{
			Name:    name,
			Runtime: label,
			Status:  "unknown",
		})
	}

	cfg := tui.Config{
		Project:   cfgDeps.Project.Name,
		Workspace: cfgDeps.Workspace,
		Services:  services,
		Docker:    deps.DockerRunner,
		Proxy:     deps.ProxyManager,
		Ctx:       ctx,
	}

	model := tui.New(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func init() {
	dashboardCmd.Flags().StringVarP(&dashboardConfigPath, "file", "f", "", "Path to config file")
}
