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

// runCustomStopCommands executes each service's `stop:` command from raioz.yaml,
// running in the project directory with the service's configured env vars merged
// in (see buildStopCmdEnv — issue 044 regression fix). Returns the sorted list
// of service names whose stop command exited non-zero so the caller can
// surface a visible error block at the end of the down flow and return a
// non-zero exit code; the loop itself is best-effort and never aborts early
// (every service still gets a chance to stop, which is the whole point of
// "down").
func runCustomStopCommands(ctx context.Context, deps *models.Deps, projectDir string) []string {
	var failed []string
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

// buildStopCmdEnv returns the environment slice for a service's custom
// stop command. Starts from os.Environ() so the child inherits PATH,
// HOME, DOCKER_HOST, XDG_*, etc. — without these the typical `make
// stop` / `docker compose down` invocations fail because their
// sub-shell can't find `docker`.
//
// Each declared env-file produces one `RAIOZ_ENV_FILE=<path>` entry
// after the inherited block. Go's exec semantics: if the same key
// appears twice, the last one wins, so overrides are effective even
// though we keep the parent values for diagnostic clarity.
//
// Extracted to ease regression-testing — see down_stop_env_test.go.
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
