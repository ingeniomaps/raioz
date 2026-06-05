package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/domain/models"
	exectimeout "raioz/internal/exec"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/refcount"
	"raioz/internal/runtime"
	"raioz/internal/state"
)

// downSelectiveServices stops only the named services / dependencies and
// returns. Network, proxy, and the local state file are left intact so the
// rest of the project keeps running.
//
// The caller is responsible for having matched naming.SetPrefix to the
// project's workspace before invoking this — otherwise label sweeps and
// container lookups won't agree on container names.
func (uc *DownUseCase) downSelectiveServices(
	ctx context.Context,
	deps *models.Deps,
	projectDir, projectName string,
	requested []string,
) error {
	// Workspace lock — selective down rewrites .raioz.state.json
	// (removes the targeted services). Without the lock a concurrent
	// `raioz up --watch` save-state can race and reintroduce the
	// service raioz just removed.
	release, err := acquireDownSelectiveLock(ctx, uc.deps, projectName)
	if err != nil {
		return err
	}
	defer release()

	// Resolve each requested name to a kind ("service" or "dep") and bail
	// loudly if anything is unknown — silent ignore would mask typos in
	// the requested list, which is exactly the bug the selective path
	// is supposed to avoid.
	type target struct {
		name string
		kind string // "service" | "dep"
	}
	var targets []target
	var unknown []string
	for _, n := range requested {
		switch {
		case hasService(deps, n):
			targets = append(targets, target{name: n, kind: "service"})
		case hasInfra(deps, n):
			targets = append(targets, target{name: n, kind: "dep"})
		default:
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		known := append(declaredServiceNames(deps), declaredInfraNames(deps)...)
		return fmt.Errorf(
			"down: unknown service or dependency: %s (declared in raioz.yaml: %s)",
			strings.Join(unknown, ", "), strings.Join(known, ", "),
		)
	}

	output.PrintProgress(fmt.Sprintf(
		"Stopping %d target(s) from project %q...", len(targets), projectName,
	))

	localState, _ := state.LoadLocalState(projectDir)

	for _, t := range targets {
		switch t.kind {
		case "service":
			stopSelectiveService(ctx, deps, projectDir, projectName, t.name, localState)
		case "dep":
			stopSelectiveDep(ctx, deps, projectName, t.name, localState)
		}
	}

	// Persist the host PID map with the killed services removed. We do NOT
	// touch the rest of the file — other services may still be running.
	if localState != nil {
		_ = state.SaveLocalState(projectDir, localState)
	}

	output.PrintSuccess(fmt.Sprintf(
		"Stopped %d target(s) from %q (rest of project untouched)",
		len(targets), projectName,
	))
	return nil
}

// stopSelectiveService stops a single service by name, honoring the
// service-level resolution order: custom `stop:` command first, then host
// PID kill, then label-based container sweep for compose / image services.
// Mutates localState by removing the service's PID entry.
func stopSelectiveService(
	ctx context.Context,
	deps *models.Deps,
	projectDir, projectName, name string,
	localState *models.LocalState,
) {
	svc, ok := deps.Services[name]
	if !ok {
		return
	}

	customStopOK := false
	if svc.Commands != nil && svc.Commands.Down != "" {
		customStopOK = runStopCommand(ctx, name, svc.Commands.Down, projectDir, svc.Source.Path)
	}
	// Fall back to PID kill when the custom command never ran OR ran but
	// failed (non-zero exit, missing binary, etc.). The previous version
	// logged "falling back to PID kill" without actually doing it, leaving
	// the service alive after a failed stop:.
	if !customStopOK && localState != nil {
		if pid, ok := localState.HostPIDs[name]; ok && pid > 0 {
			killProcessGroup(pid)
			logging.InfoWithContext(ctx, "Stopped host process",
				"service", name, "pid", pid)
			sweepLauncherOrphans(ctx, deps, projectDir, name)
		}
	}

	if localState != nil {
		delete(localState.HostPIDs, name)
	}

	// Sweep any container with the matching service label — covers Docker
	// services brought up by compose / dockerfile runners.
	for _, c := range listContainersByLabelsFn(ctx, map[string]string{
		naming.LabelManaged: "true",
		naming.LabelProject: projectName,
		naming.LabelService: name,
	}) {
		stopAndRemoveContainer(ctx, c, "selective-service-label")
	}

	output.PrintSuccess(name)
}

// stopSelectiveDep tears down a single dependency's compose project,
// reusing the same scope ImageRunner used. Skips when the dep is shared
// across the workspace AND another project still uses it — same rule as
// the bulk teardown path.
//
// Sibling-mode deps (issue #26) are also skipped: mode A always (the
// sibling raioz project owns the lifecycle), mode B only when this dep
// was deferred at up time per LocalState. In both cases we never
// created a container locally, so there's nothing to tear down — and
// silently running `compose down` on a non-existent project would
// falsely report success.
func stopSelectiveDep(
	ctx context.Context,
	deps *models.Deps,
	projectName, name string,
	localState *models.LocalState,
) {
	entry, ok := deps.Infra[name]
	if !ok {
		return
	}

	// Mode A: project: ../sibling — sibling is the runtime.
	if entry.Inline != nil && entry.Inline.Project != "" {
		output.PrintInfo(fmt.Sprintf(
			"%s: sibling-owned (project: %s) — leaving it up. "+
				"Run `cd %s && raioz down` to stop it from its own project.",
			name, entry.Inline.Project, entry.Inline.Project,
		))
		return
	}
	// Mode B deferred: image+siblingProject and last `up` deferred
	// because the sibling was active. The local image was never started.
	if localState != nil && localState.IsDeferred(name) {
		output.PrintInfo(fmt.Sprintf(
			"%s: deferred to sibling at up time — nothing to tear down here",
			name,
		))
		return
	}

	var override string
	if entry.Inline != nil {
		override = entry.Inline.Name
	}
	if naming.IsSharedDep(override) {
		// Selective down releases only this dep, so drop just this project's
		// reference to it and keep it up while any other project still
		// references it (issue 069).
		remaining, err := refcount.DropRef(deps.Workspace, name, projectName)
		if err != nil {
			logging.WarnWithContext(ctx, "Shared dep refcount drop failed",
				"dep", name, "error", err.Error())
		}
		if len(remaining) > 0 {
			output.PrintInfo(fmt.Sprintf(
				"%s: shared with sibling projects, leaving it up", name,
			))
			return
		}
	}

	projName := naming.DepComposeProjectNameFor(projectName, name, naming.IsSharedDep(override))
	// Tear down by `-p` alone: compose resolves the project from the engine
	// labels, so the original -f fragments (TMPDIR-bound, possibly gone) are
	// not needed. Reconstructing them and swallowing the error leaked deps.
	args := []string{"compose", "-p", projName, "down", "--remove-orphans"}
	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		logging.WarnWithContext(ctx, "Dependency teardown failed",
			"dep", name, "project", projName,
			"error", err.Error(), "output", string(out))
	}

	output.PrintSuccess(name)
}

// runStopCommand executes the user-declared `stop:` for a service.
// Returns true when the command succeeded; the caller uses the bool to
// decide whether the PID-kill fallback needs to fire. An empty command
// returns false so the caller treats it as "stop didn't actually run".
func runStopCommand(
	ctx context.Context,
	serviceName, command, projectDir, servicePath string,
) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerComposeDownTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, parts[0], parts[1:]...)
	cmd.Dir = projectDir
	if servicePath != "" {
		abs := servicePath
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(projectDir, servicePath)
		}
		cmd.Dir = abs
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		logging.WarnWithContext(ctx,
			"Custom stop command failed; falling back to PID kill",
			"service", serviceName, "command", command,
			"error", err.Error(),
			"output", strings.TrimSpace(string(out)),
		)
		return false
	}
	return true
}

// hasService / hasInfra / serviceNames / declaredInfraNames are tiny presence
// helpers kept here (instead of free functions in down.go) so the
// selective path stays self-contained. Operating on map keys directly
// would inline the same logic at every call site.
func hasService(deps *models.Deps, name string) bool {
	_, ok := deps.Services[name]
	return ok
}

func hasInfra(deps *models.Deps, name string) bool {
	_, ok := deps.Infra[name]
	return ok
}

func declaredServiceNames(deps *models.Deps) []string {
	out := make([]string, 0, len(deps.Services))
	for n := range deps.Services {
		out = append(out, n)
	}
	return out
}

func declaredInfraNames(deps *models.Deps) []string {
	out := make([]string, 0, len(deps.Infra))
	for n := range deps.Infra {
		out = append(out, n)
	}
	return out
}

// hostExecForce wraps os/exec.CommandContext for use by host-process kill.
// Defined as a thin alias because tests stub it; production calls go
// straight through to exec.
var _ = host.KillProcessTree // keep host import used for selective path consumers
