package orchestrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/runtime"
)

// register routes RuntimeDockerfile dispatches to DockerfileRunner.
func init() {
	register(models.RuntimeDockerfile, func(d *Dispatcher) runner { return d.dockerfile })
}

// DockerfileRunner handles services that have a Dockerfile but no compose file.
// It builds the image and runs it as a standalone container on the Raioz network.
type DockerfileRunner struct{}

// Start builds the Docker image and runs it.
//
// The container name is deterministic (workspace-project-service), so a
// leftover from a previous run always collides with `docker run --name`
// unless it is reconciled first — see reconcileExisting. A container already
// running short-circuits build+run (idempotent up, same contract as
// ImageRunner.Start), so a re-up reuses it instead of rebuilding; watch mode
// goes through Restart, which removes the container and does rebuild.
func (r *DockerfileRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	reused, err := r.reconcileExisting(ctx, svc.ContainerName)
	if err != nil {
		return err
	}
	if reused {
		return nil
	}

	imageName := "raioz-" + svc.Name

	logging.InfoWithContext(ctx, "Building Docker image",
		"service", svc.Name, "path", svc.Path, "image", imageName)

	// Build
	buildCmd := exec.CommandContext(ctx, runtime.Binary(), "build",
		"-t", imageName,
		"-f", svc.Detection.Dockerfile,
		svc.Path)
	buildCmd.Dir = svc.Path
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build failed: %w\n%s", err, string(output))
	}

	// Prepare run args
	args := []string{"run", "-d",
		"--name", svc.ContainerName,
		"--network", svc.NetworkName,
		"--network-alias", svc.Name,
	}

	// Add host.docker.internal mapping (Linux without Docker Desktop).
	// Gated on runtime.Supports so nerdctl 1.x — which rejects the
	// host-gateway alias — doesn't crash. ADR-046.
	if runtime.Supports(runtime.HostGatewayAlias) {
		args = append(args, "--add-host=host.docker.internal:host-gateway")
	}

	// Add port mappings
	for _, port := range svc.Ports {
		args = append(args, "-p", port)
	}

	// Env files declared via `env:` in raioz.yaml. Emitted before the -e
	// flags so the discovery vars (which raioz computes) take precedence over
	// the file — the calculated host wins over a stale value in the file.
	for _, f := range svc.EnvFilePaths {
		args = append(args, "--env-file", f)
	}

	// Add env vars
	for k, v := range svc.EnvVars {
		args = append(args, "-e", k+"="+v)
	}

	args = append(args, imageName)

	logging.InfoWithContext(ctx, "Starting container",
		"service", svc.Name, "container", svc.ContainerName)

	runCmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	if output, err := runCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker run failed: %w\n%s", err, string(output))
	}

	return nil
}

// reconcileExisting resolves a pre-existing container with the same
// deterministic name before `docker run --name` gets a chance to fail with
// "name is already in use" (exit 125). Three outcomes:
//
//   - absent (inspect fails or reports nothing): nothing to do, build+run.
//   - running: the service is already up; report reuse so Start returns
//     without rebuilding or recreating it.
//   - any other state (exited, created, paused, dead): a leftover from an
//     ordered down, a crash or a reboot. Force-remove it and let Start
//     recreate it.
//
// Mirrors proxy.removeStaleContainer and ImageRunner.Start's reuse probe —
// this runner was the last of the three `docker run` sites without it.
func (r *DockerfileRunner) reconcileExisting(ctx context.Context, containerName string) (bool, error) {
	if containerName == "" {
		return false, nil
	}

	inspect := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}}", containerName)
	out, err := inspect.Output()
	if err != nil {
		return false, nil // container does not exist
	}

	switch state := strings.TrimSpace(string(out)); state {
	case "":
		return false, nil
	case "running":
		logging.InfoWithContext(ctx, "Container already running, reusing",
			"container", containerName)
		return true, nil
	default:
		logging.InfoWithContext(ctx, "Removing stale container",
			"container", containerName, "state", state)
		rm := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", containerName)
		if rmOut, rmErr := rm.CombinedOutput(); rmErr != nil {
			return false, fmt.Errorf("docker rm %s: %w\n%s", containerName, rmErr, string(rmOut))
		}
		return false, nil
	}
}

// Stop stops and removes the container.
func (r *DockerfileRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	// Best-effort: container may already be stopped or removed.
	stopCmd := exec.CommandContext(ctx, runtime.Binary(), "stop", svc.ContainerName)
	_ = stopCmd.Run()

	rmCmd := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", svc.ContainerName)
	_ = rmCmd.Run()

	return nil
}

// Restart stops and starts the container.
func (r *DockerfileRunner) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	if err := r.Stop(ctx, svc); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop dockerfile service",
			"service", svc.Name, "error", err.Error())
	}
	return r.Start(ctx, svc)
}

// Status checks if the container is running.
func (r *DockerfileRunner) Status(ctx context.Context, svc interfaces.ServiceContext) (string, error) {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}}", svc.ContainerName)
	output, err := cmd.Output()
	if err != nil {
		return "stopped", nil
	}
	status := strings.TrimSpace(string(output))
	if status == "running" {
		return "running", nil
	}
	return "stopped", nil
}

// Logs shows container logs.
func (r *DockerfileRunner) Logs(ctx context.Context, svc interfaces.ServiceContext, follow bool, tail int) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, svc.ContainerName)

	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker logs: %w", err)
	}
	return nil
}
