package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/state"
)

// statusReport is what `raioz status` knows about a project. The table and
// the --json output are two renderings of the same value, so a script
// reads exactly what a person sees.
type statusReport struct {
	Project      string             `json:"project"`
	Workspace    string             `json:"workspace,omitempty"`
	Dependencies []dependencyStatus `json:"dependencies"`
	Services     []serviceStatus    `json:"services"`
	ProxyRunning bool               `json:"proxyRunning"`
}

type dependencyStatus struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Restarts int    `json:"restarts"`
	CPU      string `json:"cpu,omitempty"`
	Memory   string `json:"memory,omitempty"`
	Image    string `json:"image,omitempty"`
	// Dev is set while the dependency runs from a local path (`raioz dev`)
	// instead of Image.
	Dev bool `json:"dev,omitempty"`
}

type serviceStatus struct {
	Name     string `json:"name"`
	Runtime  string `json:"runtime"`
	Status   string `json:"status"`
	Restarts int    `json:"restarts"`
	PID      int    `json:"pid,omitempty"`
	Dev      bool   `json:"dev,omitempty"`
}

// collectStatus gathers the live state of every dependency and service in
// the filter, each group in name order so two runs are comparable.
func (uc *StatusUseCase) collectStatus(
	ctx context.Context, proj *YAMLProject, filter []string,
) (*statusReport, error) {
	if err := validateStatusFilter(proj, filter); err != nil {
		return nil, err
	}
	want := filterSet(filter)
	report := &statusReport{
		Project:      proj.ProjectName,
		Workspace:    proj.Deps.Workspace,
		Dependencies: []dependencyStatus{},
		Services:     []serviceStatus{},
	}

	projectDir, _ := filepath.Abs(filepath.Dir(proj.ConfigPath))
	localState, _ := state.LoadLocalState(projectDir)

	for _, name := range sortedKeysInfra(proj.Deps.Infra) {
		if !inFilter(want, name) {
			continue
		}
		entry := proj.Deps.Infra[name]
		// A dependency that is another raioz project (ADR-008 mode A) has
		// no container of its own: it is up when that project is.
		if entry.Inline != nil && entry.Inline.Project != "" {
			report.Dependencies = append(report.Dependencies,
				siblingDependencyStatus(name, entry.Inline.Project, projectDir))
			continue
		}
		st := proj.ContainerState(ctx, name)
		dep := dependencyStatus{Name: name, Status: st.Status, Restarts: st.Restarts}
		dep.CPU, dep.Memory = proj.ContainerStats(ctx, name)
		if entry.Inline != nil {
			dep.Image = entry.Inline.Image
			if entry.Inline.Tag != "" {
				dep.Image += ":" + entry.Inline.Tag
			}
		}
		dep.Dev = localState != nil && localState.IsDevOverridden(name)
		report.Dependencies = append(report.Dependencies, dep)
	}

	for _, name := range sortedKeysServices(proj.Deps.Services) {
		if !inFilter(want, name) {
			continue
		}
		svc := proj.Deps.Services[name]
		// Honor yaml overrides (command:, compose:) before scanning disk.
		result := config.ResolveServiceDetection(svc, svc.Source.Path)
		row := serviceStatus{Name: name, Runtime: string(result.Runtime), Status: statusStopped}
		if row.Runtime == "" {
			row.Runtime = "unknown"
		}
		row.Dev = localState != nil && localState.IsDevOverridden(name)

		switch {
		case svc.ProxyOverride != nil && svc.ProxyOverride.Target != "" &&
			applyContainerState(&row, func() (ContainerState, bool) {
				return dockerStateProbe(ctx, svc.ProxyOverride.Target)
			}):
			// Priority 0: when the user declared `proxy.target`, THAT
			// container is the source of truth — bypass the PID/compose
			// heuristics that go false-negative for launchers that exit
			// 0 after `docker run -d`. Reported verbatim, restart count
			// included: collapsing every non-running state into "stopped"
			// hid `restarting`, and the status alone hid a crash loop that
			// spends most of its cycle in `running`.
		case result.IsDocker():
			// A compose / Dockerfile service is a container: Docker is the
			// source of truth, and there is no PID to look at.
			st := proj.ContainerState(ctx, name)
			row.Status, row.Restarts = st.Status, st.Restarts
		case localState != nil:
			// Fallback: process alive via saved PID. A live PID is not
			// the same as a live service — see hostServiceStatus.
			if pid, ok := localState.HostPIDs[name]; ok && pid > 0 && isHostProcessAlive(pid) {
				row.Status = hostServiceStatus(ctx, svc.Port)
				row.PID = pid
			}
		}
		report.Services = append(report.Services, row)
	}

	if proj.Deps.Proxy && uc.deps.ProxyManager != nil {
		report.ProxyRunning, _ = uc.deps.ProxyManager.Status(ctx)
	}
	return report, nil
}

// applyContainerState copies a probed container state onto the row and
// reports whether the probe found the container.
func applyContainerState(row *serviceStatus, probe func() (ContainerState, bool)) bool {
	st, ok := probe()
	if ok {
		row.Status, row.Restarts = st.Status, st.Restarts
	}
	return ok
}

func printStatusReport(report *statusReport) {
	fmt.Println()
	output.PrintSectionHeader(report.Project)

	if len(report.Dependencies) > 0 {
		output.PrintSubsection(fmt.Sprintf("Dependencies (%d)", len(report.Dependencies)))
		for _, dep := range report.Dependencies {
			status := formatContainerStatus(ContainerState{Status: dep.Status, Restarts: dep.Restarts})
			image := dep.Image
			if dep.Dev {
				image += " (dev)"
			}
			fmt.Printf("    %-18s %-10s %-8s %-10s %s\n", dep.Name, status, dep.CPU, dep.Memory, image)
		}
	}

	if len(report.Services) > 0 {
		output.PrintSubsection(fmt.Sprintf("Services (%d)", len(report.Services)))
		for _, svc := range report.Services {
			status := formatContainerStatus(ContainerState{Status: svc.Status, Restarts: svc.Restarts})
			pidInfo := ""
			if svc.PID > 0 {
				pidInfo = fmt.Sprintf("pid:%d", svc.PID)
			}
			devLabel := ""
			if svc.Dev {
				devLabel = " (dev)"
			}
			fmt.Printf("    %-18s %-10s %-10s %-10s%s\n", svc.Name, svc.Runtime, status, pidInfo, devLabel)
		}
	}

	if report.ProxyRunning {
		output.PrintInfo(i18n.T("output.proxy_running"))
	}
	fmt.Println()
}

// statusJSON prints the report as JSON and nothing else, so the output
// can be piped straight into a parser.
func (uc *StatusUseCase) statusJSON(ctx context.Context, proj *YAMLProject, filter []string) error {
	report, err := uc.collectStatus(ctx, proj, filter)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode status: %w", err)
	}
	return nil
}

// siblingDependencyStatus reports a sibling-project dependency: running
// when an active project lives at the sibling's path.
func siblingDependencyStatus(name, siblingPath, projectDir string) dependencyStatus {
	if !filepath.IsAbs(siblingPath) {
		siblingPath = filepath.Join(projectDir, siblingPath)
	}
	siblingPath = filepath.Clean(siblingPath)

	dep := dependencyStatus{Name: name, Status: statusStopped, CPU: "-", Memory: "-"}
	for _, project := range recordedProjects() {
		if project.Path != "" && filepath.Clean(project.Path) == siblingPath {
			dep.Status = statusRunning
			dep.Image = "→ " + project.Name
			return dep
		}
	}
	dep.Image = "→ " + filepath.Base(siblingPath)
	return dep
}
