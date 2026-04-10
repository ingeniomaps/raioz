package upcase

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/logging"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
	"raioz/internal/state"
	"raioz/internal/watch"
)

// orchestrationResult holds the outputs of processOrchestration.
type orchestrationResult struct {
	composePath  string
	serviceNames []string
	infraNames   []string
	dispatcher   *orchestrate.Dispatcher
	detections   DetectionMap
	networkName  string
}

// startWatcher sets up file watching for services with watch: true.
// Blocks until interrupted (Ctrl+C) — this turns raioz up into a foreground process.
// Returns nil if no services need watching.
func startWatcher(
	ctx context.Context,
	deps *config.Deps,
	dispatcher *orchestrate.Dispatcher,
	detections DetectionMap,
	networkName string,
	projectDir string,
) {
	servicePaths := make(map[string]string)
	nativeWatch := make(map[string]bool)

	for name, svc := range deps.Services {
		if !svc.Watch.Enabled {
			continue
		}
		path := svc.Source.Path
		if path == "" {
			continue
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		servicePaths[name] = absPath
		if svc.Watch.Mode == "native" {
			nativeWatch[name] = true
		}
	}

	// Count services that raioz actually watches (not native)
	watchCount := 0
	var names []string
	for name := range servicePaths {
		if !nativeWatch[name] {
			watchCount++
			names = append(names, name)
		}
	}
	if watchCount == 0 {
		return
	}

	onRestart := buildRestartCallback(ctx, deps, dispatcher, detections, networkName, projectDir)

	w, err := watch.New(watch.Config{
		ServicePaths: servicePaths,
		NativeWatch:  nativeWatch,
		OnRestart:    onRestart,
	})
	if err != nil {
		logging.Warn("Failed to start file watcher", "error", err)
		output.PrintWarning("File watcher failed to start: " + err.Error())
		return
	}

	output.PrintProgressDone(fmt.Sprintf("Watching %d service(s): %v", watchCount, names))
	output.PrintInfo("Press Ctrl+C to stop")
	fmt.Println()

	// Handle Ctrl+C gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	watchCtx, cancel := context.WithCancel(ctx)
	go w.Run(watchCtx)

	// Stream logs from all services (host + docker)
	go streamAllLogs(watchCtx, deps, detections)

	// Block until signal
	<-sigCh
	fmt.Println()
	output.PrintInfo("Stopping...")
	cancel()
	w.Close()
}

// buildRestartCallback creates the function called when a file change triggers a restart.
func buildRestartCallback(
	ctx context.Context,
	deps *config.Deps,
	dispatcher *orchestrate.Dispatcher,
	detections DetectionMap,
	networkName string,
	projectDir string,
) watch.RestartFunc {
	return func(serviceName string) {
		logging.Info("File change detected, restarting", "service", serviceName)
		output.PrintInfo(fmt.Sprintf("[watch] restarting %s...", serviceName))

		det, ok := detections[serviceName]
		if !ok {
			return
		}

		svc := deps.Services[serviceName]
		containerName := fmt.Sprintf("raioz-%s-%s", deps.Project.Name, serviceName)

		svcCtx := buildServiceContext(
			serviceName, det, networkName,
			nil,
			servicePorts(svc),
			svc.GetDependsOn(),
			containerName,
			svc.Source.Path,
		)

		if err := dispatcher.Restart(ctx, svcCtx); err != nil {
			output.PrintWarning(fmt.Sprintf("[watch] failed: %s: %s", serviceName, err))
			return
		}

		// Update PID in state for host processes
		if det.Runtime != detect.RuntimeCompose &&
			det.Runtime != detect.RuntimeDockerfile &&
			det.Runtime != detect.RuntimeImage {
			pid := dispatcher.GetHostPID(serviceName)
			if pid > 0 {
				updateHostPID(projectDir, serviceName, pid)
			}
		}

		output.PrintSuccess(fmt.Sprintf("[watch] %s restarted", serviceName))
	}
}

// updateHostPID updates a single service PID in the local state.
func updateHostPID(projectDir, serviceName string, pid int) {
	localState, _ := state.LoadLocalState(projectDir)
	if localState == nil {
		return
	}
	localState.HostPIDs[serviceName] = pid
	state.SaveLocalState(projectDir, localState)
}
