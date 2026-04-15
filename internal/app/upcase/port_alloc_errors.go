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

// PortBindConflict describes a single host-port bind clash detected after
// allocation. The caller decides how to handle it (interactive prompt,
// hard error, auto-reassign, etc.).
type PortBindConflict struct {
	Kind     string // "service" or "dep"
	Name     string
	Port     int
	Explicit bool
}

// checkPortBindConflicts probes every resolved host port via a tcp listener
// and returns a conflict entry for each port that is already bound. Unlike
// the old bindCheckResult it does NOT return an error — the caller decides
// whether to fail, prompt the user, or auto-reassign.
func checkPortBindConflicts(result *PortAllocResult) []PortBindConflict {
	var conflicts []PortBindConflict
	for _, alloc := range result.Services {
		inUse, err := docker.CheckPortInUse(fmt.Sprintf("%d", alloc.Port))
		if err != nil || !inUse {
			continue
		}
		conflicts = append(conflicts, PortBindConflict{
			Kind:     "service",
			Name:     alloc.Name,
			Port:     alloc.Port,
			Explicit: alloc.Explicit,
		})
	}
	for _, alloc := range result.Deps {
		for _, m := range alloc.Mappings {
			inUse, err := docker.CheckPortInUse(fmt.Sprintf("%d", m.HostPort))
			if err != nil || !inUse {
				continue
			}
			conflicts = append(conflicts, PortBindConflict{
				Kind:     "dep",
				Name:     alloc.Name,
				Port:     m.HostPort,
				Explicit: alloc.Explicit,
			})
		}
	}
	return conflicts
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
