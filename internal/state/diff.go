package state

import (
	"fmt"
	"reflect"
	"sort"

	"raioz/internal/config"
)

// ConfigChange represents a detected change in configuration
type ConfigChange struct {
	Type     string // "service", "infra", "project"
	Name     string // Service/infra name
	Field    string // Field that changed
	OldValue string
	NewValue string
}

// CompareDeps compares two Deps configurations and returns detected changes
func CompareDeps(oldDeps *config.Deps, newDeps *config.Deps) ([]ConfigChange, error) {
	var changes []ConfigChange

	if oldDeps == nil {
		// No previous state, everything is new
		return changes, nil
	}

	// Compare project
	if oldDeps.Project.Name != newDeps.Project.Name {
		changes = append(changes, ConfigChange{
			Type:     "project",
			Name:     "project",
			Field:    "name",
			OldValue: oldDeps.Project.Name,
			NewValue: newDeps.Project.Name,
		})
	}
	oldNetworkName := oldDeps.Network.GetName()
	newNetworkName := newDeps.Network.GetName()
	if oldNetworkName != newNetworkName {
		changes = append(changes, ConfigChange{
			Type:     "project",
			Name:     "project",
			Field:    "network",
			OldValue: oldNetworkName,
			NewValue: newNetworkName,
		})
	}

	// Compare services
	changes = append(changes, compareServices(oldDeps.Services, newDeps.Services)...)

	// Compare infra
	changes = append(changes, compareInfra(oldDeps.Infra, newDeps.Infra)...)

	return changes, nil
}

// compareServices compares two service maps and returns changes
func compareServices(oldServices map[string]config.Service, newServices map[string]config.Service) []ConfigChange {
	var changes []ConfigChange

	// Collect all service names
	allNames := make(map[string]bool)
	for name := range oldServices {
		allNames[name] = true
	}
	for name := range newServices {
		allNames[name] = true
	}

	// Convert to sorted list for consistent comparison
	var names []string
	for name := range allNames {
		names = append(names, name)
	}
	sort.Strings(names)

	// Compare each service
	for _, name := range names {
		oldSvc, oldExists := oldServices[name]
		newSvc, newExists := newServices[name]

		if !oldExists {
			// New service
			changes = append(changes, ConfigChange{
				Type:     "service",
				Name:     name,
				Field:    "added",
				OldValue: "",
				NewValue: "new service",
			})
			continue
		}

		if !newExists {
			// Removed service
			changes = append(changes, ConfigChange{
				Type:     "service",
				Name:     name,
				Field:    "removed",
				OldValue: "service existed",
				NewValue: "",
			})
			continue
		}

		// Compare service fields
		svcChanges := compareServiceFields(name, oldSvc, newSvc)
		changes = append(changes, svcChanges...)
	}

	return changes
}

// compareServiceFields compares individual fields of a service
func compareServiceFields(name string, oldSvc config.Service, newSvc config.Service) []ConfigChange {
	var changes []ConfigChange

	// Compare source (git branches, image tags, etc.)
	if oldSvc.Source.Branch != newSvc.Source.Branch {
		changes = append(changes, ConfigChange{
			Type:     "service",
			Name:     name,
			Field:    "source.branch",
			OldValue: oldSvc.Source.Branch,
			NewValue: newSvc.Source.Branch,
		})
	}
	if oldSvc.Source.Tag != newSvc.Source.Tag {
		changes = append(changes, ConfigChange{
			Type:     "service",
			Name:     name,
			Field:    "source.tag",
			OldValue: oldSvc.Source.Tag,
			NewValue: newSvc.Source.Tag,
		})
	}
	if oldSvc.Source.Image != newSvc.Source.Image {
		changes = append(changes, ConfigChange{
			Type:     "service",
			Name:     name,
			Field:    "source.image",
			OldValue: oldSvc.Source.Image,
			NewValue: newSvc.Source.Image,
		})
	}

	// Compare service-level dependsOn
	if !reflect.DeepEqual(oldSvc.DependsOn, newSvc.DependsOn) {
		oldDepends := formatSlice(oldSvc.DependsOn)
		newDepends := formatSlice(newSvc.DependsOn)
		changes = append(changes, ConfigChange{
			Type:     "service",
			Name:     name,
			Field:    "dependsOn",
			OldValue: oldDepends,
			NewValue: newDepends,
		})
	}

	// Compare docker config (only if both services have docker config)
	if oldSvc.Docker != nil && newSvc.Docker != nil {
		if !reflect.DeepEqual(oldSvc.Docker.Ports, newSvc.Docker.Ports) {
			oldPorts := formatSlice(oldSvc.Docker.Ports)
			newPorts := formatSlice(newSvc.Docker.Ports)
			changes = append(changes, ConfigChange{
				Type:     "service",
				Name:     name,
				Field:    "docker.ports",
				OldValue: oldPorts,
				NewValue: newPorts,
			})
		}
		if !reflect.DeepEqual(oldSvc.Docker.DependsOn, newSvc.Docker.DependsOn) {
			oldDepends := formatSlice(oldSvc.Docker.DependsOn)
			newDepends := formatSlice(newSvc.Docker.DependsOn)
			changes = append(changes, ConfigChange{
				Type:     "service",
				Name:     name,
				Field:    "docker.dependsOn",
				OldValue: oldDepends,
				NewValue: newDepends,
			})
		}
		if oldSvc.Docker.Dockerfile != newSvc.Docker.Dockerfile {
			changes = append(changes, ConfigChange{
				Type:     "service",
				Name:     name,
				Field:    "docker.dockerfile",
				OldValue: oldSvc.Docker.Dockerfile,
				NewValue: newSvc.Docker.Dockerfile,
			})
		}
		if oldSvc.Docker.Command != newSvc.Docker.Command {
			changes = append(changes, ConfigChange{
				Type:     "service",
				Name:     name,
				Field:    "docker.command",
				OldValue: oldSvc.Docker.Command,
				NewValue: newSvc.Docker.Command,
			})
		}
	} else if oldSvc.Docker != nil && newSvc.Docker == nil {
		// Service changed from docker to host execution
		changes = append(changes, ConfigChange{
			Type:     "service",
			Name:     name,
			Field:    "docker",
			OldValue: "docker config present",
			NewValue: "host execution (source.command)",
		})
	} else if oldSvc.Docker == nil && newSvc.Docker != nil {
		// Service changed from host to docker execution
		changes = append(changes, ConfigChange{
			Type:     "service",
			Name:     name,
			Field:    "docker",
			OldValue: "host execution (source.command)",
			NewValue: "docker config present",
		})
	}

	return changes
}

// compareInfra compares two infra maps and returns changes
func compareInfra(oldInfra map[string]config.InfraEntry, newInfra map[string]config.InfraEntry) []ConfigChange {
	var changes []ConfigChange

	allNames := make(map[string]bool)
	for name := range oldInfra {
		allNames[name] = true
	}
	for name := range newInfra {
		allNames[name] = true
	}
	var names []string
	for name := range allNames {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		oldEntry, oldExists := oldInfra[name]
		newEntry, newExists := newInfra[name]

		if !oldExists {
			changes = append(changes, ConfigChange{
				Type: "infra", Name: name, Field: "added", OldValue: "", NewValue: "new infra",
			})
			continue
		}
		if !newExists {
			changes = append(changes, ConfigChange{
				Type: "infra", Name: name, Field: "removed", OldValue: "infra existed", NewValue: "",
			})
			continue
		}

		if (oldEntry.Path != "") != (newEntry.Path != "") {
			changes = append(changes, ConfigChange{
				Type: "infra", Name: name, Field: "definition", OldValue: "path or inline", NewValue: "inline or path",
			})
			continue
		}
		if oldEntry.Path != "" {
			if oldEntry.Path != newEntry.Path {
				changes = append(changes, ConfigChange{
					Type: "infra", Name: name, Field: "path", OldValue: oldEntry.Path, NewValue: newEntry.Path,
				})
			}
			continue
		}

		oldInf, newInf := oldEntry.Inline, newEntry.Inline
		if oldInf == nil || newInf == nil {
			continue
		}
		if oldInf.Image != newInf.Image {
			changes = append(changes, ConfigChange{
				Type: "infra", Name: name, Field: "image", OldValue: oldInf.Image, NewValue: newInf.Image,
			})
		}
		if oldInf.Tag != newInf.Tag {
			changes = append(changes, ConfigChange{
				Type: "infra", Name: name, Field: "tag", OldValue: oldInf.Tag, NewValue: newInf.Tag,
			})
		}
		if !reflect.DeepEqual(oldInf.Ports, newInf.Ports) {
			changes = append(changes, ConfigChange{
				Type: "infra", Name: name, Field: "ports",
				OldValue: formatSlice(oldInf.Ports), NewValue: formatSlice(newInf.Ports),
			})
		}
	}

	return changes
}

// formatSlice formats a string slice for display
func formatSlice(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	return fmt.Sprintf("%v", s)
}

// HasSignificantChanges checks if changes are significant enough to require recreation
func HasSignificantChanges(changes []ConfigChange) bool {
	for _, change := range changes {
		// Significant changes that require recreation
		significantFields := []string{
			"source.branch",
			"source.tag",
			"source.image",
			"dependsOn",
			"docker.ports",
			"docker.dependsOn",
			"docker.dockerfile",
			"docker.command",
			"image",
			"tag",
			"ports",
			"added",
			"removed",
		}

		for _, field := range significantFields {
			if change.Field == field {
				return true
			}
		}
	}
	return false
}

// FormatChanges formats changes for display
func FormatChanges(changes []ConfigChange) string {
	if len(changes) == 0 {
		return "No changes detected"
	}

	var result string
	result += fmt.Sprintf("Detected %d configuration change(s):\n", len(changes))

	for _, change := range changes {
		if change.OldValue == "" {
			result += fmt.Sprintf("  + %s.%s: %s (new)\n", change.Name, change.Field, change.NewValue)
		} else if change.NewValue == "" {
			result += fmt.Sprintf("  - %s.%s: %s (removed)\n", change.Name, change.Field, change.OldValue)
		} else {
			result += fmt.Sprintf("  ~ %s.%s: %s -> %s\n",
				change.Name, change.Field, change.OldValue, change.NewValue)
		}
	}

	return result
}
