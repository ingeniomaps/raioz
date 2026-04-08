package upcase

import (
	"context"
	"fmt"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/root"
	"raioz/internal/state"
)

// processState loads state, detects changes, and returns old deps, changes, added services, and assisted services map
func (uc *UseCase) processState(
	ctx context.Context,
	deps *config.Deps,
	ws *interfaces.Workspace,
	configPath string,
) (*config.Deps, []state.ConfigChange, []string, map[string]string, error) {
	// Load previous state
	oldDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		return nil, nil, nil, nil, errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_load_previous"),
		).WithSuggestion(
			i18n.T("error.state_load_previous_suggestion"),
		).WithContext("workspace", ws.Root).WithError(err)
	}

	// Compare deps to detect changes
	changes, err := uc.deps.StateManager.CompareDeps(oldDeps, deps)
	if err != nil {
		return nil, nil, nil, nil, errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.state_compare_failed"),
		).WithSuggestion(
			i18n.T("error.state_compare_suggestion"),
		).WithError(err)
	}

	// Log changes if any
	if len(changes) > 0 {
		changeSummary := uc.deps.StateManager.FormatChanges(changes)
		logging.InfoWithContext(ctx, "Configuration changes detected", "changes_count", len(changes))
		logging.DebugWithContext(ctx, "Changes", "changes", changeSummary)
	}

	// Calculate addedServices and assistedServicesMap from dependency assist
	// Note: addedServices and assistedServicesMap should be calculated in validateUp,
	// but for now we return empty values here
	addedServices := []string{}
	assistedServicesMap := make(map[string]string)

	return oldDeps, changes, addedServices, assistedServicesMap, nil
}

// saveState saves state, generates/updates root config, detects drift, and logs audit events
func (uc *UseCase) saveState(
	ctx context.Context,
	deps *config.Deps,
	ws *interfaces.Workspace,
	composePath string,
	serviceNames []string,
	addedServices []string,
	assistedServicesMap map[string]string,
	appliedOverrides []string,
) error {
	// Save state
	if err := uc.deps.StateManager.Save(ws, deps); err != nil {
		return errors.New(
			errors.ErrCodeStateSaveError,
			i18n.T("error.state_save_failed"),
		).WithSuggestion(
			i18n.T("error.state_save_suggestion"),
		).WithContext("workspace", ws.Root).WithError(err)
	}

	// Only log at debug level - technical detail not useful for end users
	logging.DebugWithContext(ctx, "State saved successfully", "workspace", ws.Root)

	// Generate or update root config with applied overrides
	if root.Exists(ws) {
		// Update existing root config
		rootConfig, err := root.Load(ws)
		if err != nil {
			logging.Warn("Failed to load root config, will generate new one", "error", err)
			rootConfig = nil
		}
		if rootConfig != nil {
			if err := root.UpdateFromDeps(rootConfig, deps, appliedOverrides, assistedServicesMap); err != nil {
				return errors.New(
					errors.ErrCodeStateSaveError,
					i18n.T("error.root_config_update_failed"),
				).WithSuggestion(
					i18n.T("error.state_save_suggestion"),
				).WithContext("workspace", ws.Root).WithError(err)
			}
			if err := root.Save(ws, rootConfig); err != nil {
				return errors.New(
					errors.ErrCodeStateSaveError,
					i18n.T("error.root_config_save_failed"),
				).WithSuggestion(
					i18n.T("error.state_save_suggestion"),
				).WithContext("workspace", ws.Root).WithError(err)
			}
		} else {
			// Generate new root config
			rootConfig, err := root.GenerateFromDeps(deps, appliedOverrides, assistedServicesMap)
			if err != nil {
				return errors.New(
					errors.ErrCodeStateSaveError,
					i18n.T("error.root_config_generate_failed"),
				).WithSuggestion(
					i18n.T("error.root_config_generate_suggestion"),
				).WithError(err)
			}
			if err := root.Save(ws, rootConfig); err != nil {
				return errors.New(
					errors.ErrCodeStateSaveError,
					i18n.T("error.root_config_save_failed"),
				).WithSuggestion(
					i18n.T("error.state_save_suggestion"),
				).WithContext("workspace", ws.Root).WithError(err)
			}
		}
	} else {
		// Generate new root config
		rootConfig, err := root.GenerateFromDeps(deps, appliedOverrides, assistedServicesMap)
		if err != nil {
			return errors.New(
				errors.ErrCodeStateSaveError,
				i18n.T("error.root_config_generate_failed"),
			).WithSuggestion(
				i18n.T("error.root_config_generate_suggestion"),
			).WithError(err)
		}
		if err := root.Save(ws, rootConfig); err != nil {
			return errors.New(
				errors.ErrCodeStateSaveError,
				i18n.T("error.root_config_save_failed"),
			).WithSuggestion(
				i18n.T("error.state_save_suggestion"),
			).WithContext("workspace", ws.Root).WithError(err)
		}
	}

	// Detect and log drift for assisted services
	rootConfig, err := root.Load(ws)
	if err == nil && rootConfig != nil {
		drifts, err := root.DetectAssistedServiceDrift(rootConfig, ws)
		if err == nil && len(drifts) > 0 {
			driftSummary := root.FormatDrifts(drifts)
			logging.Warn("Configuration drift detected", "drifts_count", len(drifts))
			logging.Debug("Drifts", "drifts", driftSummary)
		}
	}

	return nil
}

// updateGlobalState updates the global state with project information
func (uc *UseCase) updateGlobalState(
	ctx context.Context,
	deps *config.Deps,
	ws *interfaces.Workspace,
	composePath string,
	serviceNames []string,
) error {
	// Get service info
	services := make(map[string]config.Service)
	for _, name := range serviceNames {
		if svc, exists := deps.Services[name]; exists {
			services[name] = svc
		}
	}

	// Get service info from docker
	serviceInfos, err := uc.deps.DockerRunner.GetServicesInfoWithContext(
		ctx,
		composePath,
		serviceNames,
		deps.Project.Name,
		services,
		ws,
	)
	if err != nil {
		// Log error but don't fail - service info is optional
		logging.Warn("Failed to get service info for global state", "error", err)
		serviceInfos = nil
	}

	// Convert interfaces.ServiceInfo to state.ServiceInfo
	stateServiceInfos := make(map[string]*state.ServiceInfo)
	for name, info := range serviceInfos {
		stateServiceInfos[name] = &state.ServiceInfo{
			Status:  info.Status,
			Version: info.Commit,
			Image:   info.Image,
		}
	}

	// Build service states
	serviceStates := uc.deps.StateManager.BuildServiceStates(deps, stateServiceInfos)

	// Create project state
	projectState := &state.ProjectState{
		Name:          deps.Project.Name,
		Workspace:     deps.GetWorkspaceName(),
		LastExecution: time.Now(),
		Services:      serviceStates,
	}

	// Update global state
	if err := uc.deps.StateManager.UpdateProjectState(deps.Project.Name, projectState); err != nil {
		return fmt.Errorf("failed to update global state: %w", err)
	}

	return nil
}
