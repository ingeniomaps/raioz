package production

import (
	"fmt"
	"strings"

	"raioz/internal/config"
)

// MigrateComposeToDeps converts a production Docker Compose configuration to .raioz.json format
func MigrateComposeToDeps(
	prod *ProductionConfig,
	projectName string,
	networkName string,
) (*config.Deps, error) {
	if projectName == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if networkName == "" {
		networkName = fmt.Sprintf("%s-network", projectName)
	}

	deps := &config.Deps{
		SchemaVersion: "1.0",
		Network:       config.NetworkConfig{Name: networkName, IsObject: false},
		Project: config.Project{
			Name: projectName,
		},
		Services: make(map[string]config.Service),
		Infra:    make(map[string]config.Infra),
		Env: config.EnvConfig{
			UseGlobal: false,
			Files:     []string{},
		},
	}

	// Separate services and infrastructure
	for name, prodSvc := range prod.Services {
		if isInfraService(name) {
			// Add to infra
			infra := config.Infra{
				Image:   "",
				Tag:     "latest",
				Ports:   []string{},
				Volumes: []string{},
				Env:     nil,
			}

			image, tag := ExtractImageAndTag(prodSvc.Image)
			infra.Image = image
			infra.Tag = tag

			if len(prodSvc.Ports) > 0 {
				infra.Ports = NormalizePorts(prodSvc.Ports)
			}

			if len(prodSvc.Volumes) > 0 {
				infra.Volumes = prodSvc.Volumes
			}

			deps.Infra[name] = infra
		} else {
			// Add to services
			svc := config.Service{
				Source: config.SourceConfig{
					Kind: "image",
				},
				Docker: &config.DockerConfig{
					Mode:      "prod",
					Ports:     []string{},
					Volumes:   []string{},
					DependsOn: []string{},
				},
				Env: nil,
			}

			image, tag := ExtractImageAndTag(prodSvc.Image)
			svc.Source.Image = image
			svc.Source.Tag = tag
			svc.Source.Kind = "image"

			if len(prodSvc.Ports) > 0 {
				svc.Docker.Ports = NormalizePorts(prodSvc.Ports)
			}

			if len(prodSvc.Volumes) > 0 {
				svc.Docker.Volumes = prodSvc.Volumes
			}

			depends := ParseDependsOn(prodSvc.DependsOn)
			if len(depends) > 0 {
				svc.Docker.DependsOn = depends
			}

			deps.Services[name] = svc
		}
	}

	return deps, nil
}

// ValidateMigratedDeps validates that a migrated .raioz.json is compatible with the schema
func ValidateMigratedDeps(deps *config.Deps) []string {
	var warnings []string

	// Check for required fields
	if deps.SchemaVersion != "1.0" {
		warnings = append(warnings, fmt.Sprintf("Schema version should be '1.0', got '%s'", deps.SchemaVersion))
	}

	if deps.Project.Name == "" {
		warnings = append(warnings, "Project name is required")
	}

	if deps.Network.GetName() == "" {
		warnings = append(warnings, "Project network is required")
	}

	// Check services
	for name, svc := range deps.Services {
		if svc.Source.Kind == "" {
			warnings = append(warnings, fmt.Sprintf("Service '%s': source kind is required", name))
		}

		if svc.Source.Kind == "image" {
			if svc.Source.Image == "" {
				warnings = append(warnings, fmt.Sprintf("Service '%s': image is required for image source", name))
			}
			if svc.Source.Tag == "" {
				warnings = append(warnings, fmt.Sprintf("Service '%s': tag defaults to 'latest'", name))
				svc.Source.Tag = "latest"
			}
		}

		if svc.Docker.Mode == "" {
			warnings = append(warnings, fmt.Sprintf("Service '%s': mode defaults to 'prod'", name))
			svc.Docker.Mode = "prod"
		} else if svc.Docker.Mode != "dev" && svc.Docker.Mode != "prod" {
			warnings = append(warnings, fmt.Sprintf("Service '%s': invalid mode '%s', using 'prod'", name, svc.Docker.Mode))
			svc.Docker.Mode = "prod"
		}

		// Check dependencies exist
		for _, dep := range svc.Docker.DependsOn {
			if _, exists := deps.Services[dep]; !exists {
				if _, exists := deps.Infra[dep]; !exists {
					warnings = append(warnings, fmt.Sprintf(
						"Service '%s': dependency '%s' not found in services or infra", name, dep))
				}
			}
		}
	}

	// Check infra
	for name, infra := range deps.Infra {
		if infra.Image == "" {
			warnings = append(warnings, fmt.Sprintf("Infra '%s': image is required", name))
		}
		if infra.Tag == "" {
			warnings = append(warnings, fmt.Sprintf("Infra '%s': tag defaults to 'latest'", name))
		}
	}

	return warnings
}

// SuggestGitSource suggests a git source configuration for a service based on common patterns
func SuggestGitSource(serviceName, imageName string) *config.SourceConfig {
	// Common patterns for suggesting git repos
	// This is a best-effort suggestion
	if strings.Contains(imageName, "gcr.io") || strings.Contains(imageName, "docker.io") {
		// Extract org/repo from image name
		parts := strings.Split(imageName, "/")
		if len(parts) >= 2 {
			orgRepo := parts[len(parts)-1]
			// Remove tag if present
			orgRepo = strings.Split(orgRepo, ":")[0]

			return &config.SourceConfig{
				Kind:   "git",
				Repo:   fmt.Sprintf("git@github.com:org/%s.git", orgRepo),
				Branch: "main",
				Path:   fmt.Sprintf("services/%s", serviceName),
			}
		}
	}

	return nil
}

// EnhanceMigratedDeps enhances a migrated .raioz.json with suggestions for git sources
func EnhanceMigratedDeps(deps *config.Deps) {
	for name, svc := range deps.Services {
		// If service uses image source, suggest git source if possible
		if svc.Source.Kind == "image" {
			suggested := SuggestGitSource(name, svc.Source.Image)
			if suggested != nil {
				// Don't overwrite, but could be used as a hint
				// For now, we keep the image source
			}
		}
	}
}
