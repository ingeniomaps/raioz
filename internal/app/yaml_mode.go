package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	dockerpkg "raioz/internal/docker"
	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

func findConfigFile() string {
	for _, c := range []string{"raioz.yaml", "raioz.yml", ".raioz.json"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// YAMLProject holds resolved info for a YAML-mode project.
type YAMLProject struct {
	Deps        *models.Deps
	ProjectName string
	NetworkName string
	ConfigPath  string
}

// ResolveYAMLProject attempts to load the config and returns a YAMLProject
// if it's a YAML-mode project (schemaVersion 2.0). Returns nil if not.
func ResolveYAMLProject(deps *Dependencies, configPath string) *YAMLProject {
	if configPath == "" || configPath == ":auto:" {
		configPath = findConfigFile()
		if configPath == "" {
			return nil
		}
	}

	cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || cfgDeps == nil {
		return nil
	}

	if cfgDeps.SourceFormat != models.SourceFormatYAML {
		return nil
	}

	// Activate the workspace prefix so naming.DepContainer / .Container
	// produce the same names the up flow used. Without this, status /
	// subsequent commands would look for `raioz-<proj>-<dep>` while the
	// container on disk is `<workspace>-<dep>`.
	naming.SetPrefix(cfgDeps.Workspace)

	return &YAMLProject{
		Deps:        cfgDeps,
		ProjectName: cfgDeps.Project.Name,
		NetworkName: cfgDeps.Network.GetName(),
		ConfigPath:  configPath,
	}
}

// ContainerPrefix returns the naming prefix for this project's containers.
func (p *YAMLProject) ContainerPrefix() string {
	return fmt.Sprintf("raioz-%s-", p.ProjectName)
}

// ListRunningContainers returns names of running containers for this project.
func (p *YAMLProject) ListRunningContainers(ctx context.Context) []string {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "ps",
		"--filter", "name="+p.ContainerPrefix(),
		"--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	names := strings.TrimSpace(string(out))
	if names == "" {
		return nil
	}
	return strings.Split(names, "\n")
}

// resolveInfraContainerName picks the right container name for a dependency
// based on workspace-sharing rules and user-supplied `name:` overrides,
// falling back to the legacy per-project form when neither applies.
func (p *YAMLProject) resolveInfraContainerName(name string) string {
	if p.Deps == nil {
		return naming.Container(p.ProjectName, name)
	}
	var override string
	if entry, ok := p.Deps.Infra[name]; ok && entry.Inline != nil {
		override = entry.Inline.Name
	}
	if _, isInfra := p.Deps.Infra[name]; isInfra {
		return naming.DepContainer(p.ProjectName, name, override)
	}
	return naming.Container(p.ProjectName, name)
}

// ContainerState is everything a single `docker inspect` can say about a
// container's liveness.
//
// Restarts is not decoration: State.Status alone cannot tell a crash loop
// from a healthy service. `restarting` covers only the backoff between
// attempts, so a container that takes seconds to die reads `running` in
// nearly every sample — 12 of 12 samples over 18s of a 5s-crash loop said
// `running`, and `docker ps` agreed ("Up 2 seconds"). The restart counter
// is the one field that does not flicker, and only the restart policy
// bumps it: a manual `docker restart` leaves it at 0.
type ContainerState struct {
	Status   string // State.Status verbatim: running, restarting, exited, ...
	Restarts int    // State.RestartCount
}

// dockerStateProbe is the probe the status paths consult. Package var so
// tests can decide the answer without a docker daemon — same seam as
// hostStatusPortProbe.
var dockerStateProbe = inspectContainerState

// ContainerStatus returns the status of a specific container, discarding
// the restart count. For callers that only branch on liveness.
func (p *YAMLProject) ContainerStatus(ctx context.Context, name string) string {
	return p.ContainerState(ctx, name).Status
}

// ContainerState returns the runtime state of a specific container. Routes
// both the canonical-name probe and the label-based fallback
// through naming.ResolveContainer — the single resolver shared by proxy,
// discovery, and down. A container that does not exist reports "stopped".
func (p *YAMLProject) ContainerState(ctx context.Context, name string) ContainerState {
	var override string
	if p.Deps != nil {
		if entry, ok := p.Deps.Infra[name]; ok && entry.Inline != nil {
			override = entry.Inline.Name
		}
	}
	resolved, _ := naming.ResolveContainer(ctx, dockerpkg.NewLookup(),
		p.ProjectName, name, override)
	if resolved == "" {
		return ContainerState{Status: statusStopped}
	}
	if st, ok := dockerStateProbe(ctx, resolved); ok {
		return st
	}
	return ContainerState{Status: statusStopped}
}

// inspectContainerState returns the runtime state of a container by name.
// The bool is false when docker inspect fails (typically because the
// container does not exist), so callers can branch cleanly on "miss" vs
// "found" without re-running shell parsing.
func inspectContainerState(ctx context.Context, name string) (ContainerState, bool) {
	if name == "" {
		return ContainerState{}, false
	}
	cmd := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}} {{.RestartCount}}", name)
	out, err := cmd.Output()
	if err != nil {
		return ContainerState{}, false
	}
	return parseContainerState(string(out))
}

// parseContainerState reads the two-field inspect format. Split from the
// exec so the parsing has a test that does not need a docker daemon. A
// missing count is tolerated rather than fatal: the status still answers
// more than nothing.
func parseContainerState(out string) (ContainerState, bool) {
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return ContainerState{}, false
	}
	st := ContainerState{Status: fields[0]}
	if len(fields) > 1 {
		st.Restarts, _ = strconv.Atoi(fields[1])
	}
	return st, true
}

// formatContainerStatus renders the status cell of the status tables. The
// restart count is appended only when it is non-zero, so a healthy row is
// unchanged and a container that has died on its own says so in every
// phase of its loop — see ContainerState for why State.Status alone does
// not. The cell overflows its column when the marker is present; that row
// is the one the reader is meant to notice.
func formatContainerStatus(st ContainerState) string {
	if st.Restarts > 0 {
		return fmt.Sprintf("%s restarts:%d", st.Status, st.Restarts)
	}
	return st.Status
}

// ContainerStats returns CPU and memory for a container.
func (p *YAMLProject) ContainerStats(ctx context.Context, name string) (cpu, mem string) {
	containerName := p.resolveInfraContainerName(name)
	cmd := exec.CommandContext(ctx, runtime.Binary(), "stats", "--no-stream",
		"--format", "{{.CPUPerc}}\t{{.MemUsage}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "-", "-"
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "-", "-"
}
