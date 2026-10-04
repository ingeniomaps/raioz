package app

import (
	"fmt"
	"strings"
)

// filterSet turns the args slice into a presence map for O(1) checks.
// Empty slice → nil map → inFilter always returns true (no filter active).
func filterSet(filter []string) map[string]struct{} {
	if len(filter) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(filter))
	for _, n := range filter {
		m[n] = struct{}{}
	}
	return m
}

// inFilter reports whether `name` should be shown. A nil map (no filter)
// matches everything; a non-nil map matches only declared keys.
func inFilter(want map[string]struct{}, name string) bool {
	if want == nil {
		return true
	}
	_, ok := want[name]
	return ok
}

// validateStatusFilter fails fast with a useful error when the filter
// references a name that is neither a service nor a dependency. Otherwise
// the user would see an empty report and assume nothing is running, which
// is exactly the misleading UX the filter is supposed to fix, just inverted.
func validateStatusFilter(proj *YAMLProject, filter []string) error {
	if len(filter) == 0 {
		return nil
	}
	var unknown []string
	for _, n := range filter {
		if _, ok := proj.Deps.Services[n]; ok {
			continue
		}
		if _, ok := proj.Deps.Infra[n]; ok {
			continue
		}
		unknown = append(unknown, n)
	}
	if len(unknown) == 0 {
		return nil
	}

	known := make([]string, 0, len(proj.Deps.Services)+len(proj.Deps.Infra))
	for n := range proj.Deps.Services {
		known = append(known, n)
	}
	for n := range proj.Deps.Infra {
		known = append(known, n)
	}
	return fmt.Errorf(
		"status: unknown service or dependency: %s (declared in raioz.yaml: %s)",
		strings.Join(unknown, ", "), strings.Join(known, ", "),
	)
}
