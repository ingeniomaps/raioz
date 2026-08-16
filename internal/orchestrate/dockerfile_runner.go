package orchestrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
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

	// Identify the container as raioz-managed (ADR-001). Everything that
	// recognizes raioz's own containers — down, status, the route GC —
	// matches on these labels, so without them `raioz down` reported
	// success while the service kept running.
	args = append(args, labelArgs(naming.Labels(
		naming.WorkspaceName(), svc.ProjectName, svc.Name, naming.KindService,
	))...)

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

// labelArgs turns a label set into `--label k=v` flags, key-sorted so the
// command line is deterministic across runs (Go map iteration is not).
func labelArgs(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, "--label", k+"="+labels[k])
	}
	return args
}

// reconcileExisting resolves a pre-existing container with the same
// deterministic name before `docker run --name` gets a chance to fail with
// "name is already in use" (exit 125). Three outcomes:
//
//   - absent (inspect fails or reports nothing): nothing to do, build+run.
//   - running AND raioz-managed: the service is already up; report reuse so
//     Start returns without rebuilding or recreating it.
//   - anything else: a leftover from an ordered down, a crash or a reboot —
//     or a container from a raioz old enough to have created it without the
//     ADR-001 labels, which `down` can never stop. Force-remove it and let
//     Start recreate it, labels included.
//
// Mirrors proxy.removeStaleContainer and ImageRunner.Start's reuse probe —
// this runner was the last of the three `docker run` sites without it.
func (r *DockerfileRunner) reconcileExisting(ctx context.Context, containerName string) (bool, error) {
	if containerName == "" {
		return false, nil
	}

	inspect := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}}|{{index .Config.Labels \""+naming.LabelManaged+"\"}}",
		containerName)
	out, err := inspect.Output()
	if err != nil {
		return false, nil // container does not exist
	}

	state, managed, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	if state == "" {
		return false, nil
	}
	if state == "running" && managed == "true" {
		logging.InfoWithContext(ctx, "Container already running, reusing",
			"container", containerName)
		return true, nil
	}

	logging.InfoWithContext(ctx, "Removing stale container",
		"container", containerName, "state", state, "managed", managed == "true")
	rm := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", containerName)
	if rmOut, rmErr := rm.CombinedOutput(); rmErr != nil {
		return false, fmt.Errorf("docker rm %s: %w\n%s", containerName, rmErr, string(rmOut))
	}
	return false, nil
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
