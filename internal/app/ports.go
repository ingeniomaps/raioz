package app

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

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

	endpoints := activeEndpointsFn(ctx)
	if opts.ProjectName != "" {
		var own []activeEndpoint
		for _, ep := range endpoints {
			if ep.Project == opts.ProjectName {
				own = append(own, ep)
			}
		}
		endpoints = own
	}

	if len(endpoints) == 0 {
		output.PrintInfo(i18n.T("output.no_active_ports"))
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.active_ports_header"))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "PORT\tPROJECT\tSERVICE\tRUNS AS")
	fmt.Fprintln(w, "────\t───────\t───────\t───────")

	for _, ep := range endpoints {
		project := ep.Project
		if project == "" {
			project = ep.Workspace
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", ep.Port, project, ep.Service, ep.Runner)
	}

	w.Flush()
	return nil
}
