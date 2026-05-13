package interfaces

import "context"

// NetworkManager covers Docker network lifecycle: create-or-reuse,
// inspect which projects touch a network, sweep unused.
//
// ADR-012: one of six segregated interfaces composed by DockerRunner.
type NetworkManager interface {
	// EnsureNetworkWithConfigAndContext creates the network (with an
	// optional subnet and labels) when it doesn't exist; reuses it
	// otherwise. Labels MUST be populated before this call — Docker
	// forbids retro-labeling, so a re-up of a labelless network keeps
	// it labelless. Empty/nil labels preserve the pre-label behavior.
	EnsureNetworkWithConfigAndContext(
		ctx context.Context, name, subnet string,
		labels map[string]string, askConfirmation bool,
	) error

	// GetNetworkProjects returns the list of project names that have
	// containers attached to networkName. Used by `down` to decide
	// whether the network is still needed after sweep.
	GetNetworkProjects(networkName, baseDir string) ([]string, error)

	// IsNetworkInUseWithContext reports whether any container is
	// currently connected to networkName.
	IsNetworkInUseWithContext(ctx context.Context, networkName string) (bool, error)

	// CleanUnusedNetworksWithContext sweeps every raioz-managed network
	// with zero connected containers. Honors dryRun: when true returns
	// the candidate list without removing.
	CleanUnusedNetworksWithContext(ctx context.Context, dryRun bool) ([]string, error)
}
