package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

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
	writeTableHeader(w, i18n.T("ports.col_port"), i18n.T("ports.col_project"),
		i18n.T("ports.col_service"), i18n.T("ports.col_runs_as"))

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

// writeTableHeader writes the column titles and a rule under each, as wide
// as its title in whatever language it is in.
func writeTableHeader(w io.Writer, columns ...string) {
	rules := make([]string, len(columns))
	for i, col := range columns {
		rules[i] = strings.Repeat("─", utf8.RuneCountInString(col))
	}
	fmt.Fprintln(w, strings.Join(columns, "\t"))
	fmt.Fprintln(w, strings.Join(rules, "\t"))
}
