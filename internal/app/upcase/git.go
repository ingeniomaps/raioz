package upcase

import (
	"context"
	"fmt"
	"os"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	exectimeout "raioz/internal/exec"
	"raioz/internal/git"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/workspace"
)

// processGitRepos handles cloning/updating repos and branch changes
func (uc *UseCase) processGitRepos(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace, oldDeps *config.Deps, forceReclone bool) error {
	// Convert interfaces.Workspace to concrete workspace.Workspace for operations that need it
	wsConcrete := (*workspace.Workspace)(ws)

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
				return workspace.GetServicePath(wsConcrete, name, svc)
			}
			ctx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
			defer cancel()
			if err := git.UpdateReposIfBranchChanged(ctx, repoPathResolver, oldDeps, deps); err != nil {
				return errors.New(errors.ErrCodeGitCloneFailed, "Failed to update repositories after branch changes").WithSuggestion("Check network connectivity and repository access. " + "Verify that branch names are correct and accessible.").WithError(err)
			}
		}
	}

	// Clone repos for services
	var disabledServices []string
	for name, svc := range deps.Services {
		// Check if service is disabled
		if svc.Enabled != nil && !*svc.Enabled {
			disabledServices = append(disabledServices, name)
			output.PrintInfo(fmt.Sprintf("Service %s is disabled, skipping", name))
			continue
		}
		if svc.Source.Kind == "git" {
			output.PrintInfo(fmt.Sprintf("ℹ️  resolving %s", name))
			// Use correct directory based on access mode
			serviceDir := workspace.GetServiceDir(wsConcrete, svc)

			// Check if repo exists before operation to determine action
			repoExistedBefore := false
			if _, err := os.Stat(serviceDir); err == nil {
				repoExistedBefore = true
			}

			// Check if repo exists to show appropriate message
			var actionMessage string
			if forceReclone {
				actionMessage = fmt.Sprintf("Re-cloning repository for service '%s'...", name)
			} else if git.IsReadonly(svc.Source) {
				// Check if readonly repo exists
				if repoExistedBefore {
					actionMessage = fmt.Sprintf("Repository for service '%s' already exists (readonly, skipping update)", name)
				} else {
					actionMessage = fmt.Sprintf("Cloning repository for service '%s'...", name)
				}
			} else {
				// Check if editable repo exists
				if repoExistedBefore {
					actionMessage = fmt.Sprintf("Updating repository for service '%s'...", name)
				} else {
					actionMessage = fmt.Sprintf("Cloning repository for service '%s'...", name)
				}
			}

			output.PrintProgress(actionMessage)
			serviceCtx := logging.WithService(ctx, name)
			logging.DebugWithContext(serviceCtx, "Ensuring repository", "repo", svc.Source.Repo, "branch", svc.Source.Branch, "path", svc.Source.Path, "service_dir", serviceDir, "force", forceReclone, "existed_before", repoExistedBefore)
			cloneStartTime := time.Now()
			if err := uc.deps.GitRepository.EnsureRepoWithForce(svc.Source, serviceDir, forceReclone); err != nil {
				logging.ErrorWithContext(serviceCtx, "Failed to ensure repository", "repo", svc.Source.Repo, "branch", svc.Source.Branch, "duration_ms", time.Since(cloneStartTime).Milliseconds(), "error", err.Error())
				output.PrintProgressError(fmt.Sprintf("Failed to ensure repository for service '%s'", name))
				return err
			}
			logging.DebugWithContext(serviceCtx, "Repository ensured successfully", "repo", svc.Source.Repo, "branch", svc.Source.Branch, "duration_ms", time.Since(cloneStartTime).Milliseconds())

			// Show appropriate success message based on what actually happened
			if forceReclone {
				output.PrintProgressDone(fmt.Sprintf("Repository re-cloned for service '%s'", name))
				output.PrintServiceCloned(name)
			} else if git.IsReadonly(svc.Source) {
				if repoExistedBefore {
					// Already existed, no update needed
					output.PrintProgressDone(fmt.Sprintf("Repository for service '%s' ready (readonly)", name))
					output.PrintInfo(fmt.Sprintf("Service %s is readonly (protected from automatic updates, volumes mounted as :ro)", name))
					output.PrintInfo(fmt.Sprintf("Service %s will be automatically recreated if it fails (restart: unless-stopped)", name))
				} else {
					output.PrintProgressDone(fmt.Sprintf("Repository cloned for service '%s'", name))
					output.PrintServiceCloned(name)
					output.PrintInfo(fmt.Sprintf("Service %s is readonly (protected from automatic updates, volumes mounted as :ro)", name))
					output.PrintInfo(fmt.Sprintf("Service %s will be automatically recreated if it fails (restart: unless-stopped)", name))
				}
			} else {
				// Editable repo
				if repoExistedBefore {
					output.PrintProgressDone(fmt.Sprintf("Repository updated for service '%s'", name))
					output.PrintSuccess(fmt.Sprintf("%s actualizado", name))
				} else {
					output.PrintProgressDone(fmt.Sprintf("Repository cloned for service '%s'", name))
					output.PrintServiceCloned(name)
				}
			}
		}
	}
	if len(disabledServices) > 0 {
		output.PrintInfo(fmt.Sprintf("Skipped %d disabled service(s): %v", len(disabledServices), disabledServices))
	}

	return nil
}
