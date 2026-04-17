package app

import (
	"context"
	"sort"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// downOtherProjectsOnly handles the `--conflicting` / `--all-projects`
// short-circuits: it never touches the cwd project, only sibling projects
// detected via Docker labels. Mutual exclusivity with the regular down
// path is enforced by the caller.
func (uc *DownUseCase) downOtherProjectsOnly(
	ctx context.Context,
	opts DownOptions,
) error {
	cwdDeps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	cwdProject := ""
	if cwdDeps != nil {
		cwdProject = cwdDeps.Project.Name
	}

	if opts.Conflicting {
		baseDir, baseErr := uc.deps.Workspace.GetBaseDir()
		if baseErr != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				i18n.T("error.workspace_resolve"),
			).WithError(baseErr)
		}
		if cwdDeps == nil {
			output.PrintWarning(i18n.T("output.config_load_fallback"))
			return nil
		}
		_, err := DownConflictingProjects(ctx, cwdDeps, baseDir)
		return err
	}

	// AllProjects branch
	_, err := DownAllOtherProjects(ctx, cwdProject)
	return err
}

// uniqueConflictingProjects returns the deduplicated, sorted set of project
// names extracted from a list of PortConflicts, skipping the cwd's own name
// and any conflict missing a project label. Pure function so the filter
// logic stays testable without a Docker daemon.
func uniqueConflictingProjects(
	conflicts []docker.PortConflict,
	currentProject string,
) []string {
	set := map[string]struct{}{}
	for _, c := range conflicts {
		if c.Project != "" && c.Project != currentProject {
			set[c.Project] = struct{}{}
		}
	}
	return sortedKeys(set)
}

// filterOtherActiveProjects returns the active project list minus the cwd's.
// Pure helper — same pattern as uniqueConflictingProjects.
func filterOtherActiveProjects(active []string, currentProject string) []string {
	set := map[string]struct{}{}
	for _, p := range active {
		if p != currentProject && p != "" {
			set[p] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// DownConflictingProjects stops every active raioz project (cross-workspace)
// whose published host ports collide with the cwd's raioz.yaml. Returns the
// list of project names that were torn down. The cwd project itself is
// never touched — callers typically run `raioz up` afterwards to bring up
// the cwd cleanly.
func DownConflictingProjects(
	ctx context.Context,
	cwdDeps *config.Deps,
	baseDir string,
) ([]string, error) {
	if cwdDeps == nil {
		return nil, nil
	}
	conflicts, err := docker.ValidatePorts(cwdDeps, baseDir, cwdDeps.Project.Name)
	if err != nil {
		return nil, err
	}
	names := uniqueConflictingProjects(conflicts, cwdDeps.Project.Name)
	if len(names) == 0 {
		output.PrintInfo(i18n.T("output.no_conflicting_projects"))
		return nil, nil
	}
	return stopProjects(ctx, names), nil
}

// DownAllOtherProjects stops every active raioz project except the one
// whose name matches `currentProject` (typically the cwd's). Pass an empty
// string to stop every active project unconditionally.
func DownAllOtherProjects(
	ctx context.Context,
	currentProject string,
) ([]string, error) {
	active, err := docker.ListActiveProjects(ctx)
	if err != nil {
		return nil, err
	}
	names := filterOtherActiveProjects(active, currentProject)
	if len(names) == 0 {
		output.PrintInfo(i18n.T("output.no_other_projects"))
		return nil, nil
	}
	return stopProjects(ctx, names), nil
}

func stopProjects(ctx context.Context, names []string) []string {
	var stopped []string
	for _, name := range names {
		output.PrintInfo(i18n.T("output.stopping_other_project", name))
		containers, err := docker.StopProjectContainers(ctx, name)
		if err != nil {
			logging.WarnWithContext(ctx,
				"Failed to stop other project's containers",
				"project", name, "error", err.Error())
			continue
		}
		if len(containers) > 0 {
			stopped = append(stopped, name)
			output.PrintSuccess(i18n.T("output.other_project_stopped",
				name, len(containers)))
		}
	}
	return stopped
}
