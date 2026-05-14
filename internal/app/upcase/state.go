package upcase

import (
	"context"
	"fmt"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/root"
)

// processState loads state, detects changes, and returns old deps, changes, added services, and assisted services map
func (uc *UseCase) processState(
	ctx context.Context,
	deps *models.Deps,
	ws *interfaces.Workspace,
	configPath string,
) (*models.Deps, []models.ConfigChange, []string, map[string]string, error) {
	// ADR-011 Phase 2: drift detection between the previous snapshot
	// and the current raioz.yaml is gone. raioz `up` is convergent and
	// idempotent, so logging "configuration changes detected" carried
	// information but not correctness. If users miss the log line,
	// reintroduce drift via a hashed snapshot in LocalState — see
	// 031b's notes on LocalState.LastUpConfigHash.
	_ = ctx
	_ = ws
	_ = configPath
	addedServices := []string{}
	assistedServicesMap := make(map[string]string)
	return nil, nil, addedServices, assistedServicesMap, nil
}

// saveState saves state, generates/updates root config, detects drift, and logs audit events
func (uc *UseCase) saveState(
	ctx context.Context,
	deps *models.Deps,
	ws *interfaces.Workspace,
	composePath string,
	serviceNames []string,
	addedServices []string,
	assistedServicesMap map[string]string,
	appliedOverrides []string,
) error {
	// ADR-011 Phase 1: the legacy whole-Deps snapshot at .state.json is
	// no longer written. The auto-cleanup at the top of Execute deletes
	// any stale file left from older binaries.
	_ = ws
	_ = deps
	logging.DebugWithContext(ctx, "Legacy state snapshot intentionally skipped (ADR-011)")

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
	deps *models.Deps,
	ws *interfaces.Workspace,
	composePath string,
	serviceNames []string,
) error {
	// Get service info
	services := make(map[string]models.Service)
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

	// Convert interfaces.ServiceInfo to models.ServiceInfo
	stateServiceInfos := make(map[string]*models.ServiceInfo)
	for name, info := range serviceInfos {
		stateServiceInfos[name] = &models.ServiceInfo{
			Status:  info.Status,
			Version: info.Commit,
			Image:   info.Image,
		}
	}

	// Build service states
	serviceStates := uc.deps.StateManager.BuildServiceStates(deps, stateServiceInfos)

	// Create project state
	projectState := &models.ProjectState{
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
