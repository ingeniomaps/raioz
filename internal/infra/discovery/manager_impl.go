// Package discovery is a thin wrapper around internal/discovery so the
// wiring layer (internal/cli/wiring.go, ADR-018) builds
// every port adapter from a single uniform location under
// internal/infra/.
//
// The wrapper exists for layering, not for behavior — every call
// passes through to internal/discovery unchanged.
package discovery

import (
	"raioz/internal/discovery"
	"raioz/internal/domain/interfaces"
)

// NewManager returns the DiscoveryManager port implementation.
// internal/discovery.Manager already satisfies the interface; we just
// re-export the constructor from the infra side so callers don't
// import internal/discovery directly.
func NewManager() interfaces.DiscoveryManager {
	return discovery.NewManager()
}
