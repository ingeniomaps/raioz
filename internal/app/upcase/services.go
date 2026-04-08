package upcase

import (
	"context"
	"fmt"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
)

// checkServicesRunning checks if services are already running (if no changes)
func (uc *UseCase) checkServicesRunning(
	ctx context.Context,
	deps *config.Deps,
	ws *interfaces.Workspace,
	changes []state.ConfigChange,
	oldDeps *config.Deps,
) (bool, error) {
	// If no services or infra, nothing to check
	if len(deps.Services) == 0 && len(deps.Infra) == 0 {
		return false, nil
	}

	if len(changes) == 0 && oldDeps != nil {
		composePath := uc.deps.Workspace.GetComposePath(ws)
		var expectedServices []string
		for name := range deps.Services {
			expectedServices = append(expectedServices, name)
		}
		for name := range deps.Infra {
			expectedServices = append(expectedServices, name)
		}
		if len(expectedServices) > 0 {
			allRunning, err := uc.deps.DockerRunner.AreServicesRunning(composePath, expectedServices)
			if err == nil && allRunning {
				output.PrintSuccess(i18n.T("up.all_services_running"))
				return true, nil
			}
		}
	}
	return false, nil
}

// showDryRunSummary shows what would happen without executing
func (uc *UseCase) showDryRunSummary(deps *config.Deps, appliedOverrides []string) {
	w := uc.out()
	fmt.Fprintln(w)
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("up.dry_run.header"))
	fmt.Fprintf(w, "  %s: %s\n", i18n.T("up.dry_run.project"), deps.Project.Name)
	fmt.Fprintf(w, "  %s: %s\n", i18n.T("up.dry_run.network"), deps.Network.GetName())
	fmt.Fprintln(w)

	var gitServices, imageServices, hostServices []string
	for name, svc := range deps.Services {
		switch {
		case svc.Source.Command != "":
			hostServices = append(hostServices, name)
		case svc.Source.Kind == "git":
			gitServices = append(gitServices, name)
		case svc.Source.Kind == "image":
			imageServices = append(imageServices, name)
		default:
			imageServices = append(imageServices, name)
		}
	}

	if len(gitServices) > 0 {
		fmt.Fprintf(w, "  %s: %v\n", i18n.T("up.dry_run.git_clone"), gitServices)
	}
	if len(imageServices) > 0 {
		fmt.Fprintf(w, "  %s: %v\n", i18n.T("up.dry_run.docker_services"), imageServices)
	}
	if len(hostServices) > 0 {
		fmt.Fprintf(w, "  %s: %v\n", i18n.T("up.dry_run.host_services"), hostServices)
	}

	if len(deps.Infra) > 0 {
		var infraNames []string
		for name := range deps.Infra {
			infraNames = append(infraNames, name)
		}
		fmt.Fprintf(w, "  %s: %v\n", i18n.T("up.dry_run.infra"), infraNames)
	}

	if len(appliedOverrides) > 0 {
		fmt.Fprintf(w, "  %s: %v\n", i18n.T("up.dry_run.overrides"), appliedOverrides)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("up.dry_run.footer"))
}

// showSummary displays the final summary
func (uc *UseCase) showSummary(
	ctx context.Context,
	deps *config.Deps,
	serviceNames []string,
	infraNames []string,
	startTime time.Time,
) {
	elapsed := time.Since(startTime)
	output.PrintProjectStarted(deps.Project.Name)
	output.PrintSummary(serviceNames, infraNames, elapsed)
	// Only log operation end at debug level - not useful for end users
	logging.DebugWithContext(ctx, "Operation completed",
		"operation", "raioz up",
		"duration_ms", elapsed.Milliseconds(),
		"project", deps.Project.Name,
		"services_count", len(deps.Services),
	)
}
