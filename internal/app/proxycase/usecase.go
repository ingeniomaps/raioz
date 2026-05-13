package proxycase

import (
	"context"
	"fmt"

	"raioz/internal/domain/interfaces"
)

// Dependencies is the narrow port set the proxy use cases need.
type Dependencies struct {
	ConfigLoader interfaces.ConfigLoader
	ProxyManager interfaces.ProxyManager
}

// StatusOptions controls Status. ConfigPath is optional — when
// non-empty it lets the use case scope the manager to the project's
// workspace before probing, matching the CLI behavior pre-ADR-016.
type StatusOptions struct {
	ConfigPath string
}

// StatusUseCase reports whether the proxy is running.
type StatusUseCase struct {
	Deps *Dependencies
}

func (uc *StatusUseCase) Execute(ctx context.Context, opts StatusOptions) (bool, error) {
	if uc.Deps.ProxyManager == nil {
		return false, ErrProxyNotConfigured
	}
	uc.applyProjectScope(opts.ConfigPath)
	return uc.Deps.ProxyManager.Status(ctx)
}

// StopOptions controls Stop. Same ConfigPath semantics as Status.
type StopOptions struct {
	ConfigPath string
}

// StopUseCase stops the proxy container (or the shared one when
// running in workspace mode and the last project is leaving — that
// decision lives inside the manager).
type StopUseCase struct {
	Deps *Dependencies
}

func (uc *StopUseCase) Execute(ctx context.Context, opts StopOptions) error {
	if uc.Deps.ProxyManager == nil {
		return ErrProxyNotConfigured
	}
	uc.applyProjectScope(opts.ConfigPath)
	return uc.Deps.ProxyManager.Stop(ctx)
}

// ErrProxyNotConfigured signals the caller that ProxyManager wasn't
// wired into Dependencies (typically because the project doesn't
// declare `proxy:` in raioz.yaml). Callers normally print "proxy not
// configured" and return cleanly rather than treating it as a fault.
var ErrProxyNotConfigured = fmt.Errorf("proxy is not configured")

// applyProjectScope reads raioz.yaml (when configPath resolves) and
// applies project+workspace identity to the manager via the
// Configure entry point (ADR-013). Best-effort: if the config can't
// be loaded the manager keeps its current scope. The signature is on
// the use case because both Status and Stop need the same bit.
func (uc *StatusUseCase) applyProjectScope(configPath string) {
	applyScope(uc.Deps, configPath)
}

func (uc *StopUseCase) applyProjectScope(configPath string) {
	applyScope(uc.Deps, configPath)
}

func applyScope(deps *Dependencies, configPath string) {
	if deps == nil || deps.ConfigLoader == nil || deps.ProxyManager == nil {
		return
	}
	cfg, _, err := deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || cfg == nil {
		return
	}
	scope := interfaces.ProxyConfig{
		ProjectName: cfg.Project.Name,
		Workspace:   cfg.Workspace,
	}
	deps.ProxyManager.Configure(scope)
}
