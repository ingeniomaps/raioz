package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/logging"
	"raioz/internal/resilience"
	"raioz/internal/runtime"
)

// ImageExists checks if a Docker image exists locally
func ImageExists(image string) (bool, error) {
	return ImageExistsWithContext(context.Background(), image)
}

// ImageExistsWithContext checks if a Docker image exists locally with context support
func ImageExistsWithContext(ctx context.Context, image string) (bool, error) {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "image", "inspect", image)
	err := cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				// Image not found
				return false, nil
			}
		}
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return false, fmt.Errorf("image inspect timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return false, fmt.Errorf("failed to inspect image: %w", err)
	}

	return true, nil
}

// PullImage pulls a Docker image
func PullImage(image string) error {
	return PullImageWithContext(context.Background(), image)
}

// PullImageWithContext pulls a Docker image with context support
func PullImageWithContext(ctx context.Context, image string) error {
	logging.Info("Pulling Docker image", "image", image)

	// Use circuit breaker and retry logic for docker pull
	dockerCB := resilience.GetDockerCircuitBreaker()
	retryConfig := resilience.DockerRetryConfig()

	pullOp := fmt.Sprintf("docker pull %s", image)
	return resilience.RetryWithContext(ctx, retryConfig, pullOp, func(ctx context.Context) error {
		return dockerCB.ExecuteWithContext(ctx, pullOp, func(ctx context.Context) error {
			// Create context with timeout
			timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerPullTimeout)
			defer cancel()

			cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "pull", image)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			return exectimeout.HandleTimeoutError(timeoutCtx, err, "docker pull", exectimeout.DockerPullTimeout)
		})
	})
}

// EnsureImage ensures that a Docker image exists locally, pulling it if necessary
func EnsureImage(image string) error {
	return EnsureImageWithContext(context.Background(), image)
}

// EnsureImageWithContext ensures that a Docker image exists locally, pulling it if necessary, with context support
func EnsureImageWithContext(ctx context.Context, image string) error {
	exists, err := ImageExistsWithContext(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to check image existence: %w", err)
	}

	if exists {
		return nil // Image already exists
	}

	// Image doesn't exist, pull it
	if err := PullImageWithContext(ctx, image); err != nil {
		return fmt.Errorf("failed to pull image '%s': %w", image, err)
	}

	return nil
}

// BuildImageName constructs the full image name from image and tag
func BuildImageName(image string, tag string) string {
	if tag == "" {
		return image
	}
	return fmt.Sprintf("%s:%s", image, tag)
}

// ValidateServiceImages validates all images for services with source.kind == "image"
func ValidateServiceImages(deps *config.Deps) error {
	return ValidateServiceImagesWithContext(context.Background(), deps)
}

// ValidateServiceImagesWithContext validates all images for services with source.kind == "image" with context support
func ValidateServiceImagesWithContext(ctx context.Context, deps *config.Deps) error {
	for name, svc := range deps.Services {
		if svc.Source.Kind == "image" {
			image := BuildImageName(svc.Source.Image, svc.Source.Tag)
			if err := EnsureImageWithContext(ctx, image); err != nil {
				return fmt.Errorf("service %s: %w", name, err)
			}
		}
	}
	return nil
}

// ValidateInfraImages validates all images for infra
func ValidateInfraImages(deps *config.Deps) error {
	return ValidateInfraImagesWithContext(context.Background(), deps)
}

// ValidateInfraImagesWithContext validates all images for infra with context support
func ValidateInfraImagesWithContext(ctx context.Context, deps *config.Deps) error {
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		// Compose-mode deps delegate image management to the user's
		// compose file — `docker compose up` pulls whatever the fragment
		// declares. Skip raioz's own pre-flight pull so we don't try to
		// pull an empty `:latest` reference.
		if len(entry.Inline.Compose) > 0 {
			continue
		}
		image := BuildImageName(entry.Inline.Image, entry.Inline.Tag)
		if err := EnsureImageWithContext(ctx, image); err != nil {
			return fmt.Errorf("infra %s: %w", name, err)
		}
	}
	return nil
}

// ValidateAllImages validates all images (services and infra) before compose generation
func ValidateAllImages(deps *config.Deps) error {
	return ValidateAllImagesWithContext(context.Background(), deps)
}

// ValidateAllImagesWithContext validates all images (services and infra) before compose generation with context support
func ValidateAllImagesWithContext(ctx context.Context, deps *config.Deps) error {
	// Validate service images
	if err := ValidateServiceImagesWithContext(ctx, deps); err != nil {
		return fmt.Errorf("service image validation failed: %w", err)
	}

	// Validate infra images
	if err := ValidateInfraImagesWithContext(ctx, deps); err != nil {
		return fmt.Errorf("infra image validation failed: %w", err)
	}

	return nil
}

// GetImageInfo returns information about a Docker image
func GetImageInfo(image string) (map[string]string, error) {
	return GetImageInfoWithContext(context.Background(), image)
}

// GetImageInfoWithContext returns information about a Docker image with context support
func GetImageInfoWithContext(ctx context.Context, image string) (map[string]string, error) {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "image", "inspect", image, "--format",
		"{{.Id}}|{{.RepoTags}}|{{.Created}}")
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return nil, fmt.Errorf("image inspect timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	info := make(map[string]string)

	if len(parts) >= 1 {
		info["id"] = parts[0]
	}
	if len(parts) >= 2 {
		info["tags"] = parts[1]
	}
	if len(parts) >= 3 {
		info["created"] = parts[2]
	}

	return info, nil
}

// exposedPortCache memoizes ExposedPorts lookups for the lifetime of the
// process. Image contents don't change mid-session, so a simple map keyed
// by the full image reference is enough — no TTL, no eviction.
var exposedPortCache sync.Map // map[string]exposedPortEntry

type exposedPortEntry struct {
	port int
	err  error
}

// GetImageExposedPort returns the first TCP port declared via EXPOSE in the
// image's manifest, or 0 + error when no TCP port is declared, the image is
// not present locally, or docker inspect fails. Results are cached per
// image reference.
//
// Pure inspect — does not pull. Callers that need the image present must
// call EnsureImage first.
func GetImageExposedPort(ctx context.Context, image string) (int, error) {
	if cached, ok := exposedPortCache.Load(image); ok {
		entry := cached.(exposedPortEntry)
		return entry.port, entry.err
	}

	port, err := inspectExposedPort(ctx, image)
	exposedPortCache.Store(image, exposedPortEntry{port: port, err: err})
	return port, err
}

func inspectExposedPort(ctx context.Context, image string) (int, error) {
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(),
		"image", "inspect", image,
		"--format", "{{json .Config.ExposedPorts}}")
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return 0, fmt.Errorf("image inspect timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return 0, fmt.Errorf("image inspect failed: %w", err)
	}

	return parseExposedPorts(strings.TrimSpace(string(output)))
}

// parseExposedPorts accepts the JSON docker emits for
// `.Config.ExposedPorts` (e.g. `{"5432/tcp":{},"5432/udp":{}}`, `{}`, or
// `null`) and returns the numerically lowest TCP port. Lowest-first keeps
// the choice deterministic for images that expose several ports (pgAdmin:
// 80 + 443 → picks 80).
func parseExposedPorts(raw string) (int, error) {
	if raw == "" || raw == "null" || raw == "{}" {
		return 0, fmt.Errorf("no ExposedPorts declared")
	}

	var ports map[string]struct{}
	if err := json.Unmarshal([]byte(raw), &ports); err != nil {
		return 0, fmt.Errorf("unmarshal ExposedPorts: %w", err)
	}

	var tcp []int
	for key := range ports {
		proto := "tcp"
		portStr := key
		if idx := strings.Index(key, "/"); idx >= 0 {
			portStr = key[:idx]
			proto = key[idx+1:]
		}
		if proto != "tcp" {
			continue
		}
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 {
			continue
		}
		tcp = append(tcp, p)
	}
	if len(tcp) == 0 {
		return 0, fmt.Errorf("no TCP port in ExposedPorts")
	}
	sort.Ints(tcp)
	return tcp[0], nil
}
