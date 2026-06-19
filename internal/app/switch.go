package app

import (
	"bufio"
	"context"
	"os"
	"sort"
	"strings"

	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// SwitchOptions configures `raioz switch`.
type SwitchOptions struct {
	ConfigPath string
	// Yes skips the interactive confirmation prompt.
	Yes bool
	// Keep is a list of project names to spare from teardown even if they
	// hold colliding ports. Useful when one of the conflicts is a sibling
	// the user wants to leave running.
	Keep []string
}

// SwitchUseCase wraps `raioz down --conflicting && raioz up` into a single
// command. Detects host-port collisions against the
// cwd's raioz.yaml, prompts for confirmation, stops the offenders, then
// brings the cwd project up.
//
// The detection logic is shared with `raioz down --conflicting` and
// `raioz ports --conflicting` — same `docker.ValidatePorts` source of
// truth, same `uniqueConflictingProjects` filter — so the three commands
// stay aligned on what counts as a conflict.
type SwitchUseCase struct {
	deps *Dependencies
}

// NewSwitchUseCase creates a SwitchUseCase with the injected dependency
// container so tests can swap out the docker/config/workspace adapters.
func NewSwitchUseCase(deps *Dependencies) *SwitchUseCase {
	return &SwitchUseCase{deps: deps}
}

// Execute resolves conflicts, prompts, tears them down, and runs up.
// Returns nil when the user declines the prompt (cancel is not an error).
func (uc *SwitchUseCase) Execute(ctx context.Context, opts SwitchOptions) error {
	cwdDeps, _, loadErr := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if loadErr != nil || cwdDeps == nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.switch_no_config"),
		).WithSuggestion(i18n.T("error.switch_no_config_suggestion"))
	}

	baseDir, baseErr := uc.deps.Workspace.GetBaseDir()
	if baseErr != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithError(baseErr)
	}

	conflicts, err := docker.ValidatePorts(cwdDeps, baseDir, cwdDeps.Project.Name)
	if err != nil {
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.switch_detect_failed"),
		).WithError(err)
	}

	toStop := filterKeep(
		uniqueConflictingProjects(conflicts, cwdDeps.Project.Name),
		opts.Keep,
	)

	if len(toStop) == 0 {
		output.PrintInfo(i18n.T("output.switch_no_conflicts"))
	} else {
		printSwitchConflicts(toStop, conflicts)
		if !opts.Yes {
			confirmed, err := confirmSwitch()
			if err != nil {
				return err
			}
			if !confirmed {
				output.PrintInfo(i18n.T("output.switch_cancelled"))
				return nil
			}
		}
		stopped := stopProjects(ctx, toStop)
		if len(stopped) > 0 {
			output.PrintSuccess(
				i18n.T("output.switch_stopped", len(stopped)))
		}
	}

	upUC := NewUpUseCase(uc.deps)
	return upUC.Execute(ctx, UpOptions{ConfigPath: opts.ConfigPath})
}

// filterKeep removes any name listed in keep from names, trimming
// whitespace on the keep entries so the CLI can pass through a raw
// comma-split slice without callers having to normalize first.
func filterKeep(names, keep []string) []string {
	if len(keep) == 0 {
		return names
	}
	skip := make(map[string]struct{}, len(keep))
	for _, k := range keep {
		if t := strings.TrimSpace(k); t != "" {
			skip[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if _, drop := skip[n]; drop {
			continue
		}
		out = append(out, n)
	}
	return out
}

// portsForProject returns the deduplicated, sorted host ports a given
// project holds in the conflict list. Display-only — used to tell the
// user *why* a project is on the teardown list before they confirm.
func portsForProject(conflicts []docker.PortConflict, project string) []string {
	seen := make(map[string]struct{})
	var ports []string
	for _, c := range conflicts {
		if c.Project == project && c.Port != "" {
			if _, ok := seen[c.Port]; !ok {
				seen[c.Port] = struct{}{}
				ports = append(ports, c.Port)
			}
		}
	}
	sort.Strings(ports)
	return ports
}

func printSwitchConflicts(projects []string, conflicts []docker.PortConflict) {
	output.PrintSectionHeader(i18n.T("output.switch_conflicts_header"))
	for _, name := range projects {
		ports := portsForProject(conflicts, name)
		output.PrintInfo(i18n.T(
			"output.switch_conflict_line",
			name, strings.Join(ports, ", "),
		))
	}
}

// confirmSwitch asks the user to confirm the teardown plan. Default is N
// — accidentally typing Enter or anything other than y/yes preserves the
// current state. Mirrors the prompt style used in clean.go / volumes.go.
func confirmSwitch() (bool, error) {
	output.PrintPrompt(i18n.T("output.switch_confirm"))
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, errors.New(
			errors.ErrCodeInternalError,
			i18n.T("error.read_input"),
		).WithError(err)
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// SplitKeepList parses a comma-separated --keep value into a clean
// project-name list. Exported so the CLI layer can normalize once and
// hand a clean slice to SwitchOptions without leaking the format detail
// into the use case.
func SplitKeepList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
