package orchestrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

// Start runs `docker compose -f <original> -f <overlay> up -d` on the service's compose file.
func (r *ComposeRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	overlayPath, err := r.createNetworkOverlay(svc)
	if err != nil {
		return fmt.Errorf("failed to create network overlay: %w", err)
	}

	// Build compose command: use the original file + overlay
	composePath := svc.Detection.ComposeFile
	logging.InfoWithContext(ctx, "Starting compose service",
		"service", svc.Name, "compose", composePath, "network", svc.NetworkName)

	return r.docker.UpWithContext(ctx, composePath+":"+overlayPath)
}

// Stop stops the compose project.
func (r *ComposeRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	composePath := svc.Detection.ComposeFile
	overlayPath := r.overlayPath(svc)

	if _, err := os.Stat(overlayPath); err == nil {
		return r.docker.DownWithContext(ctx, composePath+":"+overlayPath)
	}
	return r.docker.DownWithContext(ctx, composePath)
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
	composePath := svc.Detection.ComposeFile
	statuses, err := r.docker.GetServicesStatusWithContext(ctx, composePath)
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
	return r.docker.ViewLogsWithContext(ctx, svc.Detection.ComposeFile, interfaces.LogsOptions{
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
