package config

import (
	"fmt"
	"slices"
	"sort"
)

// validAuthValues is the closed enum of accepted `services.<n>.auth`
// values. Empty (the default) reproduces the v0.1 strict hardening.
// gh and ssh are recognized at the schema level even though their
// providers register in fase 2 / 3 — that way the validator stays
// stable across the rollout.
var validAuthValues = []string{"", "inherit", "gh", "ssh"}

// validateAuthValues rejects unknown auth providers at preflight.
// Called from validateYAMLConfig so a typo'd auth: surfaces before
// any side-effect (clone, pull, exec).
func validateAuthValues(cfg *RaiozConfig, path string) error {
	if cfg == nil {
		return nil
	}
	for name, svc := range cfg.Services {
		if !slices.Contains(validAuthValues, svc.Auth) {
			return fmt.Errorf(
				"service %q in %s: invalid auth provider %q "+
					"(valid: omit for default, or one of: inherit, gh, ssh)",
				name, path, svc.Auth)
		}
	}
	return nil
}

// authWarnings reports services that declare auth: without git:.
// The bridge silently drops auth on path-only services (the field is
// stamped on SourceConfig only inside the git branch); a warning
// makes that silent drop visible so the dev catches the
// misconfiguration.
//
// Warning, not error, because dropping the value is benign — the
// service still runs correctly, just with the wrong auth strategy
// (none, since it has no remote to authenticate against).
func authWarnings(cfg *RaiozConfig) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	var warnings []string
	for _, name := range names {
		svc := cfg.Services[name]
		if svc.Auth != "" && svc.Git == "" {
			warnings = append(warnings, fmt.Sprintf(
				"service %q declares auth: %q without git: — auth is "+
					"only honored on git sources and will be ignored",
				name, svc.Auth))
		}
	}
	return warnings
}
