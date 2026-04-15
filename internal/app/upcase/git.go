package upcase

import (
	"context"
	"os"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	exectimeout "raioz/internal/exec"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// processGitRepos handles cloning/updating repos and branch changes
func (uc *UseCase) processGitRepos(
	ctx context.Context, deps *config.Deps, ws *interfaces.Workspace,
	oldDeps *config.Deps, forceReclone bool, projectDir string,
) error {
	// Update repos if branches changed (this happens during processState, but we handle the actual git updates here)
	if oldDeps != nil {
		// Check for branch changes
		var hasBranchChanges bool
		for name, svc := range deps.Services {
			if oldSvc, exists := oldDeps.Services[name]; exists {
				if svc.Source.Kind == "git" && oldSvc.Source.Kind == "git" {
					if svc.Source.Branch != oldSvc.Source.Branch {
						hasBranchChanges = true
						break
					}
				}
			}
		}

		if hasBranchChanges {
			// Use a resolver function to get correct paths based on access mode
			repoPathResolver := func(name string, svc config.Service) string {
				return uc.deps.Workspace.GetServicePath(ws, name, svc)
			}
			ctx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
			defer cancel()
			err := uc.deps.GitRepository.UpdateReposIfBranchChanged(
				ctx, repoPathResolver, oldDeps, deps,
			)
			if err != nil {
				return errors.New(
					errors.ErrCodeGitCloneFailed,
					i18n.T("error.git_update_branch_failed"),
				).WithSuggestion(
					i18n.T("error.git_update_branch_suggestion"),
				).WithError(err)
			}
		}
	}

	// Check for service conflicts before cloning
	for name, svc := range deps.Services {
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}
		if svc.Source.Kind == "git" {
			// Check if this service conflicts with a local project or another running service
			conflict, err := uc.detectServiceConflict(ctx, name, deps, ws, projectDir, false)
			if err != nil {
				logging.WarnWithContext(ctx, "Failed to detect service conflict", "service", name, "error", err.Error())
				continue
			}
			if conflict != nil {
				// Resolve conflict
				resolution, err := uc.resolveServiceConflict(ctx, conflict, false, deps.GetWorkspaceName(), projectDir)
				if err != nil {
					return err
				}
				if resolution == "skip" || resolution == "cancel" {
					// Skip this service or cancel entire operation
					if resolution == "cancel" {
						return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.operation_cancelled"))
					}
					continue
				}
				// Apply resolution
				err = uc.applyServiceConflictResolution(
					ctx, conflict, resolution, name, deps, ws, projectDir, false,
				)
				if err != nil {
					return err
				}
			}
		}
	}

	// Clone repos for services
	var disabledServices []string
	for name, svc := range deps.Services {
		// Check if service is disabled
		if svc.Enabled != nil && !*svc.Enabled {
			disabledServices = append(disabledServices, name)
			output.PrintInfo(i18n.T("up.service_disabled_skipping", name))
			continue
		}
		if svc.Source.Kind == "git" {
			output.PrintInfo(i18n.T("up.git.resolving", name))
			// Use correct directory based on access mode
			serviceDir := uc.deps.Workspace.GetServiceDir(ws, svc)

			// Check if repo exists before operation to determine action
			repoExistedBefore := false
			if _, err := os.Stat(serviceDir); err == nil {
				repoExistedBefore = true
			}

			// Check if repo exists to show appropriate message
			var actionMessage string
			if forceReclone {
				actionMessage = i18n.T("up.git.recloning", name)
			} else if uc.deps.GitRepository.IsReadonly(svc.Source) {
				// Check if readonly repo exists
				if repoExistedBefore {
					actionMessage = i18n.T("up.git.readonly_exists", name)
				} else {
					actionMessage = i18n.T("up.git.cloning", name)
				}
			} else {
				// Check if editable repo exists
				if repoExistedBefore {
					actionMessage = i18n.T("up.git.updating", name)
				} else {
					actionMessage = i18n.T("up.git.cloning", name)
				}
			}

			output.PrintProgress(actionMessage)
			serviceCtx := logging.WithService(ctx, name)
			logging.DebugWithContext(serviceCtx, "Ensuring repository",
				"repo", svc.Source.Repo, "branch", svc.Source.Branch,
				"path", svc.Source.Path, "service_dir", serviceDir,
				"force", forceReclone, "existed_before", repoExistedBefore)
			cloneStartTime := time.Now()
			err := uc.deps.GitRepository.EnsureRepoWithForce(
				svc.Source, serviceDir, forceReclone,
			)
			if err != nil {
				logging.ErrorWithContext(serviceCtx, "Failed to ensure repository",
					"repo", svc.Source.Repo, "branch", svc.Source.Branch,
					"duration_ms", time.Since(cloneStartTime).Milliseconds(),
					"error", err.Error())
				output.PrintProgressError(i18n.T("up.git.ensure_error", name))
				return err
			}
			logging.DebugWithContext(serviceCtx, "Repository ensured successfully",
				"repo", svc.Source.Repo, "branch", svc.Source.Branch,
				"duration_ms", time.Since(cloneStartTime).Milliseconds())

			// Show appropriate success message based on what actually happened
			if forceReclone {
				output.PrintProgressDone(i18n.T("up.git.recloned", name))
				output.PrintServiceCloned(name)
			} else if uc.deps.GitRepository.IsReadonly(svc.Source) {
				if repoExistedBefore {
					// Already existed, no update needed
					output.PrintProgressDone(i18n.T("up.git.readonly_ready", name))
					output.PrintInfo(i18n.T("up.git.service_readonly", name))
					output.PrintInfo(i18n.T("up.git.service_restart", name))
				} else {
					output.PrintProgressDone(i18n.T("up.git.cloned", name))
					output.PrintServiceCloned(name)
					output.PrintInfo(i18n.T("up.git.service_readonly", name))
					output.PrintInfo(i18n.T("up.git.service_restart", name))
				}
			} else {
				// Editable repo
				if repoExistedBefore {
					output.PrintProgressDone(i18n.T("up.git.updated", name))
					output.PrintSuccess(i18n.T("up.git.service_updated", name))
				} else {
					output.PrintProgressDone(i18n.T("up.git.cloned", name))
					output.PrintServiceCloned(name)
				}
			}
		}
	}
	if len(disabledServices) > 0 {
		output.PrintInfo(i18n.T("up.skipped_disabled_services", len(disabledServices), disabledServices))
	}

	return nil
}
