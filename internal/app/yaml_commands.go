package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/fsutil"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/runtime"
)

// StatusYAML shows status for a YAML orchestrated project. When `filter` is
// non-empty, only services / dependencies in that list are reported and any
// unknown name returns an error so the user notices the typo.
func (uc *StatusUseCase) StatusYAML(ctx context.Context, proj *YAMLProject, filter []string) error {
	report, err := uc.collectStatus(ctx, proj, filter)
	if err != nil {
		return err
	}
	printStatusReport(report)
	return nil
}

// RestartYAML restarts services in a YAML orchestrated project. Host
// services (declared with `command:`) go through HostRunner so the
// settle-window + launcher-pattern logic applies; docker services
// delegate to `docker restart <container>`. ADR-025.
func (uc *RestartUseCase) RestartYAML(
	ctx context.Context, proj *YAMLProject, opts RestartOptions,
) (err error) {
	services := opts.Services
	if len(services) == 0 {
		if !opts.All {
			output.PrintWarning(
				"No services specified. Use service names or --all")
			return nil
		}
		services = collectYAMLServiceNames(proj)
		if opts.IncludeInfra {
			services = append(services, collectYAMLDepNames(proj)...)
		}
		if len(services) == 0 {
			output.PrintWarning(i18n.T("warning.no_services_to_restart"))
			return nil
		}
	}

	// Lifecycle audit. Restart can be partial-success at the
	// per-service level (printed); the lifecycle pair records the
	// outer Execute outcome only.
	startTime := time.Now()
	if auditErr := audit.LogLifecycleStart(
		ctx, "restart", proj.ProjectName, proj.Deps.Workspace,
	); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogLifecycleStart failed",
			"error", auditErr.Error())
	}
	defer func() {
		status := "success"
		if err != nil {
			status = "failure"
		}
		if auditErr := audit.LogLifecycleComplete(
			ctx, "restart", proj.ProjectName, proj.Deps.Workspace,
			status, time.Since(startTime), err,
		); auditErr != nil {
			logging.DebugWithContext(ctx, "audit LogLifecycleComplete failed",
				"error", auditErr.Error())
		}
	}()

	// Track per-service failures so the command reflects them in its exit
	// code: restart used to return nil even when a relaunch died in the
	// settle window, reporting success over a stopped service.
	var failed []string
	for _, name := range services {
		if isYAMLHostService(proj, name) {
			if restartErr := uc.restartHostService(ctx, proj, name); restartErr != nil {
				output.PrintProgressError(name + ": " + restartErr.Error())
				failed = append(failed, name)
			} else {
				output.PrintProgressDone(name)
			}
			continue
		}

		// Resolve by label: the canonical name misses workspace deps and
		// any compose entry that sets its own `container_name:`. Nothing
		// found falls back to it, so docker reports what is missing.
		containers := proj.liveContainerNames(ctx, name)
		if len(containers) == 0 {
			containers = []string{naming.Container(proj.ProjectName, name)}
		}
		output.PrintProgress(i18n.T("output.restarting_service", name))
		cmd := exec.CommandContext(ctx, runtime.Binary(), append([]string{"restart"}, containers...)...)
		if out, restartErr := cmd.CombinedOutput(); restartErr != nil {
			output.PrintProgressError(name + ": " + strings.TrimSpace(string(out)))
			failed = append(failed, name)
		} else {
			output.PrintProgressDone(name)
		}
	}
	if len(failed) > 0 {
		err = errors.New(
			errors.ErrCodeServiceStartFailed,
			i18n.T("error.restart_failed"),
		).WithSuggestion(i18n.T("error.restart_suggestion")).
			WithContext("services", strings.Join(failed, ", "))
	}
	return err
}

// collectYAMLServiceNames returns the service names of a YAML project in a
// deterministic order. Sorted so `restart --all` output (and tests) don't
// depend on Go's randomized map iteration.
func collectYAMLServiceNames(proj *YAMLProject) []string {
	if proj == nil || proj.Deps == nil {
		return nil
	}
	names := make([]string, 0, len(proj.Deps.Services))
	for name := range proj.Deps.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// collectYAMLDepNames mirrors collectYAMLServiceNames for the infra map,
// used when --include-infra opts dependencies back into restart --all.
func collectYAMLDepNames(proj *YAMLProject) []string {
	if proj == nil || proj.Deps == nil {
		return nil
	}
	names := make([]string, 0, len(proj.Deps.Infra))
	for name := range proj.Deps.Infra {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// isYAMLHostService reports whether the named entry in a YAML project runs
// as a host process. Used to pick the right restart path. Returns false for
// unknown names so the docker fallback can produce its own (admittedly
// ugly) error.
//
// A declared `command:` / `commands:` is not the only signal: a host
// runtime auto-detected from the directory (`runtime: npm`, a bare
// package.json) declares no command, yet `up` launched it on the host.
// Classify it with the same ResolveServiceDetection that up and status
// use, so the three commands agree on where the service runs.
func isYAMLHostService(proj *YAMLProject, name string) bool {
	svc, ok := proj.Deps.Services[name]
	if !ok {
		return false
	}
	if svc.Docker != nil {
		return false
	}
	if svc.Source.Command != "" || svc.Commands != nil {
		return true
	}
	det := config.ResolveServiceDetection(svc, svc.Source.Path)
	return det.IsHost()
}

// ExecYAML runs a command in a container of a YAML orchestrated project.
func ExecYAML(ctx context.Context, proj *YAMLProject, serviceName string, command []string, interactive bool) error {
	// Check if it's a Docker container or host service
	containerName := proj.liveContainerName(ctx, serviceName)
	if containerName == "" {
		// Might be a host service — exec in the directory
		if svc, ok := proj.Deps.Services[serviceName]; ok && svc.Source.Path != "" {
			output.PrintInfo(i18n.T("output.exec_in_dir", svc.Source.Path))
			args := command
			if len(args) == 0 {
				args = []string{"sh"}
			}
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Dir = svc.Source.Path
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if fsutil.IsTerminal(os.Stdin) {
				cmd.Stdin = os.Stdin
			}
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("exec in service dir: %w", err)
			}
			return nil
		}
		return fmt.Errorf("service '%s' is not running", serviceName)
	}

	isTTY := fsutil.IsTerminal(os.Stdin)

	args := []string{"exec"}
	if interactive && isTTY {
		args = append(args, "-it")
	}
	args = append(args, containerName)
	if len(command) == 0 {
		args = append(args, "sh")
	} else {
		args = append(args, command...)
	}

	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	if isTTY {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker exec: %w", err)
	}
	return nil
}

// isHostProcessAlive checks if a process with the given PID is running.
func isHostProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// CheckYAML validates a YAML project config.
