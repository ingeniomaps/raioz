package production

import (
	"fmt"
	"sort"
)

// MigratedService is a compose service that becomes a raioz service: it
// has a `build:`, so there is local code behind it.
type MigratedService struct {
	Path      string
	DependsOn []string
	EnvFiles  []string
}

// MigratedDependency is a compose service that becomes a raioz dependency:
// it only names an image.
type MigratedDependency struct {
	Image    string
	Ports    []string
	Volumes  []string
	EnvFiles []string
}

// MigratedProject is what a compose file turns into under raioz's model,
// where `services:` is code the developer edits and `dependencies:` is
// images they consume. That split is what a compose file does not have and
// the migration has to work out: `build:` means code, a bare `image:`
// means a dependency.
type MigratedProject struct {
	Services     map[string]MigratedService
	Dependencies map[string]MigratedDependency
	// Warnings lists what could not be carried over, for the user to
	// finish by hand.
	Warnings []string
}

// MigrateCompose classifies the services of a compose file.
func MigrateCompose(prod *ProductionConfig) *MigratedProject {
	out := &MigratedProject{
		Services:     map[string]MigratedService{},
		Dependencies: map[string]MigratedDependency{},
	}

	names := make([]string, 0, len(prod.Services))
	for name := range prod.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		svc := prod.Services[name]
		if len(svc.Environment) > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"%s: %d inline environment value(s) not migrated — raioz.yaml references env files, "+
					"move them to one and list it under `env:`", name, len(svc.Environment)))
		}

		if context := buildContext(svc.Build); context != "" {
			out.Services[name] = MigratedService{
				Path:      context,
				DependsOn: ParseDependsOn(svc.DependsOn),
				EnvFiles:  svc.EnvFile,
			}
			if len(svc.Ports) > 0 {
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"%s: ports %v not migrated — declare `port:` if the service needs a fixed one",
					name, NormalizePorts(svc.Ports)))
			}
			continue
		}

		if svc.Image == "" {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"%s: neither `image:` nor `build:`, skipped", name))
			continue
		}
		if deps := ParseDependsOn(svc.DependsOn); len(deps) > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"%s: depends_on %v dropped — a dependency does not declare an order in raioz.yaml", name, deps))
		}
		out.Dependencies[name] = MigratedDependency{
			Image:    svc.Image,
			Ports:    NormalizePorts(svc.Ports),
			Volumes:  svc.Volumes,
			EnvFiles: svc.EnvFile,
		}
	}
	return out
}

// buildContext returns the build context of a compose `build:` entry,
// which is either the context itself or a mapping with a `context` key.
// Empty when the service does not build.
func buildContext(build interface{}) string {
	switch b := build.(type) {
	case string:
		return b
	case map[string]interface{}:
		if context, ok := b["context"].(string); ok && context != "" {
			return context
		}
		return "."
	}
	return ""
}
