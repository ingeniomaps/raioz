package app

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// PortsOptions contains options for the Ports use case
type PortsOptions struct {
	ProjectName string
	// ConfigPath is honored when Conflicting is set; ignored otherwise.
	ConfigPath string
	// Conflicting flips Execute into the read-only "which sibling projects
	// hold ports declared in my raioz.yaml?" report. No containers are
	// stopped — this is `raioz down --conflicting` minus the side effects.
	Conflicting bool
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
	if opts.Conflicting {
		return uc.listConflictingPorts(ctx, opts)
	}

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
			return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.base_dir")).WithError(err)
		}
	}

	ports, err := uc.deps.DockerRunner.GetAllActivePorts(baseDir)
	if err != nil {
		return errors.New(errors.ErrCodeDockerNotRunning, i18n.T("error.ports_get_active")).WithError(err)
	}

	if len(ports) == 0 {
		output.PrintInfo(i18n.T("output.no_active_ports"))
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.active_ports_header"))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "PORT\tPROJECT\tSERVICE")
	fmt.Fprintln(w, "────\t───────\t───────")

	for _, port := range ports {
		fmt.Fprintf(w, "%s\t%s\t%s\n", port.Port, port.Project, port.Service)
	}

	w.Flush()
	return nil
}
