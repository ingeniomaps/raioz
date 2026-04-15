package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
)

type containerStats struct {
	cpu    string
	memory string
}

// batchInspect runs a single `docker inspect` for multiple containers and
// returns parsed data keyed by container name.
func batchInspect(ctx context.Context, names []string) map[string]ContainerInspect {
	result := make(map[string]ContainerInspect, len(names))
	if len(names) == 0 {
		return result
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	args := append([]string{"inspect"}, names...)
	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		return result
	}

	var inspectData []ContainerInspect
	if err := json.Unmarshal(out, &inspectData); err != nil {
		return result
	}

	for i, data := range inspectData {
		if i < len(names) {
			result[names[i]] = data
		}
	}

	return result
}

// batchResourceUsage runs a single `docker stats --no-stream` for multiple
// containers instead of one call per container.
func batchResourceUsage(ctx context.Context, names []string) map[string]containerStats {
	result := make(map[string]containerStats, len(names))
	if len(names) == 0 {
		return result
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerStatsTimeout)
	defer cancel()

	args := append([]string{"stats", "--no-stream", "--format",
		"{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"}, names...)
	cmd := exec.CommandContext(timeoutCtx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return result
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		cpu := strings.TrimSpace(parts[1])
		memory := strings.TrimSpace(parts[2])
		memParts := strings.Fields(memory)
		if len(memParts) >= 3 {
			memory = memParts[0] + "/" + memParts[2]
		}
		result[name] = containerStats{cpu: cpu, memory: memory}
	}

	return result
}

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

func getResourceUsage(containerName string) (string, string, error) {
	return getResourceUsageWithContext(context.Background(), containerName)
}

func getResourceUsageWithContext(ctx context.Context, containerName string) (string, string, error) {
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
		return "", "", fmt.Errorf("docker stats: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected stats format")
	}

	cpu := strings.TrimSpace(parts[0])
	memory := strings.TrimSpace(parts[1])

	memParts := strings.Fields(memory)
	if len(memParts) >= 2 {
		memory = memParts[0] + "/" + memParts[2]
	}

	return cpu, memory, nil
}
