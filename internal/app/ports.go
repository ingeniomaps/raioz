package app

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"raioz/internal/output"
)

// PortsOptions contains options for the Ports use case
type PortsOptions struct {
	ProjectName string
}

// PortsUseCase handles the "ports" use case
type PortsUseCase struct {
	deps *Dependencies
}

// NewPortsUseCase creates a new PortsUseCase with injected dependencies
func NewPortsUseCase(deps *Dependencies) *PortsUseCase {
	return &PortsUseCase{deps: deps}
}

// Execute executes the ports use case
func (uc *PortsUseCase) Execute(ctx context.Context, opts PortsOptions) error {
	var baseDir string

	if opts.ProjectName != "" {
		ws, err := uc.deps.Workspace.Resolve(opts.ProjectName)
		if err == nil {
			baseDir = uc.deps.Workspace.GetBaseDirFromWorkspace(ws)
		}
	}

	if baseDir == "" {
		var err error
		baseDir, err = uc.deps.Workspace.GetBaseDir()
		if err != nil {
			return fmt.Errorf("failed to get base directory: %w", err)
		}
	}

	ports, err := uc.deps.DockerRunner.GetAllActivePorts(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get active ports: %w", err)
	}

	if len(ports) == 0 {
		output.PrintInfo("No active ports found")
		return nil
	}

	output.PrintSectionHeader("Active Ports")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "PORT\tPROJECT\tSERVICE")
	fmt.Fprintln(w, "────\t───────\t───────")

	for _, port := range ports {
		fmt.Fprintf(w, "%s\t%s\t%s\n", port.Port, port.Project, port.Service)
	}

	w.Flush()
	return nil
}
