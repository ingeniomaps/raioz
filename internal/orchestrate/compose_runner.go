package orchestrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"

	"gopkg.in/yaml.v3"
)

// ComposeRunner handles services that have their own docker-compose.yml.
// It does NOT modify the user's compose file. Instead, it creates a thin
// overlay that adds the Raioz network, and runs both together.
type ComposeRunner struct {
	docker interfaces.DockerRunner
}

// composePaths returns the user-declared compose files for this service.
// Multi-file yaml (`compose: [a.yaml, b.yaml]`) surfaces here as a slice; for
// auto-detected services it is a single-element slice derived from ComposeFile.
func composePaths(svc interfaces.ServiceContext) []string {
	if len(svc.Detection.ComposeFiles) > 0 {
		return svc.Detection.ComposeFiles
	}
	if svc.Detection.ComposeFile != "" {
		return []string{svc.Detection.ComposeFile}
	}
	return nil
}

// ComposeProjectName returns the docker compose project name used to scope
// a service. Format: raioz-<project>-<service>. Exported because downOrchestrated
// needs the same value to tear down what the runner created.
func ComposeProjectName(projectName, serviceName string) string {
	return "raioz-" + projectName + "-" + serviceName
}

// scopedContext attaches an explicit COMPOSE_PROJECT_NAME to the context so
// `docker compose --remove-orphans` does not sweep containers from unrelated
// projects that happen to share the compose file directory basename.
func scopedContext(ctx context.Context, svc interfaces.ServiceContext) context.Context {
	return docker.WithComposeProjectName(ctx, ComposeProjectName(svc.ProjectName, svc.Name))
}

// Start runs `docker compose -f <a> -f <b> ... -f <overlay> up -d`.
func (r *ComposeRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	overlayPath, err := r.createNetworkOverlay(svc)
	if err != nil {
		return fmt.Errorf("failed to create network overlay: %w", err)
	}

	files := append(composePaths(svc), overlayPath)
	spec := docker.JoinComposePaths(files)

	logging.InfoWithContext(ctx, "Starting compose service",
		"service", svc.Name, "compose", spec, "network", svc.NetworkName,
		"project", ComposeProjectName(svc.ProjectName, svc.Name))

	return r.docker.UpWithContext(scopedContext(ctx, svc), spec)
}

// Stop stops the compose project, honoring overlay + multi-file specs.
func (r *ComposeRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	files := composePaths(svc)
	overlayPath := r.overlayPath(svc)

	if _, err := os.Stat(overlayPath); err == nil {
		files = append(files, overlayPath)
	}
	return r.docker.DownWithContext(scopedContext(ctx, svc), docker.JoinComposePaths(files))
}

// Restart restarts the compose project.
func (r *ComposeRunner) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	if err := r.Stop(ctx, svc); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop compose service, continuing with start",
			"service", svc.Name, "error", err.Error())
	}
	return r.Start(ctx, svc)
}

// Status checks if compose services are running.
func (r *ComposeRunner) Status(ctx context.Context, svc interfaces.ServiceContext) (string, error) {
	spec := docker.JoinComposePaths(composePaths(svc))
	statuses, err := r.docker.GetServicesStatusWithContext(scopedContext(ctx, svc), spec)
	if err != nil {
		return "unknown", err
	}
	for _, status := range statuses {
		if status == "running" {
			return "running", nil
		}
	}
	return "stopped", nil
}

// Logs streams logs from the compose project.
func (r *ComposeRunner) Logs(ctx context.Context, svc interfaces.ServiceContext, follow bool, tail int) error {
	spec := docker.JoinComposePaths(composePaths(svc))
	return r.docker.ViewLogsWithContext(scopedContext(ctx, svc), spec, interfaces.LogsOptions{
		Follow: follow,
		Tail:   tail,
	})
}

// createNetworkOverlay generates a small compose override that adds the Raioz network
// to all services in the original compose file, without modifying the original.
func (r *ComposeRunner) createNetworkOverlay(svc interfaces.ServiceContext) (string, error) {
	overlay := map[string]any{
		"networks": map[string]any{
			svc.NetworkName: map[string]any{
				"external": true,
			},
		},
	}

	// Read original compose to get service names
	services, err := r.docker.GetAvailableServicesWithContext(context.Background(), svc.Detection.ComposeFile)
	if err != nil {
		// If we can't read services, create a generic overlay
		return r.writeOverlay(svc, overlay)
	}

	// Add network to each service
	svcOverrides := make(map[string]any)
	for _, name := range services {
		svcOverrides[name] = map[string]any{
			"networks": []string{svc.NetworkName, "default"},
		}
	}
	overlay["services"] = svcOverrides

	return r.writeOverlay(svc, overlay)
}

func (r *ComposeRunner) writeOverlay(svc interfaces.ServiceContext, overlay map[string]any) (string, error) {
	path := r.overlayPath(svc)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	data, err := yaml.Marshal(overlay)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	return path, nil
}

func (r *ComposeRunner) overlayPath(svc interfaces.ServiceContext) string {
	dir := filepath.Dir(svc.Detection.ComposeFile)
	return filepath.Join(dir, ".raioz-overlay.yml")
}
