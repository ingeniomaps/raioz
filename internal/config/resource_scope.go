package config

import (
	"sort"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
)

// serviceResourceWarnings flags a service that declares `resources:` but
// does not run in a container raioz creates. The cap is silently without
// effect there: a host process is not a container, and a compose stack sets
// its own limits in its own file. A root default is not flagged — it is a
// default, and it simply skips what it cannot reach.
func serviceResourceWarnings(cfg *RaiozConfig) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Services))
	for name, svc := range cfg.Services {
		if !svc.Resources.IsZero() {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var warnings []string
	for _, name := range names {
		service, err := yamlServiceToService(name, cfg.Services[name])
		if err != nil {
			continue
		}
		runtime := ResolveServiceDetection(service, service.Source.Path).Runtime
		if runtime != models.RuntimeDockerfile {
			warnings = append(warnings, i18n.T("warning.service_resources_no_effect", name, string(runtime)))
		}
	}
	return warnings
}
