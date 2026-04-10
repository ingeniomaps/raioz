package orchestrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/naming"

	"gopkg.in/yaml.v3"
)

// ImageRunner handles dependencies that are Docker images (postgres, redis, etc.).
// It generates a minimal compose file per dependency and runs it.
type ImageRunner struct {
	docker interfaces.DockerRunner
}

// Start pulls the image and runs it via a generated compose file.
func (r *ImageRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	composePath, err := r.generateCompose(svc)
	if err != nil {
		return fmt.Errorf("failed to generate compose for dependency '%s': %w", svc.Name, err)
	}

	logging.InfoWithContext(ctx, "Starting dependency",
		"name", svc.Name, "compose", composePath)

	return r.docker.UpWithContext(ctx, composePath)
}

// Stop stops the dependency via its compose file.
func (r *ImageRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	composePath := r.composePath(svc)
	if _, err := os.Stat(composePath); err != nil {
		return nil // No compose file, nothing to stop
	}
	return r.docker.DownWithContext(ctx, composePath)
}

// Restart restarts the dependency.
func (r *ImageRunner) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	if err := r.Stop(ctx, svc); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop dependency",
			"name", svc.Name, "error", err.Error())
	}
	return r.Start(ctx, svc)
}

// Status checks if the dependency container is running.
func (r *ImageRunner) Status(ctx context.Context, svc interfaces.ServiceContext) (string, error) {
	composePath := r.composePath(svc)
	if _, err := os.Stat(composePath); err != nil {
		return "stopped", nil
	}
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

// Logs shows dependency container logs.
func (r *ImageRunner) Logs(ctx context.Context, svc interfaces.ServiceContext, follow bool, tail int) error {
	composePath := r.composePath(svc)
	return r.docker.ViewLogsWithContext(ctx, composePath, interfaces.LogsOptions{
		Follow: follow,
		Tail:   tail,
	})
}

// generateCompose creates a minimal docker-compose.yml for a single dependency.
func (r *ImageRunner) generateCompose(svc interfaces.ServiceContext) (string, error) {
	// Build image reference
	imageRef := svc.Name // Will be overridden below

	// Extract image from env vars or use container name pattern
	// The actual image is passed via the ServiceContext from the config
	if img, ok := svc.EnvVars["RAIOZ_IMAGE"]; ok {
		imageRef = img
		delete(svc.EnvVars, "RAIOZ_IMAGE")
	}

	service := map[string]any{
		"image":          imageRef,
		"container_name": svc.ContainerName,
		"restart":        "unless-stopped",
		"networks": map[string]any{
			svc.NetworkName: map[string]any{
				"aliases": []string{svc.Name},
			},
		},
	}

	// Add host.docker.internal for Linux
	service["extra_hosts"] = []string{"host.docker.internal:host-gateway"}

	// Add ports
	if len(svc.Ports) > 0 {
		service["ports"] = svc.Ports
	}

	// Add env vars (excluding internal Raioz vars)
	envList := []string{}
	for k, v := range svc.EnvVars {
		if strings.HasPrefix(k, "RAIOZ_") {
			continue
		}
		if k == "RAIOZ_ENV_FILE" {
			continue
		}
		envList = append(envList, k+"="+v)
	}
	if len(envList) > 0 {
		service["environment"] = envList
	}

	// Add env_file if specified (for loading .env.postgres, etc.)
	if envFile, ok := svc.EnvVars["RAIOZ_ENV_FILE"]; ok && envFile != "" {
		service["env_file"] = []string{envFile}
	}

	compose := map[string]any{
		"services": map[string]any{
			svc.Name: service,
		},
		"networks": map[string]any{
			svc.NetworkName: map[string]any{
				"external": true,
			},
		},
	}

	return r.writeCompose(svc, compose)
}

func (r *ImageRunner) writeCompose(svc interfaces.ServiceContext, compose map[string]any) (string, error) {
	path := r.composePath(svc)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	data, err := yaml.Marshal(compose)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	return path, nil
}

func (r *ImageRunner) composePath(svc interfaces.ServiceContext) string {
	// Each dependency gets its own subdirectory so Docker Compose treats
	// each one as a separate project (project name = directory name).
	// Without this, all deps share the "deps" project and docker compose
	// stops previous services when starting a new one.
	dir := filepath.Dir(naming.DepComposePath(svc.ProjectName, svc.Name))
	return filepath.Join(dir, "docker-compose.yml")
}
