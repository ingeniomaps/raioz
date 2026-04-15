package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/git"
	"raioz/internal/runtime"
	"raioz/internal/workspace"
)

// ServiceInfo contains detailed information about a service
type ServiceInfo struct {
	Name        string
	Status      string // running, stopped
	Health      string // healthy, unhealthy, starting, none
	Uptime      string // time since start
	Memory      string // memory usage
	CPU         string // CPU usage
	Image       string // image name and tag
	Version     string // commit SHA or image digest
	LastUpdated string // last update time
	Linked      bool   // true if service is linked to external path
	LinkTarget  string // external path if linked (empty if not linked)
}

// ContainerInspect contains docker inspect output structure
type ContainerInspect struct {
	State struct {
		Status string `json:"Status"`
		Health *struct {
			Status string `json:"Status"`
		} `json:"Health"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	Config struct {
		Image string   `json:"Image"`
		Env   []string `json:"Env"`
	} `json:"Config"`
	Image string `json:"Image"` // image digest
}

// GetContainerName returns the container name for a service
func GetContainerName(composePath string, serviceName string) (string, error) {
	return GetContainerNameWithContext(context.Background(), composePath, serviceName)
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
		"docker", "compose", "-f", composePath,
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

// GetServiceInfo retrieves detailed information about a service
func GetServiceInfo(
	composePath string,
	serviceName string,
	projectName string,
	svc *config.Service,
	ws *workspace.Workspace,
) (*ServiceInfo, error) {
	return GetServiceInfoWithContext(context.Background(), composePath, serviceName, projectName, svc, ws)
}

// GetServiceInfoWithContext retrieves detailed information about a service with context support
func GetServiceInfoWithContext(
	ctx context.Context,
	composePath string,
	serviceName string,
	projectName string,
	svc *config.Service,
	ws *workspace.Workspace,
) (*ServiceInfo, error) {
	info := &ServiceInfo{
		Name:   serviceName,
		Status: "stopped",
		Health: "none",
	}

	// Get container name
	containerName, err := GetContainerNameWithContext(ctx, composePath, serviceName)
	if err != nil || containerName == "" {
		// Service not running
		return info, nil
	}

	info.Status = "running"

	// Create context with timeout for inspect
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	// Get container inspect data
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "inspect", containerName)
	output, err := cmd.Output()
	if err != nil {
		// If inspect fails, return basic info
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			// Log timeout but return basic info
			return info, nil
		}
		return info, nil
	}

	var inspectData []ContainerInspect
	if err := json.Unmarshal(output, &inspectData); err != nil || len(inspectData) == 0 {
		return info, nil
	}

	inspect := inspectData[0]

	// Parse health status
	if inspect.State.Health != nil {
		info.Health = strings.ToLower(inspect.State.Health.Status)
	} else {
		info.Health = "none"
	}

	// Parse uptime
	if inspect.State.StartedAt != "" {
		startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if err == nil {
			uptime := time.Since(startedAt)
			info.Uptime = formatUptime(uptime)
		}
	}

	// Get image info
	info.Image = inspect.Config.Image

	// Get image digest/version (first 12 chars)
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

	// Get resource usage
	cpu, memory, err := getResourceUsageWithContext(ctx, containerName)
	if err == nil {
		info.CPU = cpu
		info.Memory = memory
	}

	// Get version from git if it's a git-based service
	if svc != nil && svc.Source.Kind == "git" && ws != nil {
		repoPath := workspace.GetServicePath(ws, serviceName, *svc)

		// Create context with timeout for git operations
		ctx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
		defer cancel()

		if commitSHA, err := git.GetCommitSHA(ctx, repoPath); err == nil {
			info.Version = commitSHA
			// Get commit date for last updated
			if commitDate, err := git.GetCommitDate(ctx, repoPath); err == nil {
				info.LastUpdated = commitDate
			}
		}
	}

	// Get last updated time from container start if not set from git
	if info.LastUpdated == "" && inspect.State.StartedAt != "" {
		startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
		if err == nil {
			info.LastUpdated = startedAt.Format("2006-01-02 15:04:05")
		}
	}

	return info, nil
}

// GetServicesInfo retrieves detailed information for all services
func GetServicesInfo(
	composePath string,
	serviceNames []string,
	projectName string,
	services map[string]config.Service,
	ws *workspace.Workspace,
) (map[string]*ServiceInfo, error) {
	return GetServicesInfoWithContext(context.Background(), composePath, serviceNames, projectName, services, ws)
}

// GetServicesInfoWithContext retrieves detailed information for all services with context support.
// It batches docker inspect and docker stats calls to avoid N+1 command overhead.
func GetServicesInfoWithContext(
	ctx context.Context,
	composePath string,
	serviceNames []string,
	projectName string,
	services map[string]config.Service,
	ws *workspace.Workspace,
) (map[string]*ServiceInfo, error) {
	result := make(map[string]*ServiceInfo, len(serviceNames))

	// Step 1: resolve container names for all services
	containerMap := make(map[string]string) // serviceName → containerName
	for _, name := range serviceNames {
		cn, err := GetContainerNameWithContext(ctx, composePath, name)
		if err != nil || cn == "" {
			result[name] = &ServiceInfo{Name: name, Status: "stopped", Health: "none"}
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

	// Step 3: single batch docker stats for all running containers
	statsMap := batchResourceUsage(ctx, containerNames)

	// Step 4: assemble ServiceInfo for each service
	for _, name := range serviceNames {
		if _, ok := containerMap[name]; !ok {
			continue // already set to stopped above
		}
		cn := containerMap[name]
		info := &ServiceInfo{Name: name, Status: "running", Health: "none"}

		if inspect, ok := inspectMap[cn]; ok {
			if inspect.State.Health != nil {
				info.Health = strings.ToLower(inspect.State.Health.Status)
			}
			if inspect.State.StartedAt != "" {
				startedAt, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt)
				if err == nil {
					info.Uptime = formatUptime(time.Since(startedAt))
					info.LastUpdated = startedAt.Format("2006-01-02 15:04:05")
				}
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

		if stats, ok := statsMap[cn]; ok {
			info.CPU = stats.cpu
			info.Memory = stats.memory
		}

		// Git version override for git-based services
		var svc *config.Service
		if s, ok := services[name]; ok {
			svc = &s
		}
		if svc != nil && svc.Source.Kind == "git" && ws != nil {
			repoPath := workspace.GetServicePath(ws, name, *svc)
			gitCtx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
			defer cancel()
			if commitSHA, err := git.GetCommitSHA(gitCtx, repoPath); err == nil {
				info.Version = commitSHA
				if commitDate, err := git.GetCommitDate(gitCtx, repoPath); err == nil {
					info.LastUpdated = commitDate
				}
			}
		}

		result[name] = info
	}

	return result, nil
}
