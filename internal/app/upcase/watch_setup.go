package upcase

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
	"raioz/internal/state"
	"raioz/internal/watch"
)

// orchestrationResult holds the outputs of processOrchestration.
type orchestrationResult struct {
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
	deps *models.Deps,
	dispatcher *orchestrate.Dispatcher,
	detections DetectionMap,
	networkName string,
	projectDir string,
	envFor func(serviceName string) map[string]string,
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

	onRestart := buildRestartCallback(ctx, deps, dispatcher, detections, networkName, projectDir, envFor)

	w, err := watch.New(watch.Config{
		ServicePaths: servicePaths,
		NativeWatch:  nativeWatch,
		OnRestart:    onRestart,
	})
	if err != nil {
		logging.Warn("Failed to start file watcher", "error", err)
		output.PrintWarning(i18n.T("warning.watcher_failed", err.Error()))
		return
	}

	output.PrintProgressDone(i18n.T("watch.watching", watchCount, names))
	output.PrintInfo(i18n.T("output.press_ctrlc"))
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
	output.PrintInfo(i18n.T("output.stopping"))

	// Tear down services BEFORE cancelling the watch context. Two reasons:
	//   1. We call dispatcher.Stop with the parent `ctx` (still alive) so
	//      ImageRunner can run `docker compose down` cleanly — using a
	//      cancelled ctx would abort the compose invocation mid-way.
	//   2. If we cancelled first, exec.CommandContext would SIGKILL only the
	//      `sh -c` wrapper (not the process group), orphaning grandchildren.
	//      HostRunner.Stop does the right thing — group SIGTERM with polling
	//      and SIGKILL fallback — so we let it drive the teardown instead.
	stopAllServicesForShutdown(ctx, deps, dispatcher, detections, networkName)

	cancel()
	w.Close()
}

// stopAllServicesForShutdown tears down every service and dependency that was
// started by processOrchestration. Called from the watch-mode Ctrl+C handler
// so the user gets the same clean teardown they'd get from `raioz down`,
// without having to run a second command.
//
// Order: services first, then infra. This mirrors the dependency order a
// service would expect — stop apps before the databases they depend on.
// Errors are logged but not propagated: shutdown is best-effort and we want
// to try every service even if one fails.
func stopAllServicesForShutdown(
	ctx context.Context,
	deps *models.Deps,
	dispatcher *orchestrate.Dispatcher,
	detections DetectionMap,
	networkName string,
) {
	for name, svc := range deps.Services {
		det, ok := detections[name]
		if !ok {
			continue
		}
		svcCtx := buildServiceContext(
			name, det, networkName,
			nil,
			servicePorts(svc),
			svc.GetDependsOn(),
			naming.Container(deps.Project.Name, name),
			svc.Source.Path,
			deps.Project.Name,
		)
		if svc.Commands != nil && svc.Commands.Down != "" {
			svcCtx.StopCommand = svc.Commands.Down
		}
		if svc.ProxyOverride != nil {
			svcCtx.ProxyTarget = svc.ProxyOverride.Target
		}
		if err := dispatcher.Stop(ctx, svcCtx); err != nil {
			logging.WarnWithContext(ctx, "Failed to stop service on shutdown",
				"service", name, "error", err.Error())
		}
	}

	for name, entry := range deps.Infra {
		det, ok := detections[name]
		if !ok {
			continue
		}
		svcCtx := buildServiceContext(
			name, det, networkName,
			nil,
			infraPorts(entry),
			nil,
			naming.Container(deps.Project.Name, name),
			"",
			deps.Project.Name,
		)
		if err := dispatcher.Stop(ctx, svcCtx); err != nil {
			logging.WarnWithContext(ctx, "Failed to stop dependency on shutdown",
				"dependency", name, "error", err.Error())
		}
	}
}

// buildRestartCallback creates the function called when a file change triggers a restart.
func buildRestartCallback(
	ctx context.Context,
	deps *models.Deps,
	dispatcher *orchestrate.Dispatcher,
	detections DetectionMap,
	networkName string,
	projectDir string,
	envFor func(serviceName string) map[string]string,
) watch.RestartFunc {
	return func(serviceName string) {
		logging.Info("File change detected, restarting", "service", serviceName)
		output.PrintInfo(i18n.T("watch.restarting", serviceName))

		det, ok := detections[serviceName]
		if !ok {
			return
		}

		svc := deps.Services[serviceName]
		containerName := naming.Container(deps.Project.Name, serviceName)

		// Preserve the PORT the allocator picked on the initial start so
		// the host process rebinds the same port on every reload. Without
		// this, a service that moved from 3000→3001 would regress to 3000
		// on the next file-change restart and clash with whoever took 3000.
		//
		// The rest of the env is recomputed: a reload that dropped the
		// discovery vars would bring the service back unable to reach
		// its dependencies.
		restartEnv := map[string]string{}
		if envFor != nil {
			for k, v := range envFor(serviceName) {
				restartEnv[k] = v
			}
		}
		if !det.IsDocker() && det.Port > 0 {
			restartEnv["PORT"] = strconv.Itoa(det.Port)
		}

		svcCtx := buildServiceContext(
			serviceName, det, networkName,
			restartEnv,
			servicePorts(svc),
			svc.GetDependsOn(),
			containerName,
			svc.Source.Path,
			deps.Project.Name,
		)
		if svc.Commands != nil && svc.Commands.Down != "" {
			svcCtx.StopCommand = svc.Commands.Down
		}
		if svc.ProxyOverride != nil {
			svcCtx.ProxyTarget = svc.ProxyOverride.Target
		}
		applyServiceEnv(&svcCtx, svc.Env, projectDir)

		if err := dispatcher.Restart(ctx, svcCtx); err != nil {
			output.PrintWarning(i18n.T("watch.failed", serviceName, err))
			return
		}

		// Update PID in state for host processes
		if det.Runtime != models.RuntimeCompose &&
			det.Runtime != models.RuntimeDockerfile &&
			det.Runtime != models.RuntimeImage {
			pid := dispatcher.GetHostPID(serviceName)
			if pid > 0 {
				updateHostPID(projectDir, serviceName, pid)
			}
		}

		output.PrintSuccess(i18n.T("watch.restarted", serviceName))
	}
}

// updateHostPID updates a single service PID in the local state.
func updateHostPID(projectDir, serviceName string, pid int) {
	localState, _ := state.LoadLocalState(projectDir)
	if localState == nil {
		return
	}
	localState.HostPIDs[serviceName] = pid
	// Best-effort: watch PID updates are advisory for `status` — losing
	// one doesn't break the watched process itself.
	_ = state.SaveLocalState(projectDir, localState)
}

// ServiceStartContext rebuilds, from the config alone, the context `up`
// starts a service with: detection, recomputed env, ports and stop command.
// ok is false when the name is not a service of the project.
func ServiceStartContext(
	ctx context.Context, dm interfaces.DiscoveryManager, deps *models.Deps, projectDir, name string,
) (svcCtx interfaces.ServiceContext, ok bool) {
	svc, isService := deps.Services[name]
	det, detected := BuildDetectionMap(deps)[name]
	if !isService || !detected {
		return interfaces.ServiceContext{}, false
	}
	svcCtx = buildServiceContext(
		name, det, deps.Network.GetName(),
		ComputedServiceEnv(ctx, dm, docker.NewLookup(), deps, projectDir, name),
		servicePorts(svc),
		svc.GetDependsOn(),
		naming.Container(deps.Project.Name, name),
		svc.Source.Path,
		deps.Project.Name,
	)
	if svc.Commands != nil && svc.Commands.Down != "" {
		svcCtx.StopCommand = svc.Commands.Down
	}
	if svc.ProxyOverride != nil {
		svcCtx.ProxyTarget = svc.ProxyOverride.Target
	}
	applyServiceEnv(&svcCtx, svc.Env, projectDir)
	return svcCtx, true
}
