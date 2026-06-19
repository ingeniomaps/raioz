package runtime

import (
	"os"
	"strings"
	"sync"
)

// Capability names a feature whose support differs across container
// runtimes (docker, podman, nerdctl) or runtime versions.
// raioz historically assumed flag parity; in practice nerdctl 1.x,
// podman < 4.7, and rootless setups diverge.
//
// V1 of the matrix supports only the most-impactful capabilities; the
// table is intentionally small so adding a new gate requires
// considered review (see ADR-046). Detection is conservative: when
// version detection fails or returns an unknown shape, raioz assumes
// the runtime DOES support the capability (matches today's behaviour
// — runners just emit flags and hope) and emits a debug log line so
// the choice is auditable.
type Capability int

const (
	// HostGatewayAlias is `--add-host=host.docker.internal:host-gateway`
	// support. Docker 24+, podman 4.7+, nerdctl 2.x. nerdctl 1.x must
	// use an explicit IP instead. Used by compose/dockerfile/image
	// runners.
	HostGatewayAlias Capability = iota
	// ComposeProfiles is `docker compose --profile` support. Docker
	// 24+, podman compose ≥ 4.6, nerdctl compose 1.7+.
	ComposeProfiles
	// LabelFilterOnDown is reliable `--filter "label=..."` support on
	// `compose down`. Some podman versions drop labels mid-teardown.
	LabelFilterOnDown
)

// String returns the canonical name used in the
// `RAIOZ_RUNTIME_CAPABILITY` override (`HostGatewayAlias=true,...`).
func (c Capability) String() string {
	switch c {
	case HostGatewayAlias:
		return "HostGatewayAlias"
	case ComposeProfiles:
		return "ComposeProfiles"
	case LabelFilterOnDown:
		return "LabelFilterOnDown"
	}
	return "Unknown"
}

// Supports reports whether the active runtime is known to support the
// given capability. Resolution order:
//
//  1. Operator override via RAIOZ_RUNTIME_CAPABILITY (fast path so
//     nerdctl 2.x users can opt back into HostGatewayAlias before
//     version detection lands).
//  2. Known-broken combinations (e.g. nerdctl + HostGatewayAlias).
//  3. Optimistic default (true).
func Supports(c Capability) bool {
	if v, ok := overrideFor(c); ok {
		return v
	}
	switch Binary() {
	case "nerdctl":
		// nerdctl 1.x doesn't support host-gateway alias. Without
		// version detection we can't distinguish 1.x from 2.x, so
		// we conservatively return false to avoid emitting an
		// unrecognised flag. Operators on nerdctl 2.x can opt back
		// in via RAIOZ_RUNTIME_CAPABILITY=HostGatewayAlias=true.
		if c == HostGatewayAlias {
			return false
		}
	}
	return true
}

// overrideFor parses RAIOZ_RUNTIME_CAPABILITY once per process and
// returns (value, true) when the env var pins the given capability.
// Format: comma-separated `Name=bool` pairs. Examples:
//
//	RAIOZ_RUNTIME_CAPABILITY=HostGatewayAlias=true
//	RAIOZ_RUNTIME_CAPABILITY=HostGatewayAlias=true,ComposeProfiles=false
//
// Unknown capability names and malformed entries are silently
// ignored — the env var is best-effort opt-in, not a validation
// surface.
func overrideFor(c Capability) (bool, bool) {
	m := loadCapabilityOverrides()
	v, ok := m[c.String()]
	return v, ok
}

var (
	capabilityOverridesOnce  sync.Once
	capabilityOverridesCache map[string]bool
)

func loadCapabilityOverrides() map[string]bool {
	capabilityOverridesOnce.Do(func() {
		raw := os.Getenv("RAIOZ_RUNTIME_CAPABILITY")
		capabilityOverridesCache = parseCapabilityOverrides(raw)
	})
	return capabilityOverridesCache
}

func parseCapabilityOverrides(raw string) map[string]bool {
	out := map[string]bool{}
	if raw == "" {
		return out
	}
	for _, entry := range strings.Split(raw, ",") {
		eq := strings.IndexByte(entry, '=')
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(entry[:eq])
		val := strings.TrimSpace(strings.ToLower(entry[eq+1:]))
		switch val {
		case "1", "true", "yes":
			out[name] = true
		case "0", "false", "no":
			out[name] = false
		}
	}
	return out
}

// ResetCapabilityOverridesForTest clears the once-cached parse so
// tests can verify the env-var lookup deterministically. Test-only.
func ResetCapabilityOverridesForTest() {
	capabilityOverridesOnce = sync.Once{}
	capabilityOverridesCache = nil
}
