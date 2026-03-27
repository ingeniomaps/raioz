package upcase

import (
	"context"
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
