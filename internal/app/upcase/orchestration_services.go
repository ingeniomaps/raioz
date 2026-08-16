// Step 3 of processOrchestration: bring up the project's own services in
// dependency order. Split out of orchestration.go, which keeps the phase
// sequencing (detect → infra → services → proxy).

package upcase

import (
	"context"
	"strconv"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
)

// serviceDispatcher is the slice of orchestrate.Dispatcher the up flow
// actually uses: start a service, and report the host PID it produced.
// Depending on the narrow port instead of the concrete type keeps this file
// (and host_lifecycle.go) off the app→infra import list — ADR-012/ADR-029 —
// and lets tests hand in a stub without an orchestrator.
type serviceDispatcher interface {
	Start(ctx context.Context, svc interfaces.ServiceContext) error
	GetHostPID(serviceName string) int
}

// startServicesParams carries the orchestration state Step 3 reads. A struct
// rather than a dozen positional args: every field is already computed by the
// earlier steps and passing them by name keeps the call site readable.
type startServicesParams struct {
	deps         *models.Deps
	detections   DetectionMap
	serviceNames []string
	endpoints    map[string]interfaces.ServiceEndpoint
	portAllocs   *PortAllocResult
	dispatcher   serviceDispatcher
	networkName  string
	projectDir   string
	deferredDeps []string
}

// startServices starts every service in order, stopping at the first failure.
//
// Services that already started keep running — tearing down a stack of warm
// services because the last one failed would throw away expensive work — so
// the failure path persists their PIDs first. Without that, `down` and
// `status` lose track of live processes and the next `up` sees their ports as
// held by a stranger.
func (uc *UseCase) startServices(ctx context.Context, p startServicesParams) error {
	if len(p.serviceNames) == 0 {
		return nil
	}

	output.PrintProgress(i18n.T("up.starting_services", len(p.serviceNames)))
	svcStart := time.Now()
	started := make([]string, 0, len(p.serviceNames))

	for _, name := range p.serviceNames {
		svc := p.deps.Services[name]
		detection := p.detections[name]

		svcCtx := uc.buildStartContext(name, svc, detection, p)

		if err := p.dispatcher.Start(ctx, svcCtx); err != nil {
			savePartialHostPIDs(p.projectDir, p.deps.Project.Name, p.deps.Workspace,
				p.networkName, p.dispatcher, started, p.detections, p.deferredDeps)
			return errors.ServiceStartFailed(name, string(detection.Runtime), err)
		}

		started = append(started, name)
		output.PrintSuccess(name + " (" + string(detection.Runtime) + ")")
	}

	logging.InfoWithContext(ctx, "Services started",
		"count", len(p.serviceNames),
		"duration_ms", time.Since(svcStart).Milliseconds())
	output.PrintProgressDone(i18n.T("up.services_started", len(p.serviceNames)))
	return nil
}

// buildStartContext assembles the ServiceContext handed to the runner:
// discovery env vars, the allocator's port, and the yaml-level overrides.
func (uc *UseCase) buildStartContext(
	name string,
	svc models.Service,
	detection models.DetectResult,
	p startServicesParams,
) interfaces.ServiceContext {
	envVars := make(map[string]string)
	if uc.deps.DiscoveryManager != nil {
		envVars = uc.deps.DiscoveryManager.GenerateEnvVars(
			name, detection.Runtime, p.endpoints, p.deps.Proxy,
		)
	}

	// Inject PORT for host services so frameworks honoring $PORT (Next.js,
	// Vite, Django, etc.) rebind to the allocator's pick. Docker services
	// get their port via published config.
	if p.portAllocs != nil {
		if alloc, ok := p.portAllocs.Services[name]; ok && alloc.IsHost() && alloc.Port > 0 {
			envVars["PORT"] = strconv.Itoa(alloc.Port)
		}
	}

	svcCtx := buildServiceContext(
		name, detection, p.networkName,
		envVars,
		servicePorts(svc),
		svc.GetDependsOn(),
		naming.Container(p.deps.Project.Name, name),
		svc.Source.Path,
		p.deps.Project.Name,
	)

	// Propagate custom stop command (from `stop:` in raioz.yaml) so the
	// runner can use it instead of SIGTERMing the PID.
	if svc.Commands != nil && svc.Commands.Down != "" {
		svcCtx.StopCommand = svc.Commands.Down
	}
	// ADR-025: needed for HostRunner's launcher-container wait.
	if svc.ProxyOverride != nil {
		svcCtx.ProxyTarget = svc.ProxyOverride.Target
	}

	// Pass the service's own `env:` (inline vars + --env-file) to the
	// runner; mirrors the deps path in orchestration.go.
	applyServiceEnv(&svcCtx, svc.Env, p.projectDir)

	return svcCtx
}
