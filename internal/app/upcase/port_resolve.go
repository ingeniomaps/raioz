package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/state"
)

// publishedHostPortFn reads a container's live published host port. Declared
// here (port_resolve.go already imports internal/docker) and as a package var
// so tests can stub it without a running docker daemon — same rationale as
// portInUseProbe. Consumed by reuseSharedDepHostPorts in port_alloc_locked.go.
var publishedHostPortFn = docker.GetPublishedHostPort

// isOwnContainer reports whether the port occupant is a raioz container that
// belongs to THIS run and will therefore be reused — not a foreign conflict.
// Two cases qualify: a leftover container from the same project, or a
// workspace-shared dependency this project also declares. Shared deps omit
// the project label (ADR-002), so they are matched on workspace + service
// instead.
func isOwnContainer(occ docker.PortOccupant, deps *models.Deps, activeWorkspace string) bool {
	if !occ.IsRaioz || deps == nil {
		return false
	}
	if occ.ProjectName != "" && occ.ProjectName == deps.Project.Name {
		return true
	}
	return occ.Workspace != "" && occ.Workspace == activeWorkspace &&
		isDeclaredDep(deps, occ.Service)
}

// ownHostServicePIDFn is a package var so tests can stand in for the
// state file and /proc.
var ownHostServicePIDFn = ownHostServicePID

// ownHostServicePID returns the PID recorded for the service in conflict
// when that process is the one holding the port, 0 otherwise. The up flow
// leaves host processes of a just-upped project alone (see
// cleanStaleHostProcesses); without this the bind check would then report
// the project's own service as a foreign occupant.
//
// Where the port owner cannot be read (non-Linux) a live recorded PID is
// taken as enough: the alternative is refusing every repeated up there.
func ownHostServicePID(c PortBindConflict, configPath string) int {
	if c.Kind != "service" || configPath == "" {
		return 0
	}
	projectDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return 0
	}
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil {
		return 0
	}
	pid := localState.HostPIDs[c.Name]
	if pid <= 0 || !host.IsProcessAlive(pid) {
		return 0
	}
	if held, known := host.ProcessGroupListensOn(pid, c.Port); known && !held {
		return 0
	}
	return pid
}

// runningHostPorts builds the hostPortOwner for a project: the ports each
// recorded host service is listening on right now. Nil when there is no
// state to read, which leaves the allocator on its plain bump-upward path.
func runningHostPorts(projectDir string) hostPortOwner {
	if projectDir == "" {
		return nil
	}
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil || len(localState.HostPIDs) == 0 {
		return nil
	}
	return func(service string) []int {
		pid := localState.HostPIDs[service]
		if pid <= 0 || !host.IsProcessAlive(pid) {
			return nil
		}
		ports, _ := host.GroupListeningPorts(pid)
		return ports
	}
}

// isDeclaredDep reports whether name matches a dependency declared by this
// project. Used to recognize a workspace-shared dep's running container as
// our own (reuse) rather than a foreign port conflict.
func isDeclaredDep(deps *models.Deps, name string) bool {
	if deps == nil || name == "" {
		return false
	}
	_, ok := deps.Infra[name]
	return ok
}

// resolvePortBindConflicts walks every bind conflict detected after port
// allocation, identifies what occupies the port, and lets the user decide
// how to proceed. Raioz never kills containers or processes it did not
// start — the user must resolve external occupancy themselves or let raioz
// reassign the port.
//
// When a port is reassigned the PortAllocResult is updated in-place and,
// if configPath is non-empty, the raioz.yaml file is patched so future
// runs use the new port.
func resolvePortBindConflicts(
	ctx context.Context,
	conflicts []PortBindConflict,
	result *PortAllocResult,
	configPath string,
	deps *models.Deps,
	activeWorkspace string,
) error {
	reader := bufio.NewReader(os.Stdin)

	for _, c := range conflicts {
		occ := docker.IdentifyPortOccupant(ctx, c.Port)

		// If the port is held by one of our own raioz containers — a
		// same-project leftover or a workspace-shared dep we declare — it
		// will be reused when the run proceeds, so it is not a conflict.
		if isOwnContainer(occ, deps, activeWorkspace) {
			continue
		}

		// Same reasoning for a host service an earlier up of this project
		// left running: the port is busy because the service is up.
		if pid := ownHostServicePIDFn(c, configPath); pid > 0 {
			if result.RunningHost == nil {
				result.RunningHost = make(map[string]int)
			}
			result.RunningHost[c.Name] = pid
			continue
		}

		// --- Show what is occupying the port ---
		printConflictBanner(c, occ)

		// Non-interactive contexts (CI, scripts, piped stdin) cannot answer
		// the prompt: ReadString would hit EOF and surface a cryptic
		// "failed to read input" error. Fail fast with an actionable message
		// instead. Mirrors handleProxyStartFailure's non-tty fallback.
		if !stdinIsInteractiveFn() {
			return errors.New(errors.ErrCodePortConflict,
				fmt.Sprintf(i18n.T("port.conflict.non_interactive"),
					c.Port, c.Kind, c.Name)).
				WithSuggestion(i18n.T("port.conflict.non_interactive_hint"))
		}

		// --- Prompt the user ---
		resolution, newPort, err := promptPortResolution(c, reader)
		if err != nil {
			return err
		}

		switch resolution {
		case "auto":
			alt, err := docker.FindAlternativePort(
				fmt.Sprintf("%d", c.Port), 100)
			if err != nil {
				return errors.New(errors.ErrCodePortConflict,
					fmt.Sprintf(i18n.T("error.port_allocation_exhausted"), c.Name))
			}
			applyPortChange(c, alt, result, configPath)

		case "specify":
			applyPortChange(c, newPort, result, configPath)

		case "skip":
			return errors.New(errors.ErrCodePortConflict,
				i18n.T("port.conflict.retry_after_resolve"))
		}
	}
	return nil
}

// printConflictBanner prints a human-readable description of who is using
// the port.
func printConflictBanner(c PortBindConflict, occ docker.PortOccupant) {
	output.PrintWarning("")
	if occ.IsDocker {
		if occ.IsRaioz {
			output.PrintWarning(fmt.Sprintf(
				i18n.T("port.conflict.occupied_by_raioz"),
				c.Port, occ.ProjectName))
		} else {
			output.PrintWarning(fmt.Sprintf(
				i18n.T("port.conflict.occupied_by_container"),
				c.Port, occ.ContainerName))
		}
	} else {
		output.PrintWarning(fmt.Sprintf(
			i18n.T("port.conflict.occupied_by_external"), c.Port))
	}

	label := c.Kind
	output.PrintInfo(fmt.Sprintf("  %s: %s, port: %d", label, c.Name, c.Port))
}

// promptPortResolution shows the interactive menu and returns the user's
// choice. For "specify" it also returns the chosen port number.
func promptPortResolution(
	c PortBindConflict,
	reader *bufio.Reader,
) (string, int, error) {
	output.PrintInfo("")
	output.PrintInfo(i18n.T("port.conflict.choose_resolution"))

	options := []string{
		i18n.T("port.conflict.opt_auto"),
		i18n.T("port.conflict.opt_specify"),
		i18n.T("port.conflict.opt_skip"),
	}
	for i, opt := range options {
		output.PrintInfo(fmt.Sprintf(i18n.T("port.conflict.option"), i+1, opt))
	}
	output.PrintPrompt(fmt.Sprintf(
		i18n.T("port.conflict.your_choice"), len(options)))

	response, err := reader.ReadString('\n')
	if err != nil {
		return "", 0, fmt.Errorf("failed to read input: %w", err)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(response))
	if err != nil || choice < 1 || choice > len(options) {
		// Default to skip on bad input.
		return "skip", 0, nil
	}

	switch choice {
	case 1:
		return "auto", 0, nil
	case 2:
		return promptSpecificPort(c, reader)
	default:
		return "skip", 0, nil
	}
}

// promptSpecificPort asks the user for a concrete port number, validating
// that it is not already in use.
func promptSpecificPort(
	c PortBindConflict,
	reader *bufio.Reader,
) (string, int, error) {
	for {
		output.PrintPrompt(i18n.T("port.conflict.enter_port"))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", 0, fmt.Errorf("failed to read input: %w", err)
		}

		port, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || port < 1 || port > 65535 {
			output.PrintWarning(i18n.T("port.conflict.invalid_port"))
			continue
		}

		inUse, _ := docker.CheckPortInUse(fmt.Sprintf("%d", port))
		if inUse {
			output.PrintWarning(fmt.Sprintf(
				i18n.T("port.conflict.port_also_in_use"), port))
			continue
		}

		return "specify", port, nil
	}
}

// applyPortChange updates the allocation result in-place and optionally
// patches the raioz.yaml config so the change persists.
func applyPortChange(
	c PortBindConflict,
	newPort int,
	result *PortAllocResult,
	configPath string,
) {
	label := c.Kind
	output.PrintSuccess(fmt.Sprintf(
		i18n.T("port.conflict.reassigned"),
		label, c.Name, c.Port, newPort))

	// Update the allocation result.
	switch c.Kind {
	case "service":
		if alloc, ok := result.Services[c.Name]; ok {
			alloc.Port = newPort
			result.Services[c.Name] = alloc
		}
	case "dep":
		if alloc, ok := result.Deps[c.Name]; ok {
			for i, m := range alloc.Mappings {
				if m.HostPort == c.Port {
					alloc.Mappings[i].HostPort = newPort
					break
				}
			}
			result.Deps[c.Name] = alloc
		}
	}

	// Persist the change to raioz.yaml so future runs don't collide.
	if configPath != "" {
		if err := config.UpdatePort(configPath, c.Name, c.Kind, c.Port, newPort); err == nil {
			output.PrintInfo(fmt.Sprintf(
				i18n.T("port.conflict.config_updated"), configPath))
		}
	}
}
