package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/docker"
	"raioz/internal/logging"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
)

// stopComposeServices tears down compose-based yaml services by invoking
// `docker compose -f <files> down` under the same COMPOSE_PROJECT_NAME scope
// used at `up` time. Required because the default prefix-based cleanup only
// matches containers named `raioz-<project>-<svc>`, and docker compose names
// its own containers `<dir>-<svc>-N` instead.
func stopComposeServices(ctx context.Context, deps *config.Deps) {
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
			if dr.Runtime != detect.RuntimeCompose || len(dr.ComposeFiles) == 0 {
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

// runCustomStopCommands executes each service's `stop:` command from raioz.yaml,
// running in the project directory with the service's configured env vars merged
// in. Failures are logged but do not abort the overall down flow.
func runCustomStopCommands(ctx context.Context, deps *config.Deps, projectDir string) {
	for name, svc := range deps.Services {
		if svc.Commands == nil || svc.Commands.Down == "" {
			continue
		}

		stopCmd := svc.Commands.Down
		logging.InfoWithContext(ctx, "Running custom stop command",
			"service", name, "command", stopCmd)
		output.PrintInfo("Stopping " + name + " via: " + stopCmd)

		parts := strings.Fields(stopCmd)
		if len(parts) == 0 {
			continue
		}
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		cmd.Dir = projectDir

		if svc.Env != nil {
			for _, f := range svc.Env.GetFilePaths() {
				if f == "" {
					continue
				}
				cmd.Env = append(cmd.Env, "RAIOZ_ENV_FILE="+f)
			}
		}

		if out, err := cmd.CombinedOutput(); err != nil {
			logging.WarnWithContext(ctx, "Custom stop command failed",
				"service", name, "error", err.Error(), "output", string(out))
			output.PrintWarning(fmt.Sprintf("Stop command for %s failed: %v", name, err))
		}
	}
}
