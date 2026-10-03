package app

import (
	"context"
	"sort"

	"raioz/internal/docker"
	"raioz/internal/domain/models"
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
		_, err := DownConflictingProjects(ctx, cwdDeps, baseDir,
			approveStopping(opts.Yes, reasonConflicting))
		return err
	}

	// AllProjects branch
	_, err := DownAllOtherProjects(ctx, cwdProject, approveStopping(opts.Yes, reasonAllProjects))
	return err
}

// downOtherWorkspaceProjects stops every OTHER raioz project with live
// containers in the workspace — what makes `raioz down --all` a workspace
// shutdown rather than a cwd-project one.
//
// Label-invisible projects (`command:` launchers) survive the scan. Not a
// silent half-down: the proxy gate still sees them through their route
// targets and keeps serving them (ADR-005).
func (uc *DownUseCase) downOtherWorkspaceProjects(
	ctx context.Context, workspace, currentProject string, approve stopApproval,
) error {
	if workspace == "" {
		return nil
	}
	live, err := liveWorkspaceProjects(ctx, workspace)
	if err != nil {
		logging.WarnWithContext(ctx, "Skipping workspace-wide down: docker probe failed",
			"workspace", workspace, "error", err.Error())
		return nil
	}
	names := filterOtherActiveProjects(sortedKeys(live), currentProject)
	if len(names) == 0 {
		return nil
	}
	// Declined: the siblings stay up and the cwd project still goes down.
	approved, err := approve(names)
	if err != nil || !approved {
		return err
	}
	stopProjects(ctx, names)
	return nil
}

// uniqueConflictingProjects returns the deduplicated, sorted set of project
// names extracted from a list of PortConflicts, skipping the cwd's own name
// and any conflict missing a project label. Pure function so the filter
// logic stays testable without a Docker daemon.
func uniqueConflictingProjects(
	conflicts []portConflict,
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

// Package-level hooks so tests can simulate Docker without a daemon.
// Same pattern as listContainersByLabelsFn in down_proxy.go.
var (
	portConflictsFn         = findPortConflicts
	listPublishedPortsFn    = docker.ListManagedPublishedPorts
	listActiveProjectsFn    = docker.ListActiveProjects
	stopProjectContainersFn = docker.StopProjectContainers
)

// portConflict is aliased here so the files that build and print
// conflicts stay off the app→infra import list (ADR-029).
type portConflict = docker.PortConflict

// publishedPort is aliased for the same reason.
type publishedPort = docker.PublishedPort

// DownConflictingProjects stops every active raioz project (cross-workspace)
// whose published host ports collide with the cwd's raioz.yaml. Returns the
// list of project names that were torn down. The cwd project itself is
// never touched — callers typically run `raioz up` afterwards to bring up
// the cwd cleanly.
func DownConflictingProjects(
	ctx context.Context,
	cwdDeps *models.Deps,
	baseDir string,
	approve stopApproval,
) ([]string, error) {
	if cwdDeps == nil {
		return nil, nil
	}
	_ = baseDir
	conflicts, err := portConflictsFn(ctx, cwdDeps)
	if err != nil {
		return nil, err
	}
	names := uniqueConflictingProjects(conflicts, cwdDeps.Project.Name)
	if len(names) == 0 {
		output.PrintInfo(i18n.T("output.no_conflicting_projects"))
		return nil, nil
	}
	if approved, err := approve(names); err != nil || !approved {
		return nil, err
	}
	return stopProjects(ctx, names), nil
}

// DownAllOtherProjects stops every active raioz project except the one
// whose name matches `currentProject` (typically the cwd's). Pass an empty
// string to stop every active project unconditionally.
func DownAllOtherProjects(
	ctx context.Context,
	currentProject string,
	approve stopApproval,
) ([]string, error) {
	// Docker only knows the projects that run containers; one made of
	// host services alone is active too, and only the state lists it.
	active, err := listActiveProjectsFn(ctx)
	if err != nil {
		return nil, err
	}
	for _, project := range recordedProjects() {
		active = append(active, project.Name)
	}
	names := filterOtherActiveProjects(active, currentProject)
	if len(names) == 0 {
		output.PrintInfo(i18n.T("output.no_other_projects"))
		return nil, nil
	}
	if approved, err := approve(names); err != nil || !approved {
		return nil, err
	}
	return stopProjects(ctx, names), nil
}

// stopProjects tears down each named project and returns the ones that
// went down. A project whose directory is on record is stopped through
// its own `raioz down`; removing its containers is the fallback for one
// brought up by a raioz too old to have recorded it, and leaves any host
// service of that project running.
func stopProjects(ctx context.Context, names []string) []string {
	var stopped []string
	for _, name := range names {
		output.PrintInfo(i18n.T("output.stopping_other_project", name))
		if dir := projectPathFn(name); dir != "" {
			if err := downProjectFn(ctx, dir); err == nil {
				stopped = append(stopped, name)
				output.PrintSuccess(i18n.T("output.other_project_down", name))
				continue
			}
		}
		containers, err := stopProjectContainersFn(ctx, name)
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
