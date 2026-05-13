package app

import (
	"raioz/internal/discovery"
	"raioz/internal/domain/interfaces"
	"raioz/internal/infra/config"
	"raioz/internal/infra/docker"
	"raioz/internal/infra/env"
	"raioz/internal/infra/git"
	"raioz/internal/infra/host"
	"raioz/internal/infra/lock"
	infrasnapshot "raioz/internal/infra/snapshot"
	"raioz/internal/infra/state"
	infratunnel "raioz/internal/infra/tunnel"
	"raioz/internal/infra/validate"
	"raioz/internal/infra/workspace"
	"raioz/internal/proxy"
)

// Dependencies holds all dependencies for use cases
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

// NewDependencies creates a new Dependencies instance with default implementations
func NewDependencies() *Dependencies {
	return &Dependencies{
		ConfigLoader:     config.NewConfigLoader(),
		Validator:        validate.NewValidator(),
		DockerRunner:     docker.NewDockerRunner(),
		GitRepository:    git.NewGitRepository(),
		Workspace:        workspace.NewWorkspaceManager(),
		StateManager:     state.NewStateManager(),
		LockManager:      lock.NewLockManager(),
		HostRunner:       host.NewHostRunner(),
		EnvManager:       env.NewEnvManager(),
		ProxyManager:     proxy.NewManager(proxy.CertsDir()),
		DiscoveryManager: discovery.NewManager(),
		SnapshotManager:  infrasnapshot.NewManager(""),
		TunnelManager:    infratunnel.NewManager(),
	}
}

// NewDependenciesWithMocks creates a new Dependencies instance with injected mocks (for testing)
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
) *Dependencies {
	return &Dependencies{
		ConfigLoader:  configLoader,
		Validator:     validator,
		DockerRunner:  dockerRunner,
		GitRepository: gitRepo,
		Workspace:     workspace,
		StateManager:  stateManager,
		LockManager:   lockManager,
		HostRunner:    hostRunner,
		EnvManager:    envManager,
	}
}
