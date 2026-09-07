package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"raioz/internal/domain/models"
	exectimeout "raioz/internal/exec"
	"raioz/internal/git"
	"raioz/internal/runtime"
	"raioz/internal/workspace"
)

// ServiceInfo is what the global state records about a service.
//
// It carries only what a consumer reads. Health, uptime, CPU and memory
// used to be collected here too — including a `docker stats` on every
// `up` — and then dropped on the floor: the sole consumer narrows this
// to models.ServiceInfo{Status, Version, Image}. The dashboard, the one
// place those numbers are shown, queries Docker itself.
type ServiceInfo struct {
	Name    string
	Status  string // State.Status verbatim, or "stopped" with no container
	Image   string // image name and tag
	Version string // commit SHA or image digest
}

// ContainerInspect contains docker inspect output structure
type ContainerInspect struct {
	State struct {
		Status string `json:"Status"`
		Health *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Image string   `json:"Image"`
		Env   []string `json:"Env"`
	} `json:"Config"`
	Image string `json:"Image"` // image digest
}

// GetContainerLabel returns the value of a single Docker label on a container
// looked up by name. Returns "" with no error when the container does not
// exist or when the label is absent. Useful for reasoning about ownership
// ("is this container labeled for project X?") without parsing full inspect
// JSON. Errors are returned only on timeout.
func GetContainerLabel(ctx context.Context, name, key string) (string, error) {
	if name == "" || key == "" {
		return "", nil
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	// Dotted label keys (e.g. com.raioz.project) need `index` in a Go
	// template — dot-access would misparse them as field chains.
	format := fmt.Sprintf(`{{ index .Config.Labels %q }}`, key)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(),
		"inspect", "--format", format, name)
	out, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return "", fmt.Errorf("docker inspect timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return "", nil
	}
	value := strings.TrimSpace(string(out))
	if value == "<no value>" {
		// Go template returns "<no value>" when the key is absent.
		return "", nil
	}
	return value, nil
}

// GetContainerStatusByName returns the raw Docker state of a container
// (running, exited, created, paused, restarting, removing, dead) looked up
// directly via `docker inspect --format '{{.State.Status}}' <name>`.
// Returns "" with no error when the container does not exist. An error is
// only returned on timeout. Use this when the caller knows the container
// name but does not have a compose file available (e.g. status of services
// started via non-compose runners).
func GetContainerStatusByName(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", nil
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(),
		"inspect", "--format", "{{.State.Status}}", name)
	out, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return "", fmt.Errorf("docker inspect timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// GetContainerNameWithContext returns the container name for a service with context support
func GetContainerNameWithContext(ctx context.Context, composePath string, serviceName string) (string, error) {
	// Validate path to prevent command injection
	if err := ValidateComposePath(composePath); err != nil {
		return "", fmt.Errorf("invalid compose path: %w", err)
	}

	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerStatusTimeout)
	defer cancel()

	// Use docker compose ps to get container name
	cmd := exec.CommandContext(timeoutCtx,
		runtime.Binary(), "compose", "-f", composePath,
		"ps", "-q", serviceName,
	)
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return "", fmt.Errorf("docker compose ps timed out after %v", exectimeout.DockerStatusTimeout)
		}
		return "", fmt.Errorf("failed to get container name: %w", err)
	}

	containerID := strings.TrimSpace(string(output))
	if containerID == "" {
		return "", nil // Service not running
	}

	// Get container name from ID
	cmd2 := exec.CommandContext(timeoutCtx, runtime.Binary(), "inspect", "-f", "{{.Name}}", containerID)
	nameOutput, err := cmd2.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return "", fmt.Errorf("docker inspect timed out after %v", exectimeout.DockerInspectTimeout)
		}
		return "", fmt.Errorf("failed to get container name: %w", err)
	}

	name := strings.TrimSpace(string(nameOutput))
	// Remove leading slash
	name = strings.TrimPrefix(name, "/")

	return name, nil
}

// GetServicesInfoWithContext retrieves information for all services with
// context support. One batched docker inspect covers every container.
func GetServicesInfoWithContext(
	ctx context.Context,
	composePath string,
	serviceNames []string,
	projectName string,
	services map[string]models.Service,
	ws *workspace.Workspace,
) (map[string]*ServiceInfo, error) {
	result := make(map[string]*ServiceInfo, len(serviceNames))

	// Step 1: resolve container names for all services
	containerMap := make(map[string]string) // serviceName → containerName
	for _, name := range serviceNames {
		cn, err := GetContainerNameWithContext(ctx, composePath, name)
		if err != nil || cn == "" {
			result[name] = &ServiceInfo{Name: name, Status: "stopped"}
			continue
		}
		containerMap[name] = cn
	}

	if len(containerMap) == 0 {
		return result, nil
	}

	// Step 2: single batch docker inspect for all running containers
	containerNames := make([]string, 0, len(containerMap))
	for _, cn := range containerMap {
		containerNames = append(containerNames, cn)
	}
	inspectMap := batchInspect(ctx, containerNames)

	// Step 3: assemble ServiceInfo for each service
	for _, name := range serviceNames {
		if _, ok := containerMap[name]; !ok {
			continue // already set to stopped above
		}
		cn := containerMap[name]
		info := &ServiceInfo{Name: name, Status: "running"}

		if inspect, ok := inspectMap[cn]; ok {
			// The container name only proves the container exists. Its
			// state is what the caller asked for, and it is already
			// decoded in the batch inspect above.
			if s := strings.ToLower(inspect.State.Status); s != "" {
				info.Status = s
			}
			info.Image = inspect.Config.Image
			if inspect.Image != "" {
				parts := strings.Split(inspect.Image, ":")
				if len(parts) > 1 {
					digest := parts[1]
					if len(digest) > 12 {
						info.Version = digest[:12]
					} else {
						info.Version = digest
					}
				}
			}
		}

		// Git version override for git-based services
		var svc *models.Service
		if s, ok := services[name]; ok {
			svc = &s
		}
		if svc != nil && svc.Source.Kind == "git" && ws != nil {
			repoPath := workspace.GetServicePath(ws, name, *svc)
			gitCtx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
			defer cancel()
			if commitSHA, err := git.GetCommitSHA(gitCtx, repoPath); err == nil {
				info.Version = commitSHA
			}
		}

		result[name] = info
	}

	return result, nil
}
