package naming

import "context"

// ContainerLookup is the contract a Docker adapter satisfies so naming can
// query the live container set without importing internal/docker (which
// already imports naming). Implementations live next to their Docker
// helpers; the only caller-supplied wiring is the choice of adapter.
type ContainerLookup interface {
	// Exists reports whether a container with the given name exists at
	// all (running or stopped). Errors are propagated; "not found" is
	// false with nil error.
	Exists(ctx context.Context, name string) (bool, error)
	// FindByLabels returns the names of containers whose Docker labels
	// match every key/value in `labels`. Empty result is not an error.
	FindByLabels(ctx context.Context, labels map[string]string) []string
}

// ResolveContainer returns the actual container name for a dep/service
// owned by `project`. Strategy:
//
//  1. Try the canonical name from DepContainer. If the container exists,
//     return it — this is the hot path covering every workspace-shared
//     or per-project dep raioz itself created.
//  2. Otherwise, fall back to a label-based lookup
//     (com.raioz.managed=true + com.raioz.service + com.raioz.project).
//     Catches the case where a `compose:` dep declared
//     `container_name:` in the user's docker-compose, so the canonical
//     name no longer matches the live container.
//  3. Return "" with nil error when neither resolves — callers decide
//     whether that's expected (status reports "stopped") or an error
//     (down sweep finds nothing to remove).
//
// `lookup` may be nil; the function then returns the canonical name
// without probing Docker. Useful for code paths that want a deterministic
// answer before any container is created (e.g. building stamp values).
//
// See ADR-001 (label-based identity) for the underlying contract.
func ResolveContainer(
	ctx context.Context,
	lookup ContainerLookup,
	project, service, nameOverride string,
) (string, error) {
	canonical := DepContainer(project, service, nameOverride)
	if lookup == nil {
		return canonical, nil
	}
	exists, err := lookup.Exists(ctx, canonical)
	if err != nil {
		return "", err
	}
	if exists {
		return canonical, nil
	}

	filters := map[string]string{
		LabelManaged: "true",
		LabelService: service,
	}
	if project != "" {
		filters[LabelProject] = project
	}
	matches := lookup.FindByLabels(ctx, filters)
	if len(matches) > 0 {
		return matches[0], nil
	}
	return "", nil
}

// ContainerTarget is ResolveContainer with the canonical name as a fallback
// when no live container is found. Use this when the result is being used
// to *address* a container (Caddy upstream, discovery env var) that may
// not be up yet — the canonical is the name raioz will create.
//
// Errors from the underlying lookup are swallowed and the canonical is
// returned. Callers that need to distinguish "container not found" from
// "Docker probe failed" should use ResolveContainer directly.
func ContainerTarget(
	ctx context.Context,
	lookup ContainerLookup,
	project, service, nameOverride string,
) string {
	name, err := ResolveContainer(ctx, lookup, project, service, nameOverride)
	if err != nil || name == "" {
		return DepContainer(project, service, nameOverride)
	}
	return name
}
