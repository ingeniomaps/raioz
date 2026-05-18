package upcase

import (
	"fmt"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/netutil"
)

// privilegedPortCeiling is the first unprivileged port on POSIX. Binding below
// this requires root (CAP_NET_BIND_SERVICE). When a non-root raioz process is
// looking for an auto-bump target starting below this line, every net.Listen()
// attempt in the 1..1023 range would fail with EACCES — wasting ~944 syscalls.
// Skip straight to 1024 instead.
const privilegedPortCeiling = 1024

// portInUseProbe is the host-port-busy check used by findFreePort. Declared
// as a package variable so tests can stub it — the production flow always
// wants to see real host state, but a unit test asserting deterministic port
// numbers shouldn't fail just because the CI host happens to have 3000 bound.
var portInUseProbe = netutil.CheckPortInUse

// PortAllocation is the resolved port for a single service together with the
// metadata the orchestrator needs to wire it up.
type PortAllocation struct {
	// Name is the service name (as declared in raioz.yaml).
	Name string
	// Port is the final host port the service will listen on (for host
	// runtimes).
	Port int
	// Wanted is the port the service *wanted* before allocation (explicit
	// declaration or inferred default). Used for clear error messages.
	Wanted int
	// Explicit is true when the developer declared `port:` in raioz.yaml.
	// Explicit declarations are load-bearing: they are never silently
	// remapped — we fail loud on conflicts instead.
	Explicit bool
	// Runtime is the detected runtime for the service. Host runtimes need
	// PORT injection; docker runtimes don't.
	Runtime models.Runtime
}

// IsHost reports whether the service runs directly on the host and therefore
// needs its own host-level port (as opposed to a container port that lives in
// its own network namespace).
func (a PortAllocation) IsHost() bool {
	return !runtimeIsDocker(a.Runtime)
}

// runtimeIsDocker is a free-function mirror of models.DetectResult.IsDocker
// so the allocator can classify by Runtime alone (without a full DetectResult).
func runtimeIsDocker(r models.Runtime) bool {
	return r == models.RuntimeCompose ||
		r == models.RuntimeDockerfile ||
		r == models.RuntimeImage
}

// DepPortAllocation is the resolved host-side binding for a dependency that
// the user asked to publish. Nothing is set for internal-only deps (the
// dep allocations map simply has no entry for them).
type DepPortAllocation struct {
	Name     string
	Mappings []DepPortMapping
	Explicit bool // user pinned specific host ports vs asked for `publish: true`
}

// DepPortMapping is one container→host port pair. A dep with multiple exposed
// ports can have multiple mappings.
type DepPortMapping struct {
	HostPort      int
	ContainerPort int
}

// PortAllocResult is what AllocateHostPorts returns. Services carries the
// host-port assignments for services that run on the host; Deps carries the
// host-port assignments for dependencies that the user asked to publish.
// Both are produced in a single pass so they share the same taken-port set
// and cannot collide with each other.
type PortAllocResult struct {
	Services map[string]PortAllocation
	Deps     map[string]DepPortAllocation
}

// AllocateHostPorts walks services AND dependencies, classifies them, resolves
// wanted ports, detects conflicts, and deterministically assigns the final
// host port for everything that needs one.
//
// Rules:
//
//  1. Explicit declarations (`port: 3000` on a service, `publish: 5432` on a
//     dep) are sacred: they get the exact port or `up` fails. Two explicit
//     declarations on the same host port — whether two services, two deps,
//     or one of each — is a hard error.
//  2. Implicit/inferred ports (from runtime defaults) are negotiable: if two
//     services would pick the same port, raioz walks the list sorted by name
//     and bumps the later one to the next free port. Same for deps with
//     `publish: true` that fall back to expose defaults.
//  3. A cross-project conflict (another process already bound to the port on
//     the host) is detected via a cheap listener test and reported as an
//     error pointing at the offending service or dep.
//
// Docker services themselves are not allocated here — they live in their own
// network namespace so two of them can both run on container port 3000
// without conflicting on the host. Only dependencies the user *publishes*
// compete for the host port space.
func AllocateHostPorts(
	deps *models.Deps,
	detections DetectionMap,
) (*PortAllocResult, error) {
	result := &PortAllocResult{
		Services: map[string]PortAllocation{},
		Deps:     map[string]DepPortAllocation{},
	}
	// Shared "taken" set — maps port to a human-friendly owner label like
	// "service 'web-a'" or "dep 'postgres'". When a later pass wants that
	// port, the label becomes the error message verbatim.
	taken := map[int]string{}

	svcNames := sortedKeys(deps.Services)
	depNames := sortedKeys(deps.Infra)

	if err := allocExplicitServices(deps, detections, svcNames, taken, result); err != nil {
		return nil, err
	}
	if err := allocExplicitDeps(deps, depNames, taken, result); err != nil {
		return nil, err
	}
	if err := allocImplicitServices(deps, detections, svcNames, taken, result); err != nil {
		return nil, err
	}
	if err := allocAutoDeps(deps, depNames, taken, result); err != nil {
		return nil, err
	}
	// NOTE: host-port bind conflicts are checked by the caller via
	// checkPortBindConflicts() so they can be resolved interactively
	// instead of failing hard.
	return result, nil
}

// allocExplicitServices handles pass 1: services with `port: N` declared.
// Takes the port exactly and records ownership; two explicit services on
// the same port is a hard error.
func allocExplicitServices(
	deps *models.Deps,
	detections DetectionMap,
	svcNames []string,
	taken map[int]string,
	result *PortAllocResult,
) error {
	for _, name := range svcNames {
		svc := deps.Services[name]
		det := detections[name]
		if runtimeIsDocker(det.Runtime) || svc.Port <= 0 {
			continue
		}
		if holder, clash := taken[svc.Port]; clash {
			return portConflictExplicitError(
				fmt.Sprintf("service '%s'", name), holder, svc.Port,
			)
		}
		taken[svc.Port] = fmt.Sprintf("service '%s'", name)
		result.Services[name] = PortAllocation{
			Name:     name,
			Port:     svc.Port,
			Wanted:   svc.Port,
			Explicit: true,
			Runtime:  det.Runtime,
		}
	}
	return nil
}

// allocExplicitDeps handles pass 2: dependencies with `publish: N` or
// `publish: [N, M]`. Each explicit host port is claimed directly; conflicts
// with anything already in taken (including earlier services) are hard errors.
func allocExplicitDeps(
	deps *models.Deps,
	depNames []string,
	taken map[int]string,
	result *PortAllocResult,
) error {
	for _, name := range depNames {
		entry := deps.Infra[name]
		if entry.Inline == nil || entry.Inline.Publish == nil {
			continue
		}
		// Mode A sibling deps (issue #26) have no local container, so
		// any host port we'd reserve here would never get bound.
		// Skipping also frees the port for a regular dep that wants it.
		if entry.Inline.Project != "" {
			continue
		}
		pub := entry.Inline.Publish
		if pub.Auto || len(pub.Ports) == 0 {
			continue
		}
		mappings, err := assignExplicitDepPorts(name, entry.Inline, taken)
		if err != nil {
			return err
		}
		result.Deps[name] = DepPortAllocation{
			Name:     name,
			Mappings: mappings,
			Explicit: true,
		}
	}
	return nil
}

// allocImplicitServices handles pass 3: services without `port:` that need
// a host-facing port inferred from framework defaults or `.env PORT`.
// Collisions with anything in taken cause a deterministic bump upward.
func allocImplicitServices(
	deps *models.Deps,
	detections DetectionMap,
	svcNames []string,
	taken map[int]string,
	result *PortAllocResult,
) error {
	for _, name := range svcNames {
		if _, done := result.Services[name]; done {
			continue
		}
		svc := deps.Services[name]
		det := detections[name]
		if runtimeIsDocker(det.Runtime) {
			continue
		}
		wanted := inferServicePort(svc, det)
		if wanted <= 0 {
			continue
		}
		owner := fmt.Sprintf("service '%s'", name)
		final, err := findFreePort(wanted, taken, owner)
		if err != nil {
			return err
		}
		taken[final] = owner
		result.Services[name] = PortAllocation{
			Name:     name,
			Port:     final,
			Wanted:   wanted,
			Explicit: false,
			Runtime:  det.Runtime,
		}
	}
	return nil
}

// allocAutoDeps handles pass 4: dependencies with `publish: true`. For each
// port in Expose, raioz scans upward from the container port and finds the
// nearest free host port. Deps without an Expose declaration are skipped
// (the allocator logs a warning and leaves them internal-only).
func allocAutoDeps(
	deps *models.Deps,
	depNames []string,
	taken map[int]string,
	result *PortAllocResult,
) error {
	for _, name := range depNames {
		entry := deps.Infra[name]
		if entry.Inline == nil || entry.Inline.Publish == nil {
			continue
		}
		// Mode A sibling deps own no local container — see allocExplicitDeps.
		if entry.Inline.Project != "" {
			continue
		}
		if !entry.Inline.Publish.Auto {
			continue
		}
		mappings, err := assignAutoDepPorts(name, entry.Inline, taken)
		if err != nil {
			return err
		}
		if len(mappings) == 0 {
			continue
		}
		result.Deps[name] = DepPortAllocation{
			Name:     name,
			Mappings: mappings,
			Explicit: false,
		}
	}
	return nil
}

// assignExplicitDepPorts handles `publish: 5432` and `publish: [5432, 9090]`.
// Each requested host port is paired with the container port at the same
// index in Expose (or the same number if Expose is shorter/empty). Conflicts
// with anything already in `taken` fail loud.
func assignExplicitDepPorts(
	name string,
	infra *models.Infra,
	taken map[int]string,
) ([]DepPortMapping, error) {
	mappings := make([]DepPortMapping, 0, len(infra.Publish.Ports))
	owner := fmt.Sprintf("dep '%s'", name)

	for i, hostPort := range infra.Publish.Ports {
		containerPort := hostPort
		if i < len(infra.Expose) {
			containerPort = infra.Expose[i]
		}
		if holder, clash := taken[hostPort]; clash {
			return nil, portConflictExplicitError(owner, holder, hostPort)
		}
		taken[hostPort] = owner
		mappings = append(mappings, DepPortMapping{
			HostPort:      hostPort,
			ContainerPort: containerPort,
		})
	}
	return mappings, nil
}

// assignAutoDepPorts handles `publish: true`. We walk each container port in
// Expose and find the nearest free host port, starting from the container
// port itself so the common case (5432 free → host 5432) stays obvious.
// When Expose is empty we log a warning and return nothing — the user asked
// to publish but didn't tell us what, so we skip rather than guess.
func assignAutoDepPorts(
	name string,
	infra *models.Infra,
	taken map[int]string,
) ([]DepPortMapping, error) {
	if len(infra.Expose) == 0 {
		logging.Warn(
			"dependency has publish: true but no expose: declaration — skipping host binding",
			"dep", name,
		)
		return nil, nil
	}

	owner := fmt.Sprintf("dep '%s'", name)
	mappings := make([]DepPortMapping, 0, len(infra.Expose))

	for _, containerPort := range infra.Expose {
		hostPort, err := findFreePort(containerPort, taken, owner)
		if err != nil {
			return nil, err
		}
		taken[hostPort] = owner
		mappings = append(mappings, DepPortMapping{
			HostPort:      hostPort,
			ContainerPort: containerPort,
		})
	}
	return mappings, nil
}

// findFreePort + sortedKeys live in port_alloc_util.go. Error builders
// (portConflictExplicitError, checkPortBindConflicts, serviceBindError,
// depBindError) live in port_alloc_errors.go.
