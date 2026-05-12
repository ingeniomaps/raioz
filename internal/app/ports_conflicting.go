package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// listConflictingPorts is the read-only counterpart of
// `raioz down --conflicting`. It surfaces which sibling raioz projects are
// holding host ports declared in the cwd's raioz.yaml, without touching any
// of them. Useful for debugging "why does my up keep failing on port X?".
func (uc *PortsUseCase) listConflictingPorts(
	_ context.Context,
	opts PortsOptions,
) error {
	cwdDeps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if cwdDeps == nil {
		output.PrintWarning(i18n.T("output.config_load_fallback"))
		return nil
	}

	baseDir, err := uc.deps.Workspace.GetBaseDir()
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.base_dir"),
		).WithError(err)
	}

	conflicts, err := docker.ValidatePorts(cwdDeps, baseDir, cwdDeps.Project.Name)
	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.ports_get_active"),
		).WithError(err)
	}

	if len(conflicts) == 0 {
		output.PrintInfo(i18n.T("output.no_conflicting_projects"))
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.conflicting_ports_header"))
	printConflictingPortsTable(os.Stdout, conflicts)
	return nil
}

// printConflictingPortsTable writes a tab-aligned PORT/PROJECT/SERVICE/
// ALTERNATIVE table to w. Pure formatter so tests can assert content
// without spawning Docker.
func printConflictingPortsTable(w io.Writer, conflicts []docker.PortConflict) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(tw, "PORT\tPROJECT\tSERVICE\tALTERNATIVE")
	fmt.Fprintln(tw, "────\t───────\t───────\t───────────")
	for _, c := range conflicts {
		alt := c.Alternative
		if alt == "" {
			alt = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Port, c.Project, c.Service, alt)
	}
	tw.Flush()
}
