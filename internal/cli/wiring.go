// Package cli holds the Cobra command tree. wiring.go assembles the
// production app.Dependencies out of concrete adapters — the single
// place infrastructure packages get instantiated for the running
// binary. Per ADR-018, internal/app/dependencies.go owns the struct
// shape; this file owns the wiring.
package cli

import (
	"raioz/internal/app"
	"raioz/internal/infra/config"
	"raioz/internal/infra/discovery"
	"raioz/internal/infra/docker"
	"raioz/internal/infra/env"
	"raioz/internal/infra/git"
	"raioz/internal/infra/host"
	"raioz/internal/infra/lock"
	infraproxy "raioz/internal/infra/proxy"
	"raioz/internal/infra/snapshot"
	"raioz/internal/infra/state"
	"raioz/internal/infra/tunnel"
	"raioz/internal/infra/validate"
	"raioz/internal/infra/workspace"
)

// newDependencies builds the production *app.Dependencies. Every
// adapter constructor goes through `internal/infra/*` — no CLI file
// should import `internal/<concrete>` directly. The (CLI-internal)
// lower-case name signals that callers outside this package keep
// using app.NewDependencies (re-exported below) to preserve the
// existing API surface.
func newDependencies() *app.Dependencies {
	return &app.Dependencies{
		ConfigLoader:     config.NewConfigLoader(),
		Validator:        validate.NewValidator(),
		DockerRunner:     docker.NewDockerRunner(),
		GitRepository:    git.NewGitRepository(),
		Workspace:        workspace.NewWorkspaceManager(),
		StateManager:     state.NewStateManager(),
		LockManager:      lock.NewLockManager(),
		HostRunner:       host.NewHostRunner(),
		EnvManager:       env.NewEnvManager(),
		ProxyManager:     infraproxy.NewManager(),
		DiscoveryManager: discovery.NewManager(),
		SnapshotManager:  snapshot.NewManager(""),
		TunnelManager:    tunnel.NewManager(),
	}
}
