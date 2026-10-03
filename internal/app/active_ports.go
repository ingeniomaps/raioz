package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"raioz/internal/app/upcase"
	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/state"
)

// activeEndpoint is a host port some raioz project holds right now.
// A container is one way raioz launches things, a host process is
// another; both hold host ports and both belong in this inventory.
type activeEndpoint struct {
	Project   string // empty for a workspace-shared dependency (ADR-002)
	Workspace string
	Service   string
	Runner    string // "container" or "host"
	Port      int
}

const (
	runnerContainer = "container"
	runnerHost      = "host"
)

// activeEndpointsFn is a package var so tests can describe a machine
// without a docker daemon or live processes.
var activeEndpointsFn = collectActiveEndpoints

// collectActiveEndpoints lists the host ports held by every active raioz
// project: the ones its containers publish (read from Docker labels) and
// the ones its host services listen on (recorded PID → process group →
// listening sockets). Either source failing leaves the other standing.
func collectActiveEndpoints(ctx context.Context) []activeEndpoint {
	var out []activeEndpoint

	published, err := listPublishedPortsFn(ctx)
	if err != nil {
		logging.WarnWithContext(ctx, "Could not list published container ports", "error", err.Error())
	}
	for _, p := range published {
		out = append(out, activeEndpoint{
			Project: p.Project, Workspace: p.Workspace, Service: p.Service,
			Runner: runnerContainer, Port: p.HostPort,
		})
	}

	for _, project := range recordedProjects() {
		if project.Path == "" {
			continue
		}
		localState, err := state.LoadLocalState(project.Path)
		if err != nil || localState == nil {
			continue
		}
		for service, pid := range localState.HostPIDs {
			if pid <= 0 || !host.IsProcessAlive(pid) {
				continue
			}
			ports, _ := host.GroupListeningPorts(pid)
			for _, port := range ports {
				out = append(out, activeEndpoint{
					Project: project.Name, Workspace: project.Workspace, Service: service,
					Runner: runnerHost, Port: port,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Project+"/"+out[i].Service < out[j].Project+"/"+out[j].Service
	})
	return out
}

// recordedProjects returns the projects the global state lists as active.
// Package var so tests never read the machine's own state.
var recordedProjects = loadRecordedProjects

func loadRecordedProjects() []models.ProjectState {
	globalState, err := state.LoadGlobalState()
	if err != nil || globalState == nil {
		return nil
	}
	var out []models.ProjectState
	for _, name := range globalState.ActiveProjects {
		if project, ok := globalState.Projects[name]; ok {
			out = append(out, project)
		}
	}
	return out
}

// projectPathFn returns the directory of an active project, "" when it
// was not recorded. Package var so tests never read the machine's state.
var projectPathFn = recordedProjectPath

func recordedProjectPath(name string) string {
	for _, project := range recordedProjects() {
		if project.Name == name {
			return project.Path
		}
	}
	return ""
}

// findPortConflicts returns the host ports the cwd project asks for that
// another raioz project holds. The cwd project's own endpoints and the
// shared dependencies of its workspace are not conflicts: up reuses them.
func findPortConflicts(ctx context.Context, cwdDeps *models.Deps) ([]portConflict, error) {
	if cwdDeps == nil {
		return nil, nil
	}
	active := activeEndpointsFn(ctx)

	var conflicts []portConflict
	seen := map[string]bool{}
	for _, wanted := range upcase.WantedHostPorts(cwdDeps) {
		for _, ep := range active {
			if ep.Port != wanted.Port || ownEndpoint(ep, cwdDeps) {
				continue
			}
			owner := ep.Project
			if owner == "" {
				owner = ep.Workspace
			}
			key := strconv.Itoa(ep.Port) + "/" + owner + "/" + ep.Service
			if seen[key] {
				continue
			}
			seen[key] = true
			conflicts = append(conflicts, portConflict{
				Port: strconv.Itoa(ep.Port), Project: owner, Service: ep.Service,
			})
		}
	}
	return conflicts, nil
}

// ownEndpoint reports whether an active endpoint is the cwd project's own:
// one of its services or containers, or a shared dependency of its
// workspace that it declares too.
func ownEndpoint(ep activeEndpoint, cwdDeps *models.Deps) bool {
	if ep.Project != "" {
		return ep.Project == cwdDeps.Project.Name
	}
	if ep.Workspace == "" || ep.Workspace != cwdDeps.Workspace {
		return false
	}
	_, declared := cwdDeps.Infra[ep.Service]
	return declared
}

// downProjectFn tears another project down. Package var so tests do not
// re-execute the test binary.
var downProjectFn = downProjectAt

// downProjectAt runs `raioz down` in another project's directory. A
// project is torn down by its own down, not by removing its containers:
// that is the only path that also stops its host services, runs its
// `stop:` commands and clears its state.
func downProjectAt(ctx context.Context, projectDir string) error {
	if _, err := os.Stat(filepath.Join(projectDir, "raioz.yaml")); err != nil {
		return fmt.Errorf("no raioz.yaml in %s: %w", projectDir, err)
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the raioz binary: %w", err)
	}
	cmd := exec.CommandContext(ctx, self, "down")
	cmd.Dir = projectDir
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		logging.WarnWithContext(ctx, "raioz down failed for another project",
			"dir", projectDir, "error", err.Error(), "output", string(out))
		return fmt.Errorf("raioz down in %s: %w", projectDir, err)
	}
	return nil
}

// ActiveProjectConfig returns the raioz.yaml of an active project given its
// name, so `-p <name>` works from any directory. ok is false when the
// project is not active or its directory was never recorded.
func ActiveProjectConfig(name string) (path string, ok bool) {
	dir := projectPathFn(name)
	if dir == "" {
		return "", false
	}
	for _, candidate := range []string{"raioz.yaml", "raioz.yml"} {
		config := filepath.Join(dir, candidate)
		if _, err := os.Stat(config); err == nil {
			return config, true
		}
	}
	return "", false
}

// hostPortOwner names the project and service whose host process holds
// port, for the conflict message `up` prints.
func hostPortOwner(ctx context.Context, port int) (project, service string) {
	for _, ep := range activeEndpointsFn(ctx) {
		if ep.Runner == runnerHost && ep.Port == port {
			return ep.Project, ep.Service
		}
	}
	return "", ""
}

func init() {
	upcase.HostPortOwnerFn = hostPortOwner
	upcase.ProjectDownFn = func(ctx context.Context, projectDir string) error {
		return downProjectFn(ctx, projectDir)
	}
}
