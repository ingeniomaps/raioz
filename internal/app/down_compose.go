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

// stopComposeServices tears down compose-based yaml services by invoking
// `docker compose -f <files> down` under the same COMPOSE_PROJECT_NAME scope
// used at `up` time. Required because the default prefix-based cleanup only
// matches containers named `raioz-<project>-<svc>`, and docker compose names
// its own containers `<dir>-<svc>-N` instead.
func stopComposeServices(ctx context.Context, deps *models.Deps) {
	for name, svc := range deps.Services {
		if svc.Source.Command != "" {
			continue
		}

		files := svc.Source.ComposeFiles
		if len(files) == 0 {
			if svc.Source.Path == "" {
				continue
			}
			dr := detect.Detect(svc.Source.Path)
			if dr.Runtime != models.RuntimeCompose || len(dr.ComposeFiles) == 0 {
				continue
			}
			files = dr.ComposeFiles
		}

		overlay := filepath.Join(filepath.Dir(files[0]), ".raioz-overlay.yml")
		callFiles := append([]string{}, files...)
		if _, err := os.Stat(overlay); err == nil {
			callFiles = append(callFiles, overlay)
		}

		spec := docker.JoinComposePaths(callFiles)
		scopedCtx := docker.WithComposeProjectName(
			ctx, orchestrate.ComposeProjectName(deps.Project.Name, name),
		)

		logging.InfoWithContext(ctx, "Stopping compose service",
			"service", name, "files", files,
			"project", orchestrate.ComposeProjectName(deps.Project.Name, name))

		if err := docker.DownWithContext(scopedCtx, spec); err != nil {
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
