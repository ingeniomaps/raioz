package runtime

// Capability names a feature whose support differs across container
// runtimes (docker, podman, nerdctl) or runtime versions. Issue 041 —
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
	// runners (issue 021).
	HostGatewayAlias Capability = iota
	// ComposeProfiles is `docker compose --profile` support. Docker
	// 24+, podman compose ≥ 4.6, nerdctl compose 1.7+.
	ComposeProfiles
	// LabelFilterOnDown is reliable `--filter "label=..."` support on
	// `compose down`. Some podman versions drop labels mid-teardown.
	LabelFilterOnDown
)

// Supports reports whether the active runtime is known to support the
// given capability. Conservative default (true) when detection can't
// classify the runtime. Issue 041.
func Supports(c Capability) bool {
	// V1: minimum-viable lookup keyed on the binary name only. Version
	// parsing is a follow-up (see ADR-046 § "What's deferred"). For
	// known-broken combinations we return false; everything else gets
	// the optimistic true.
	switch Binary() {
	case "nerdctl":
		// nerdctl 1.x doesn't support host-gateway alias. Without
		// version detection we can't distinguish 1.x from 2.x, so
		// we conservatively return false to avoid emitting an
		// unrecognised flag. Operators on nerdctl 2.x can opt back
		// in via RAIOZ_RUNTIME_CAPABILITY override (future).
		if c == HostGatewayAlias {
			return false
		}
	}
	return true
}
