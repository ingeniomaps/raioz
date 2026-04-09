package config

import (
	"fmt"
	"strings"
)

// YAMLToDeps converts a RaiozConfig (from raioz.yaml) to the existing Deps structure.
// This bridge allows all existing code (filters, validation, orchestration) to work unchanged.
func YAMLToDeps(cfg *RaiozConfig) (*Deps, error) {
	deps := &Deps{
		SchemaVersion: "2.0",
		Workspace:     cfg.Workspace,
		Project: Project{
			Name: cfg.Project,
		},
		Services: make(map[string]Service),
		Infra:    make(map[string]InfraEntry),
	}

	// Convert proxy config
	if cfg.Proxy != nil && cfg.Proxy.Enabled {
		deps.Proxy = true
		deps.ProxyConfig = cfg.Proxy
	}

	// Convert pre/post hooks
	if len(cfg.Pre) > 0 {
		deps.PreHook = strings.Join(cfg.Pre, " && ")
	}
	if len(cfg.Post) > 0 {
		deps.PostHook = strings.Join(cfg.Post, " && ")
	}

	// Convert network from workspace name
	if cfg.Workspace != "" {
		deps.Network = NetworkConfig{Name: cfg.Workspace + "-net"}
	} else {
		deps.Network = NetworkConfig{Name: cfg.Project + "-net"}
	}

	// Convert services
	for name, svc := range cfg.Services {
		service, err := yamlServiceToService(name, svc)
		if err != nil {
			return nil, fmt.Errorf("service '%s': %w", name, err)
		}
		deps.Services[name] = service
	}

	// Convert dependencies to infra entries
	for name, dep := range cfg.Deps {
		entry := yamlDependencyToInfra(dep)
		deps.Infra[name] = entry
	}

	return deps, nil
}

// yamlServiceToService converts a YAMLService to the existing Service struct.
func yamlServiceToService(_ string, svc YAMLService) (Service, error) {
	service := Service{
		DependsOn: []string(svc.DependsOn),
		Profiles:  []string(svc.Profiles),
	}

	// Determine source kind
	if svc.Git != "" {
		service.Source = SourceConfig{
			Kind:   "git",
			Repo:   svc.Git,
			Branch: svc.Branch,
			Path:   svc.Path,
			Access: "editable",
		}
	} else if svc.Path != "" {
		service.Source = SourceConfig{
			Kind: "local",
			Path: svc.Path,
		}
	}

	// Convert ports to docker config if specified
	if len(svc.Ports) > 0 {
		service.Docker = &DockerConfig{
			Ports: []string(svc.Ports),
		}
	}

	// Convert env files
	if len(svc.Env) > 0 {
		service.Env = &EnvValue{
			Files: []string(svc.Env),
		}
	}

	// Store new fields in extended config
	service.Watch = svc.Watch
	service.HealthEndpoint = svc.Health
	service.Hostname = svc.Hostname
	service.Routing = svc.Routing

	return service, nil
}

// yamlDependencyToInfra converts a YAMLDependency to an InfraEntry.
func yamlDependencyToInfra(dep YAMLDependency) InfraEntry {
	infra := &Infra{
		Image:   extractImage(dep.Image),
		Tag:     extractTag(dep.Image),
		Ports:   []string(dep.Ports),
		Volumes: []string(dep.Volumes),
	}

	if len(dep.Env) > 0 {
		infra.Env = &EnvValue{
			Files: []string(dep.Env),
		}
	}

	return InfraEntry{Inline: infra}
}

// extractImage gets the image name from "image:tag" format.
func extractImage(imageRef string) string {
	parts := strings.SplitN(imageRef, ":", 2)
	return parts[0]
}

// extractTag gets the tag from "image:tag" format, defaulting to "latest".
func extractTag(imageRef string) string {
	parts := strings.SplitN(imageRef, ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return "latest"
}

// LoadDepsFromYAML loads a raioz.yaml and returns a Deps struct for backward compatibility.
func LoadDepsFromYAML(path string) (*Deps, []string, error) {
	cfg, err := LoadYAML(path)
	if err != nil {
		return nil, nil, err
	}

	deps, err := YAMLToDeps(cfg)
	if err != nil {
		return nil, nil, err
	}

	return deps, nil, nil
}
