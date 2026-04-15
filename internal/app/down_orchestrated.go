package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
	"raioz/internal/runtime"
	"raioz/internal/state"
)

// downOrchestrated handles raioz down for YAML-based (orchestrated) projects.
// Instead of using a generated compose file, it stops containers by name
// and kills host processes tracked in .raioz.state.json.
func (uc *DownUseCase) downOrchestrated(ctx context.Context, opts DownOptions) error {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "raioz.yaml"
	}

	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || deps == nil {
		return nil // Cannot load config — fall through to legacy down
	}

	if deps.SchemaVersion != "2.0" {
		return nil // Not a YAML project — fall through to legacy down
	}

	// Set naming prefix from workspace
	naming.SetPrefix(deps.Workspace)

	projectDir, _ := filepath.Abs(filepath.Dir(configPath))
	projectName := deps.Project.Name

	output.PrintProgress("Stopping project " + projectName + "...")

	// Run custom per-service stop commands first (declared in raioz.yaml as `stop:`).
	// These take precedence over PID/prefix-based cleanup because they know how to
	// tear down whatever the `command:` started — e.g. `make start` spawning its
	// own docker compose project whose containers are outside the raioz naming
	// convention.
	runCustomStopCommands(ctx, deps, projectDir)

	// Tear down compose-based services (yaml `compose:` or auto-detected) by
	// invoking `docker compose down` with the same project name scope that
	// ComposeRunner used to bring them up. Without this, the prefix-based sweep
	// below misses them because docker compose names containers `<dir>-<svc>-N`
	// rather than `raioz-<project>-<svc>`.
	stopComposeServices(ctx, deps)

	// Stop host processes from local state
	localState, _ := state.LoadLocalState(projectDir)
	if localState != nil {
		for name, pid := range localState.HostPIDs {
			if pid > 0 {
				// Skip services that already ran a custom stop command.
				if svc, ok := deps.Services[name]; ok && svc.Commands != nil && svc.Commands.Down != "" {
					continue
				}
				logging.InfoWithContext(ctx, "Stopping host process",
					"service", name, "pid", pid)
				killProcessGroup(pid)
			}
		}
		// Clear PIDs from state. If the state file has no Project name —
		// which happens when `up` never wrote it (e.g. older versions, or
		// when a previous down corrupted it) — prefer removing the file
		// outright to writing garbage (`project:""`, zero time) back.
		localState.HostPIDs = make(map[string]int)
		if localState.Project == "" {
			_ = state.RemoveLocalState(projectDir)
		} else {
			_ = state.SaveLocalState(projectDir, localState)
		}
	}

	// Stop raioz containers by label (safe even when names collide with other
	// projects). Legacy name-prefix sweep is kept behind this as a fallback
	// for containers created before labels shipped.
	stopRaiozContainers(ctx, projectName)
	containerPrefix := naming.ContainerPrefix(projectName)
	stopContainersByPrefix(ctx, containerPrefix)

	// Also tumba a container named exactly `<prefix>-<project>` (no trailing
	// service dash). Services declared with `command: make start` — or any
	// user-owned launch script — commonly set container_name that way, and
	// the prefix sweep above skips them because `name=<prefix>-<project>-`
	// is substring-matched, not anchored.
	stopExactContainer(ctx, naming.GetPrefix()+"-"+projectName)

	// Final safety net: anything still labeled for this project after the
	// custom stop command + sweeps means something leaked. Log loudly so
	// the dev sees it instead of silently leaving containers running.
	if leftovers := docker.ListContainersByLabels(ctx, map[string]string{
		naming.LabelManaged: "true",
		naming.LabelProject: projectName,
	}); len(leftovers) > 0 {
		output.PrintWarning(fmt.Sprintf(
			"Project '%s' down finished but these raioz-managed containers survived: %v",
			projectName, leftovers,
		))
	}

	// Stop dependency compose projects
	stopDependencyComposeProjects(ctx, deps, projectName)

	// Stop proxy
	uc.stopProxy(ctx, opts)

	// Clean local state. Same rule as above: never persist a state file
	// with an empty Project — prefer removing it over writing garbage.
	if localState != nil {
		localState.HostPIDs = make(map[string]int)
		if localState.Project == "" {
			_ = state.RemoveLocalState(projectDir)
		} else {
			_ = state.SaveLocalState(projectDir, localState)
		}
	}

	// Clean network
	networkName := deps.Network.GetName()
	if networkName != "" {
		inUse, _ := uc.deps.DockerRunner.IsNetworkInUseWithContext(ctx, networkName)
		if !inUse {
			// Best-effort: network removal may race with another project teardown.
			_ = exec.CommandContext(ctx, runtime.Binary(), "network", "rm", networkName).Run()
			logging.InfoWithContext(ctx, "Network removed", "network", networkName)
		}
	}

	output.PrintSuccess("Project '" + projectName + "' stopped")
	return nil
}

// stopAndRemoveContainer runs docker stop + docker rm -f for a single
// container, capturing stderr. Since the caller has already verified the
// container exists (either via ListContainersByLabels, docker ps prefix
// filter, or inspect), a failure here is meaningful and gets logged as a
// warning with stderr — not silently swallowed like the old `.Run()` path.
// "No such container" is tolerated because a concurrent sweep (or the user)
// could have already removed it between list and stop.
func stopAndRemoveContainer(ctx context.Context, name, source string) {
	if name == "" {
		return
	}
	logging.Info("Stopping container", "name", name, "source", source)
	out, err := exec.CommandContext(ctx, runtime.Binary(), "stop", name).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No such container") {
		logging.Warn("docker stop failed",
			"name", name, "source", source,
			"error", err.Error(), "output", strings.TrimSpace(string(out)))
	}
	rmOut, rmErr := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", name).CombinedOutput()
	if rmErr != nil && !strings.Contains(string(rmOut), "No such container") {
		logging.Warn("docker rm failed",
			"name", name, "source", source,
			"error", rmErr.Error(), "output", strings.TrimSpace(string(rmOut)))
	}
}

// stopRaiozContainers stops and removes every container labeled as belonging
// to the given raioz project (com.raioz.project=<project>). Label-based
// filtering is the only reliable way to distinguish raioz-managed containers
// from unrelated containers that may share a name prefix on the same daemon.
func stopRaiozContainers(ctx context.Context, project string) {
	labels := map[string]string{
		naming.LabelManaged: "true",
		naming.LabelProject: project,
	}
	for _, name := range docker.ListContainersByLabels(ctx, labels) {
		stopAndRemoveContainer(ctx, name, "raioz-label")
	}
}

// stopExactContainer stops and removes a single container by exact name. Used
// as a safety net alongside the prefix/label sweeps for containers whose name
// matches neither (typical when a `command:` entry launches its own compose
// that bakes `container_name: <prefix>-<project>`).
func stopExactContainer(ctx context.Context, name string) {
	if name == "" {
		return
	}
	// Cheap existence probe first: if docker inspect fails, container doesn't
	// exist and we skip the stop/rm calls (they would just emit noise).
	state, _ := docker.GetContainerStatusByName(ctx, name)
	if state == "" {
		return
	}
	stopAndRemoveContainer(ctx, name, "raioz-exact-name")
}

// stopContainersByPrefix stops and removes raioz-managed containers whose
// name starts with the given prefix. The name filter is always combined with
// com.raioz.managed=true so unrelated containers that happen to share a
// prefix (e.g. a user's own `roax-*` fleet when raioz project is `roa`) are
// never touched. Historically this was name-only, which silently killed
// sibling projects on the same machine — see BUG-2.
func stopContainersByPrefix(ctx context.Context, prefix string) {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "ps", "-a",
		"--filter", "name="+prefix,
		"--filter", "label="+naming.LabelManaged+"=true",
		"--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		logging.Warn("docker ps failed while sweeping prefix",
			"prefix", prefix, "error", err.Error())
		return
	}

	names := strings.TrimSpace(string(out))
	if names == "" {
		return
	}

	for _, name := range strings.Split(names, "\n") {
		if name = strings.TrimSpace(name); name != "" {
			stopAndRemoveContainer(ctx, name, "raioz-prefix:"+prefix)
		}
	}
}

// stopDependencyComposeProjects stops compose projects created by image_runner.
// Uses the same COMPOSE_PROJECT_NAME that ImageRunner.Start set so Docker
// Compose can match the containers it originally created.
//
// Shared dependencies (workspace-scoped or with an explicit `name:` override)
// are skipped while OTHER raioz projects in the same workspace still have
// live containers — the last project out tumba the shared deps. Without
// this guard, project A's down would rip postgres out from under project B.
func stopDependencyComposeProjects(ctx context.Context, deps *config.Deps, projectName string) {
	others := otherProjectsActiveInWorkspace(ctx, deps.Workspace, projectName)

	for name, entry := range deps.Infra {
		var override string
		if entry.Inline != nil {
			override = entry.Inline.Name
		}
		if naming.IsSharedDep(override) && others {
			logging.InfoWithContext(ctx, "Keeping shared dependency alive for sibling projects",
				"dep", name, "workspace", deps.Workspace, "leaving_project", projectName)
			continue
		}

		projName := orchestrate.DepComposeProjectName(projectName, name)
		// Compose-based deps: user-supplied fragment(s) + raioz overlay,
		// teardown needs the same list of -f files that Start used.
		var composeArgs []string
		var envFileArgs []string
		if entry.Inline != nil && len(entry.Inline.Compose) > 0 {
			overlay := filepath.Join(
				filepath.Dir(naming.DepComposePath(projectName, name)),
				"raioz-overlay.yml",
			)
			for _, f := range entry.Inline.Compose {
				abs := f
				if a, err := filepath.Abs(f); err == nil {
					abs = a
				}
				composeArgs = append(composeArgs, "-f", abs)
			}
			composeArgs = append(composeArgs, "-f", overlay)
			if entry.Inline.Env != nil {
				for _, f := range entry.Inline.Env.GetFilePaths() {
					if f != "" {
						envFileArgs = append(envFileArgs, "--env-file", f)
					}
				}
			}
		} else {
			composeArgs = []string{"-f", naming.DepComposePath(projectName, name)}
		}
		args := []string{"compose"}
		args = append(args, envFileArgs...)
		args = append(args, "-p", projName)
		args = append(args, composeArgs...)
		args = append(args, "down")
		cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
		_ = cmd.Run() // file might not exist; best-effort teardown
	}
}

// otherProjectsActiveInWorkspace answers the "is anyone else home?" question
// needed to decide whether shared deps can be torn down. Returns true when at
// least one raioz-managed container in the workspace belongs to a project
// other than the one currently being brought down. Shared deps themselves
// have no project label, so they don't falsely signal other-project activity.
func otherProjectsActiveInWorkspace(ctx context.Context, workspace, currentProject string) bool {
	if workspace == "" {
		return false
	}
	names := docker.ListContainersByLabels(ctx, map[string]string{
		naming.LabelManaged:   "true",
		naming.LabelWorkspace: workspace,
	})
	for _, n := range names {
		proj, _ := docker.GetContainerLabel(ctx, n, naming.LabelProject)
		if proj == "" {
			continue // a shared dep itself — not a project consumer
		}
		if proj != currentProject {
			return true
		}
	}
	return false
}

// killProcessGroup kills pid and its descendants (`go run`'s compiled
// binary, `sh -c`'s grandchildren, etc). Cross-platform via the host
// helper; best-effort because the process may already be gone.
func killProcessGroup(pid int) {
	_ = host.KillProcessTree(pid)
}
