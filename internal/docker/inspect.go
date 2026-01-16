package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/git"
	"raioz/internal/link"
	exectimeout "raioz/internal/exec"
	"raioz/internal/workspace"
)

// ServiceInfo contains detailed information about a service
type ServiceInfo struct {
	Name            string
	Status          string // running, stopped
	Health          string // healthy, unhealthy, starting, none
	Uptime          string // time since start
	Memory          string // memory usage
	CPU             string // CPU usage
	Image           string // image name and tag
	Version         string // commit SHA or image digest
	LastUpdated     string // last update time
	Linked          bool   // true if service is linked to external path
	LinkTarget      string // external path if linked (empty if not linked)
}

// ContainerInspect contains docker inspect output structure
type ContainerInspect struct {
	State struct {
		Status    string `json:"Status"`
		Health    *struct {
			Status string `json:"Status"`
		} `json:"Health"`
		StartedAt string `json:"StartedAt"`
	} `json:"State"`
	Config struct {
		Image string `json:"Image"`
	} `json:"Config"`
	Image string `json:"Image"` // image digest
}

// GetContainerName returns the container name for a service
func GetContainerName(composePath string, serviceName string) (string, error) {
	return GetContainerNameWithContext(context.Background(), composePath, serviceName)
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
	cmd2 := exec.CommandContext(timeoutCtx, "docker", "inspect", "-f", "{{.Name}}", containerID)
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
	cmd := exec.CommandContext(timeoutCtx, "docker", "inspect", containerName)
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

			// Check if service is linked
			isLinked, linkTarget, err := link.IsLinked(repoPath)
			if err == nil && isLinked {
				info.Linked = true
				info.LinkTarget = linkTarget
			}

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

// GetServicesInfoWithContext retrieves detailed information for all services with context support
func GetServicesInfoWithContext(
	ctx context.Context,
	composePath string,
	serviceNames []string,
	projectName string,
	services map[string]config.Service,
	ws *workspace.Workspace,
) (map[string]*ServiceInfo, error) {
	result := make(map[string]*ServiceInfo)

	for _, name := range serviceNames {
		var svc *config.Service
		if s, ok := services[name]; ok {
			svc = &s
		}

		info, err := GetServiceInfoWithContext(ctx, composePath, name, projectName, svc, ws)
		if err != nil {
			// Continue with other services even if one fails
			continue
		}
		result[name] = info
	}

	return result, nil
}

// formatUptime formats a duration into human-readable uptime
func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// getResourceUsage retrieves CPU and memory usage for a container
func getResourceUsage(containerName string) (string, string, error) {
	return getResourceUsageWithContext(context.Background(), containerName)
}

// getResourceUsageWithContext retrieves CPU and memory usage for a container with context support
func getResourceUsageWithContext(ctx context.Context, containerName string) (string, string, error) {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerStatsTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx,
		"docker", "stats",
		"--no-stream",
		"--format",
		"{{.CPUPerc}}\t{{.MemUsage}}",
		containerName,
	)
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return "", "", fmt.Errorf("docker stats timed out after %v", exectimeout.DockerStatsTimeout)
		}
		return "", "", err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected stats format")
	}

	cpu := strings.TrimSpace(parts[0])
	memory := strings.TrimSpace(parts[1])

	// Parse memory to show only used/total
	memParts := strings.Fields(memory)
	if len(memParts) >= 2 {
		memory = memParts[0] + "/" + memParts[2]
	}

	return cpu, memory, nil
}
