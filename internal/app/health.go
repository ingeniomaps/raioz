package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
)

// HealthOptions holds options for the health use case
type HealthOptions struct {
	ConfigPath string
}

// HealthUseCase answers one question — is this project healthy — and
// answers it with an exit code so a script can act on it.
//
// It used to run the project's `commands.health`, a field only
// `.raioz.json` could declare. With that format hard-erroring since
// ADR-038 the command could only ever report "not healthy", including
// over a stack that was entirely up. What it reports now comes from the
// same probes `raioz status` reads, plus the `health:` endpoint a service
// declares — which is the user's own definition of healthy and therefore
// outranks the rest.
type HealthUseCase struct {
	deps *Dependencies
}

// NewHealthUseCase creates a new HealthUseCase
func NewHealthUseCase(deps *Dependencies) *HealthUseCase {
	return &HealthUseCase{deps: deps}
}

// healthVerdict is one line of the report.
type healthVerdict struct {
	name    string
	kind    string // "service" or "dependency"
	status  string
	healthy bool
	detail  string // endpoint probed, restart count — whatever earned the verdict
}

// healthEndpointProbe is the HTTP probe the use case consults. Package var
// so tests answer without binding a socket.
var healthEndpointProbe = host.ProbeHTTP

// Execute runs the health use case.
func (uc *HealthUseCase) Execute(ctx context.Context, opts HealthOptions) error {
	ctx = logging.WithRequestID(ctx)
	ctx = logging.WithOperation(ctx, "raioz health")

	proj := ResolveYAMLProject(uc.deps, opts.ConfigPath)
	if proj == nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.no_project"),
		).WithSuggestion(i18n.T("error.no_project_suggestion"))
	}

	return uc.reportHealth(ctx, proj)
}

// reportHealth prints one verdict per declared item and returns an error
// when any of them is not healthy, so `raioz health` exits non-zero and a
// script can branch on it.
func (uc *HealthUseCase) reportHealth(ctx context.Context, proj *YAMLProject) error {
	verdicts := uc.collectHealth(ctx, proj)
	if len(verdicts) == 0 {
		output.PrintInfo(i18n.T("health.nothing_declared"))
		return nil
	}

	unhealthy := 0
	for _, v := range verdicts {
		line := fmt.Sprintf("    %-18s %-12s %s", v.name, v.kind, v.status)
		if v.detail != "" {
			line += "  " + v.detail
		}
		if v.healthy {
			output.PrintSuccess(line)
			continue
		}
		unhealthy++
		output.PrintWarning(line)
	}

	if unhealthy == 0 {
		output.PrintSuccess(i18n.T("health.all_healthy", len(verdicts)))
		return nil
	}
	return errors.New(
		errors.ErrCodeUnhealthy,
		i18n.T("health.some_unhealthy", unhealthy, len(verdicts)),
	).WithSuggestion(i18n.T("health.some_unhealthy_suggestion"))
}

// collectHealth builds one verdict per declared dependency and service,
// dependencies first, each group in name order so two runs are comparable.
func (uc *HealthUseCase) collectHealth(ctx context.Context, proj *YAMLProject) []healthVerdict {
	var out []healthVerdict

	for _, name := range sortedKeysInfra(proj.Deps.Infra) {
		st := proj.ContainerState(ctx, name)
		v := healthVerdict{
			name:    name,
			kind:    "dependency",
			status:  st.Status,
			healthy: st.Status == statusRunning,
		}
		if st.Restarts > 0 {
			v.detail = fmt.Sprintf("restarts:%d", st.Restarts)
			// A container that keeps dying is not healthy however the
			// sample happened to land — see ContainerState.
			v.healthy = false
		}
		out = append(out, v)
	}

	projectDir, _ := filepath.Abs(filepath.Dir(proj.ConfigPath))
	localState, _ := state.LoadLocalState(projectDir)
	for _, name := range sortedKeysServices(proj.Deps.Services) {
		out = append(out, uc.serviceVerdict(ctx, name, proj.Deps.Services[name], localState))
	}
	return out
}

// serviceVerdict resolves one service, strongest signal first.
func (uc *HealthUseCase) serviceVerdict(
	ctx context.Context, name string, svc models.Service, localState *models.LocalState,
) healthVerdict {
	v := healthVerdict{name: name, kind: "service", status: statusStopped}

	// The declared endpoint wins: the user wrote it to say what healthy
	// means for this service, and no inference beats that.
	if svc.HealthEndpoint != "" && svc.Port > 0 {
		url := host.HealthURL(svc.Port, svc.HealthEndpoint)
		v.detail = url
		if healthEndpointProbe(ctx, url) {
			v.status, v.healthy = "healthy", true
		} else {
			v.status = "unhealthy"
		}
		return v
	}

	if svc.ProxyOverride != nil && svc.ProxyOverride.Target != "" {
		if st, ok := dockerStateProbe(ctx, svc.ProxyOverride.Target); ok {
			v.status = st.Status
			v.healthy = st.Status == statusRunning && st.Restarts == 0
			if st.Restarts > 0 {
				v.detail = fmt.Sprintf("restarts:%d", st.Restarts)
			}
			return v
		}
	}

	if localState != nil {
		if pid, ok := localState.HostPIDs[name]; ok && pid > 0 && isHostProcessAlive(pid) {
			v.status = hostServiceStatus(ctx, svc.Port)
			v.healthy = v.status == statusRunning
			v.detail = fmt.Sprintf("pid:%d", pid)
		}
	}
	if svc.HealthEndpoint != "" && svc.Port <= 0 {
		v.detail = i18n.T("health.endpoint_needs_port")
	}
	return v
}

func sortedKeysInfra(m map[string]models.InfraEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysServices(m map[string]models.Service) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
