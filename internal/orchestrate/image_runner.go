package orchestrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/docker"
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

// DepComposeProjectName returns the docker compose project name used to scope
// a dependency. Format: raioz-<project>-dep-<name>. The "dep-" infix avoids
// collisions with service compose project names produced by ComposeRunner.
// Exported because downOrchestrated needs the same value to tear down what the
// runner created.
func DepComposeProjectName(projectName, depName string) string {
	return naming.GetPrefix() + "-" + projectName + "-dep-" + depName
}

// Start pulls the image and runs it via a generated compose file. When the
// target container already exists in a running state (typical for shared
// dependencies when a sibling project in the same workspace already brought
// it up), Start is a no-op — this is what lets workspace-scoped deps behave
// like a single shared container instead of colliding on name.
//
// Two modes:
//
//  1. Image-based (svc.ExternalComposeFiles empty): generates a minimal
//     compose file from svc.EnvVars["RAIOZ_IMAGE"] and runs it. Legacy
//     behavior for deps declared with just `image:` in raioz.yaml.
//  2. Compose-based (svc.ExternalComposeFiles populated): uses the user's
//     existing compose fragments AS-IS plus an overlay raioz generates
//     (adds the workspace network + raioz labels + aliases). Env
//     interpolation gets the --env-file list from svc.EnvFilePaths.
func (r *ImageRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	if state, _ := r.docker.GetContainerStatusByName(ctx, svc.ContainerName); state == "running" {
		logging.InfoWithContext(ctx, "Dependency already running, reusing",
			"name", svc.Name, "container", svc.ContainerName)
		return nil
	}

	composeSpec, err := r.buildComposeSpec(svc)
	if err != nil {
		return fmt.Errorf("failed to build compose spec for dependency '%s': %w", svc.Name, err)
	}

	logging.InfoWithContext(ctx, "Starting dependency",
		"name", svc.Name, "compose", composeSpec,
		"project", DepComposeProjectName(svc.ProjectName, svc.Name))

	// Scope with an explicit COMPOSE_PROJECT_NAME so --remove-orphans does not
	// sweep containers from other raioz projects that share the same dep name
	// (e.g., two projects both declaring a "postgres" dependency). Attach the
	// env files too so ${VAR} interpolation in user-supplied compose works.
	scopedCtx := docker.WithComposeProjectName(
		ctx, DepComposeProjectName(svc.ProjectName, svc.Name),
	)
	scopedCtx = docker.WithComposeEnvFiles(scopedCtx, svc.EnvFilePaths)
	scopedCtx = docker.WithComposeExtraEnv(scopedCtx, composeInterpolationEnv(svc))
	return r.docker.UpWithContext(scopedCtx, composeSpec)
}

// composeInterpolationEnv produces the extra KEY=value pairs exported to
// docker compose so user-supplied fragments can interpolate ${PROJECT_PREFIX}
// and similar to the values raioz has already computed. Keeps user composes
// generic (works without raioz by falling back to compose `:-` defaults)
// while making raioz orchestration yield predictable container names on the
// workspace-shared convention.
func composeInterpolationEnv(svc interfaces.ServiceContext) map[string]string {
	env := map[string]string{
		// PROJECT_PREFIX matches the convention used in hypixo's .infra
		// fragments: `container_name: ${PROJECT_PREFIX:-postgres}${PROJECT_PREFIX:+-postgres}`.
		// With workspace "hypixo" this resolves to "hypixo-postgres",
		// matching naming.SharedContainer("postgres").
		"PROJECT_PREFIX": naming.GetPrefix(),
		// NETWORK_NAME is commonly referenced by composes that need to
		// know which external network to attach to.
		"NETWORK_NAME": svc.NetworkName,
	}
	// Strip default prefix so a compose's `:-` fallback kicks in for
	// projects without a workspace (otherwise PROJECT_PREFIX would be
	// literal "raioz" and yield "raioz-postgres").
	if env["PROJECT_PREFIX"] == naming.DefaultPrefix {
		delete(env, "PROJECT_PREFIX")
	}
	return env
}

// Stop stops the dependency via its compose file(s) — either the generated
// one (image mode) or the user's fragments + overlay (compose mode).
func (r *ImageRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	composeSpec := r.existingComposeSpec(svc)
	if composeSpec == "" {
		return nil // Nothing on disk, nothing to stop
	}
	scopedCtx := docker.WithComposeProjectName(
		ctx, DepComposeProjectName(svc.ProjectName, svc.Name),
	)
	scopedCtx = docker.WithComposeEnvFiles(scopedCtx, svc.EnvFilePaths)
	scopedCtx = docker.WithComposeExtraEnv(scopedCtx, composeInterpolationEnv(svc))
	return r.docker.DownWithContext(scopedCtx, composeSpec)
}

// Restart restarts the dependency.
func (r *ImageRunner) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	if err := r.Stop(ctx, svc); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop dependency",
			"name", svc.Name, "error", err.Error())
	}
	return r.Start(ctx, svc)
}

// Status checks if the dependency container is running by inspecting it
// directly by name. Going through docker inspect (instead of compose ps)
// keeps status reporting correct even when the generated compose file has
// been moved or the dependency was started outside raioz.
func (r *ImageRunner) Status(ctx context.Context, svc interfaces.ServiceContext) (string, error) {
	state, err := r.docker.GetContainerStatusByName(ctx, svc.ContainerName)
	if err != nil {
		return "unknown", err
	}
	if state == "" {
		return "stopped", nil
	}
	if state == "running" {
		return "running", nil
	}
	return state, nil
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

	// Shared deps get a workspace-scoped label set (no project owner) so
	// `raioz down` of any single project does NOT sweep them — only the last
	// project leaving the workspace tumba the dep. See stopDependencyComposeProjects
	// in down_orchestrated.go for the matching teardown logic.
	labelProject := svc.ProjectName
	if naming.WorkspaceName() != "" || svc.ContainerName != naming.Container(svc.ProjectName, svc.Name) {
		labelProject = ""
	}

	service := map[string]any{
		"image":          imageRef,
		"container_name": svc.ContainerName,
		"restart":        "unless-stopped",
		"labels": naming.Labels(
			naming.WorkspaceName(), labelProject, svc.Name, naming.KindDependency,
		),
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

// buildComposeSpec returns the docker compose spec (one-or-more -f paths)
// to use for this dependency. Image-based deps get a freshly-generated
// file via generateCompose. Compose-based deps (user provided Compose
// fragments in raioz.yaml) get their own files joined with a raioz
// overlay that adds the shared network + raioz labels + alias.
func (r *ImageRunner) buildComposeSpec(svc interfaces.ServiceContext) (string, error) {
	if len(svc.ExternalComposeFiles) == 0 {
		return r.generateCompose(svc)
	}
	overlayPath, err := r.writeInfraOverlay(svc)
	if err != nil {
		return "", fmt.Errorf("failed to write overlay: %w", err)
	}
	// Use ABSOLUTE paths so `docker compose` doesn't try to resolve them
	// relative to its own cwd (which is the raioz process cwd, not the
	// user's project dir — a common source of "file not found" errors).
	var paths []string
	for _, f := range svc.ExternalComposeFiles {
		if abs, err := filepath.Abs(f); err == nil {
			paths = append(paths, abs)
		} else {
			paths = append(paths, f)
		}
	}
	paths = append(paths, overlayPath)
	return docker.JoinComposePaths(paths), nil
}

// existingComposeSpec returns a compose spec the Stop flow can pass to
// docker compose down, reusing whatever disk files buildComposeSpec would
// have produced on a matching Start — so teardown targets the exact same
// containers the startup brought up. Returns "" when nothing is on disk
// (already down or never started), signaling caller to skip.
func (r *ImageRunner) existingComposeSpec(svc interfaces.ServiceContext) string {
	if len(svc.ExternalComposeFiles) > 0 {
		overlay := r.overlayPath(svc)
		// If the overlay is missing the dep was never up (or already
		// cleaned). The user's compose files may still exist on disk but
		// we have nothing to stop.
		if _, err := os.Stat(overlay); err != nil {
			return ""
		}
		var paths []string
		for _, f := range svc.ExternalComposeFiles {
			if abs, err := filepath.Abs(f); err == nil {
				paths = append(paths, abs)
			} else {
				paths = append(paths, f)
			}
		}
		paths = append(paths, overlay)
		return docker.JoinComposePaths(paths)
	}
	path := r.composePath(svc)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// overlayPath is where raioz writes its network+labels overlay for
// compose-based dependencies. Lives in the same per-dep temp dir that
// image-based deps use — keeps the lifecycle symmetrical.
func (r *ImageRunner) overlayPath(svc interfaces.ServiceContext) string {
	dir := filepath.Dir(naming.DepComposePath(svc.ProjectName, svc.Name))
	return filepath.Join(dir, "raioz-overlay.yml")
}

// writeInfraOverlay renders the raioz overlay that layers on top of the
// user's compose fragment(s). Shape:
//
//	services:
//	  <depname>:
//	    networks: [<workspace-net>]
//	    labels: {com.raioz.managed: true, ...}
//	networks:
//	  <workspace-net>: {external: true}
//
// The overlay relies on service name match — the user's compose must
// expose a service whose name matches `dep.Name` in raioz.yaml (the common
// case: `services.postgres:` in postgres.yml pairing with
// `dependencies.postgres:` in raioz.yaml).
func (r *ImageRunner) writeInfraOverlay(svc interfaces.ServiceContext) (string, error) {
	// Shared deps omit com.raioz.project so raioz down of a single project
	// doesn't sweep them — mirrors the same logic in generateCompose.
	labelProject := svc.ProjectName
	if naming.WorkspaceName() != "" ||
		svc.ContainerName != naming.Container(svc.ProjectName, svc.Name) {
		labelProject = ""
	}
	labels := naming.Labels(
		naming.WorkspaceName(), labelProject, svc.Name, naming.KindDependency,
	)

	overlay := map[string]any{
		"services": map[string]any{
			svc.Name: map[string]any{
				"networks": []any{svc.NetworkName, "default"},
				"labels":   labels,
			},
		},
		"networks": map[string]any{
			svc.NetworkName: map[string]any{
				"external": true,
			},
		},
	}

	path := r.overlayPath(svc)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(overlay)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
