package app

import "raioz/internal/domain/interfaces"

// Dependencies holds every port a use case can consume. The struct
// lives here in `internal/app/` so use cases reference it via a
// single import; production wiring (constructing adapters) lives in
// `internal/cli/wiring.go` (ADR-018). This file intentionally
// imports nothing under `internal/` except `domain/interfaces` —
// importing a use-case package no longer drags adapters with it.
type Dependencies struct {
	ConfigLoader     interfaces.ConfigLoader
	Validator        interfaces.Validator
	DockerRunner     interfaces.DockerRunner
	GitRepository    interfaces.GitRepository
	Workspace        interfaces.WorkspaceManager
	StateManager     interfaces.StateManager
	LockManager      interfaces.LockManager
	HostRunner       interfaces.HostRunner
	EnvManager       interfaces.EnvManager
	ProxyManager     interfaces.ProxyManager     // Optional: nil if proxy not needed
	DiscoveryManager interfaces.DiscoveryManager // Optional: nil if discovery not needed
	SnapshotManager  interfaces.SnapshotManager  // Volume backup/restore (ADR-014)
	TunnelManager    interfaces.TunnelManager    // Cloudflared/bore tunnels (ADR-015)
}

// NewDependenciesWithMocks builds a Dependencies from explicit port
// values — the test-mode constructor. Every port is accepted as the
// domain interface, so callers can pass mocks, real adapters, or a
// mix per call.
//
// Pre-ADR-018 this took only nine positional arguments because
// ProxyManager / DiscoveryManager / SnapshotManager / TunnelManager
// were constructed inline by NewDependencies and never reached the
// mock path. Today every port is mockable.
func NewDependenciesWithMocks(
	configLoader interfaces.ConfigLoader,
	validator interfaces.Validator,
	dockerRunner interfaces.DockerRunner,
	gitRepo interfaces.GitRepository,
	workspace interfaces.WorkspaceManager,
	stateManager interfaces.StateManager,
	lockManager interfaces.LockManager,
	hostRunner interfaces.HostRunner,
	envManager interfaces.EnvManager,
	proxyManager interfaces.ProxyManager,
	discoveryManager interfaces.DiscoveryManager,
	snapshotManager interfaces.SnapshotManager,
	tunnelManager interfaces.TunnelManager,
) *Dependencies {
	return &Dependencies{
		ConfigLoader:     configLoader,
		Validator:        validator,
		DockerRunner:     dockerRunner,
		GitRepository:    gitRepo,
		Workspace:        workspace,
		StateManager:     stateManager,
		LockManager:      lockManager,
		HostRunner:       hostRunner,
		EnvManager:       envManager,
		ProxyManager:     proxyManager,
		DiscoveryManager: discoveryManager,
		SnapshotManager:  snapshotManager,
		TunnelManager:    tunnelManager,
	}
}
