package orchestrate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/runtime"
)

// DockerfileRunner handles services that have a Dockerfile but no compose file.
// It builds the image and runs it as a standalone container on the Raioz network.
type DockerfileRunner struct{}

// Start builds the Docker image and runs it.
func (r *DockerfileRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
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
		return fmt.Errorf("docker build failed: %s\n%s", err, string(output))
	}

	// Prepare run args
	args := []string{"run", "-d",
		"--name", svc.ContainerName,
		"--network", svc.NetworkName,
		"--network-alias", svc.Name,
	}

	// Add host.docker.internal for Linux
	args = append(args, "--add-host=host.docker.internal:host-gateway")

	// Add port mappings
	for _, port := range svc.Ports {
		args = append(args, "-p", port)
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
		return fmt.Errorf("docker run failed: %s\n%s", err, string(output))
	}

	return nil
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
	cmd.Stdout = exec.CommandContext(ctx, "echo").Stdout
	cmd.Stderr = exec.CommandContext(ctx, "echo").Stderr
	return cmd.Run()
}
