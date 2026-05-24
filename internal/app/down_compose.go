package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sort"

	"raioz/internal/detect"
	"raioz/internal/docker"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
)

// composeInvocation describes how to drive `docker compose` for a yaml service
// that runs as its own compose stack, from outside the up flow: the `-f` file
// spec (overlay appended when present) and the COMPOSE_PROJECT_NAME scope used
// at up time.
//
// Shared by `down` (stopComposeServices) and `logs` (showComposeServiceLogs):
// docker compose names its containers `<scope>-<svc>-N` (and the user can
// override with `container_name:`), never `raioz-<project>-<svc>`, so the only
// reliable way to reach the stack is `docker compose -f <files>` under the
// same scope.
type composeInvocation struct {
	spec        string // colon-joined compose-file paths for docker.ComposeFileArgs
	projectName string // COMPOSE_PROJECT_NAME scope
}

// resolveComposeInvocation works out how to address a yaml service's compose
// stack. Returns ok=false for host-command services and for services with no
// resolvable compose files (Dockerfile / image services, which raioz names
// itself and addresses by container name). The file-resolution logic mirrors
// ResolveServiceDetection's compose precedence: explicit `compose:` files
// first, else a directory scan that must come back RuntimeCompose.
func resolveComposeInvocation(svc models.Service, project, name string) (composeInvocation, bool) {
	if svc.Source.Command != "" {
		return composeInvocation{}, false
	}

	files := svc.Source.ComposeFiles
	if len(files) == 0 {
		if svc.Source.Path == "" {
			return composeInvocation{}, false
		}
		dr := detect.Detect(svc.Source.Path)
		if dr.Runtime != models.RuntimeCompose || len(dr.ComposeFiles) == 0 {
			return composeInvocation{}, false
		}
		files = dr.ComposeFiles
	}

	// Append raioz's overlay when up wrote one — keeps the invocation scoped
	// to the same merged config (extra_hosts, network, etc.) that up applied.
	overlay := filepath.Join(filepath.Dir(files[0]), ".raioz-overlay.yml")
	callFiles := append([]string{}, files...)
	if _, err := os.Stat(overlay); err == nil {
		callFiles = append(callFiles, overlay)
	}

	return composeInvocation{
		spec:        docker.JoinComposePaths(callFiles),
		projectName: orchestrate.ComposeProjectName(project, name),
	}, true
}

// showComposeServiceLogs streams logs for a service that runs as its own
// compose stack, reusing the `-f <files>` spec + COMPOSE_PROJECT_NAME scope
// that up and down apply. Without the scope, docker compose would default to
// the compose-file directory basename and miss raioz's per-service project.
// Lives here (next to stopComposeServices, the other consumer of the shared
// resolver) so the app-layer docker import stays on one ADR-029 baseline file.
func showComposeServiceLogs(
	ctx context.Context, proj *YAMLProject, name string, follow bool, tail int,
) error {
	svc, ok := proj.Deps.Services[name]
	if !ok {
		return fmt.Errorf("unknown service %q", name)
	}
	inv, ok := resolveComposeInvocation(svc, proj.ProjectName, name)
	if !ok {
		return fmt.Errorf("service %q is not a resolvable compose stack", name)
	}
	scopedCtx := docker.WithComposeProjectName(ctx, inv.projectName)
	if err := docker.ViewLogsWithContext(scopedCtx, inv.spec, docker.LogsOptions{
		Follow: follow,
		Tail:   tail,
	}); err != nil {
		return fmt.Errorf("docker compose logs %s: %w", name, err)
	}
	return nil
}

// stopComposeServices tears down compose-based yaml services by invoking
// `docker compose -f <files> down` under the same COMPOSE_PROJECT_NAME scope
// used at `up` time. Required because the default prefix-based cleanup only
// matches containers named `raioz-<project>-<svc>`, and docker compose names
// its own containers `<dir>-<svc>-N` instead.
func stopComposeServices(ctx context.Context, deps *models.Deps) {
	for name, svc := range deps.Services {
		inv, ok := resolveComposeInvocation(svc, deps.Project.Name, name)
		if !ok {
			continue
		}

		scopedCtx := docker.WithComposeProjectName(ctx, inv.projectName)

		logging.InfoWithContext(ctx, "Stopping compose service",
			"service", name, "spec", inv.spec, "project", inv.projectName)

		if err := docker.DownWithContext(scopedCtx, inv.spec); err != nil {
			logging.WarnWithContext(ctx, "Compose service down failed",
				"service", name, "error", err.Error())
			output.PrintWarning(fmt.Sprintf("Failed to stop compose service %s: %v", name, err))
		}
	}
}

// runCustomStopCommands runs each service's `stop:` and returns the
// sorted names of failures. Best-effort: every service still gets a
// stop attempt so a partial down doesn't strand later services.
func runCustomStopCommands(ctx context.Context, deps *models.Deps, projectDir string) []string {
	var failed []string
	for name, svc := range deps.Services {
		if svc.Commands == nil || svc.Commands.Down == "" {
			continue
		}

		stopCmd := svc.Commands.Down
		logging.InfoWithContext(ctx, "Running custom stop command",
			"service", name, "command", stopCmd)
		output.PrintInfo(i18n.T("output.stopping_via", name, stopCmd))

		parts := strings.Fields(stopCmd)
		if len(parts) == 0 {
			continue
		}
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		cmd.Dir = projectDir
		cmd.Env = buildStopCmdEnv(svc)

		if out, err := cmd.CombinedOutput(); err != nil {
			logging.WarnWithContext(ctx, "Custom stop command failed",
				"service", name, "error", err.Error(), "output", string(out))
			output.PrintWarning(fmt.Sprintf("Stop command for %s failed: %v", name, err))
			failed = append(failed, name)
		}
	}
	sort.Strings(failed)
	return failed
}

// Seeds the env from os.Environ() so the child sees PATH/DOCKER_HOST/
// etc.; without this `make stop` wrappers can't find `docker`.
// RAIOZ_ENV_FILE overrides come after so they win on duplicate keys.
func buildStopCmdEnv(svc models.Service) []string {
	env := os.Environ()
	if svc.Env == nil {
		return env
	}
	for _, f := range svc.Env.GetFilePaths() {
		if f == "" {
			continue
		}
		env = append(env, "RAIOZ_ENV_FILE="+f)
	}
	return env
}
