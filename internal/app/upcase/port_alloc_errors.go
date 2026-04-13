package upcase

import (
	"fmt"

	"raioz/internal/docker"
	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// The error builders for port allocation live here — extracted from
// port_alloc.go to keep that file under the 400-line cap without harming
// readability: each builder is pure and self-contained.

// portConflictExplicitError builds the two-owner conflict error used by the
// explicit passes (services and deps alike). Called when a declaration tries
// to take a port another explicit declaration already owns.
func portConflictExplicitError(owner, holder string, port int) error {
	return errors.New(
		errors.ErrCodePortConflict,
		fmt.Sprintf(
			i18n.T("error.port_conflict_explicit"),
			owner, holder, port,
		),
	).WithSuggestion(i18n.T("error.port_conflict_explicit_suggestion"))
}

// bindCheckResult probes every resolved host port via a tcp listener. If
// something outside this project already owns it, that's a real conflict
// raioz cannot resolve — fail fast with a clear pointer to the owner.
// Explicit paths end up here too: they added directly to `taken` during
// allocation instead of probing, so this pass is their conflict gate.
func bindCheckResult(result *PortAllocResult) error {
	for _, alloc := range result.Services {
		inUse, err := docker.CheckPortInUse(fmt.Sprintf("%d", alloc.Port))
		if err != nil || !inUse {
			continue
		}
		return serviceBindError(alloc)
	}
	for _, alloc := range result.Deps {
		for _, m := range alloc.Mappings {
			inUse, err := docker.CheckPortInUse(fmt.Sprintf("%d", m.HostPort))
			if err != nil || !inUse {
				continue
			}
			return depBindError(alloc, m)
		}
	}
	return nil
}

// serviceBindError builds the bind-clash error for a service. The explicit
// and implicit variants use different i18n keys so the messaging can reflect
// whether the dev made a hard commitment (explicit) or had a default picked
// for them (implicit).
func serviceBindError(alloc PortAllocation) error {
	if alloc.Explicit {
		return errors.New(
			errors.ErrCodePortConflict,
			fmt.Sprintf(
				i18n.T("error.port_conflict_host_explicit"),
				alloc.Name, alloc.Port,
			),
		).WithSuggestion(i18n.T("error.port_conflict_host_explicit_suggestion"))
	}
	return errors.New(
		errors.ErrCodePortConflict,
		fmt.Sprintf(
			i18n.T("error.port_conflict_host_implicit"),
			alloc.Name, alloc.Port,
		),
	).WithSuggestion(i18n.T("error.port_conflict_host_implicit_suggestion"))
}

// depBindError builds the bind-clash error for a dependency. Same split as
// serviceBindError — explicit `publish: N` vs auto `publish: true`.
func depBindError(alloc DepPortAllocation, m DepPortMapping) error {
	if alloc.Explicit {
		return errors.New(
			errors.ErrCodePortConflict,
			fmt.Sprintf(
				i18n.T("error.dep_publish_conflict_host_explicit"),
				alloc.Name, m.HostPort,
			),
		).WithSuggestion(i18n.T("error.dep_publish_conflict_host_explicit_suggestion"))
	}
	return errors.New(
		errors.ErrCodePortConflict,
		fmt.Sprintf(
			i18n.T("error.dep_publish_conflict_host_auto"),
			alloc.Name, m.HostPort,
		),
	).WithSuggestion(i18n.T("error.dep_publish_conflict_host_auto_suggestion"))
}
