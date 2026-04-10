package upcase

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/output"
)

// streamForeground streams logs from all services until Ctrl+C (no file watching).
func streamForeground(ctx context.Context, deps *config.Deps, detections DetectionMap) {
	output.PrintInfo("Streaming logs (Ctrl+C to stop)")
	fmt.Println()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	streamCtx, cancel := context.WithCancel(ctx)
	go streamAllLogs(streamCtx, deps, detections)

	<-sigCh
	fmt.Println()
	output.PrintInfo("Stopping...")
	cancel()
}

// ANSI colors for service log prefixes
var serviceColors = []string{
	"\033[36m", // cyan
	"\033[33m", // yellow
	"\033[35m", // magenta
	"\033[32m", // green
	"\033[34m", // blue
	"\033[91m", // bright red
	"\033[92m", // bright green
	"\033[93m", // bright yellow
	"\033[94m", // bright blue
	"\033[95m", // bright magenta
}

const colorReset = "\033[0m"

// streamAllLogs tails log files for host services and docker logs for containers.
// Multiplexes output with colored service name prefixes. Blocks until ctx is cancelled.
func streamAllLogs(
	ctx context.Context,
	deps *config.Deps,
	detections DetectionMap,
) {
	var wg sync.WaitGroup
	colorIdx := 0

	// Calculate max service name length for alignment
	maxLen := 0
	for name := range deps.Services {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	for name := range deps.Infra {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	// Stream host service logs
	for name := range deps.Services {
		det, ok := detections[name]
		if !ok {
			continue
		}
		if det.Runtime == detect.RuntimeCompose ||
			det.Runtime == detect.RuntimeDockerfile ||
			det.Runtime == detect.RuntimeImage {
			continue
		}

		color := serviceColors[colorIdx%len(serviceColors)]
		colorIdx++
		logPath := filepath.Join(os.TempDir(), "raioz-orchestrate", "logs", name+".log")

		wg.Add(1)
		go func(svcName, logFile, c string) {
			defer wg.Done()
			tailFile(ctx, svcName, logFile, c, maxLen)
		}(name, logPath, color)
	}

	// Stream docker container logs for dependencies
	for name := range deps.Infra {
		containerName := fmt.Sprintf("raioz-%s-%s", deps.Project.Name, name)
		color := serviceColors[colorIdx%len(serviceColors)]
		colorIdx++

		wg.Add(1)
		go func(svcName, container, c string) {
			defer wg.Done()
			tailDocker(ctx, svcName, container, c, maxLen)
		}(name, containerName, color)
	}

	wg.Wait()
}

// tailFile tails a log file with colored prefix, starting from current position.
func tailFile(ctx context.Context, name, logPath, color string, padLen int) {
	// Wait for file to exist
	for {
		if _, err := os.Stat(logPath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}

	cmd := exec.CommandContext(ctx, "tail", "-n", "0", "-f", logPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}

	prefixedCopy(ctx, stdout, name, color, padLen)
	cmd.Wait()
}

// tailDocker streams docker logs with colored prefix.
func tailDocker(ctx context.Context, name, container, color string, padLen int) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--since", "1s", container)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = cmd.Stdout // Merge stderr into stdout
	if err := cmd.Start(); err != nil {
		return
	}

	prefixedCopy(ctx, stdout, name, color, padLen)
	cmd.Wait()
}

// prefixedCopy reads lines from r and prints them with a colored service prefix.
func prefixedCopy(ctx context.Context, r io.Reader, name, color string, padLen int) {
	scanner := bufio.NewScanner(r)
	prefix := fmt.Sprintf("  %s%-*s%s │ ", color, padLen, name, colorReset)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		fmt.Printf("%s%s\n", prefix, scanner.Text())
	}
}
