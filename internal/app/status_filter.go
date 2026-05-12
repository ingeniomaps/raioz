package app

import (
	"fmt"
	"strings"

	"raioz/internal/config"
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

// countMatching returns how many keys of m pass the filter. Used to
// decide whether to print the section header / count.
func countMatching(m map[string]config.InfraEntry, want map[string]struct{}) int {
	if want == nil {
		return len(m)
	}
	n := 0
	for k := range m {
		if _, ok := want[k]; ok {
			n++
		}
	}
	return n
}

// countMatchingSvc is the services counterpart. Same shape — Go's lack of
// generics over map value types makes the duplication unavoidable here.
func countMatchingSvc(m map[string]config.Service, want map[string]struct{}) int {
	if want == nil {
		return len(m)
	}
	n := 0
	for k := range m {
		if _, ok := want[k]; ok {
			n++
		}
	}
	return n
}

// validateStatusFilter fails fast with a useful error when the filter
// references a name that is neither a service nor a dependency. Otherwise
// the user would see an empty report and assume nothing is running, which
// is exactly the misleading UX that issue 014 was about (just inverted).
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
