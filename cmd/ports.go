package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"raioz/internal/docker"
	"raioz/internal/output"
	"raioz/internal/workspace"

	"github.com/spf13/cobra"
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List all ports in use by active projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get base directory
		var baseDir string
		if projectName != "" {
			ws, err := workspace.Resolve(projectName)
			if err == nil {
				baseDir = workspace.GetBaseDirFromWorkspace(ws)
			}
		}

		// Fallback to GetBaseDir if no project specified
		if baseDir == "" {
			var err error
			baseDir, err = workspace.GetBaseDir()
			if err != nil {
				return fmt.Errorf("failed to get base directory: %w", err)
			}
		}

		// Get all active ports
		ports, err := docker.GetAllActivePorts(baseDir)
		if err != nil {
			return fmt.Errorf("failed to get active ports: %w", err)
		}

		if len(ports) == 0 {
			output.PrintInfo("No active ports found")
			return nil
		}

		// Display ports
		output.PrintSectionHeader("Active Ports")

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
		fmt.Fprintln(w, "PORT\tPROJECT\tSERVICE")
		separator := "────\t───────\t───────"
		fmt.Fprintln(w, separator)

		for _, port := range ports {
			fmt.Fprintf(w, "%s\t%s\t%s\n", port.Port, port.Project, port.Service)
		}

		w.Flush()
		return nil
	},
}

func init() {
	portsCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project name (optional)")
}
