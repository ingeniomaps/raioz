package config

import (
	"fmt"
	"strings"
)

// YAMLToDeps converts a RaiozConfig (from raioz.yaml) to the existing Deps structure.
// This bridge allows all existing code (filters, validation, orchestration) to work unchanged.
func YAMLToDeps(cfg *RaiozConfig) (*Deps, error) {
	// Reject configurations where the same name appears as both a service
	// and a dependency. Without this guard, two separate compose projects
	// end up fighting over the same container name and the resulting
	// behavior is effectively undefined (whichever starts last wins, the
	// other dies with "container name already in use").
	var collisions []string
	for name := range cfg.Services {
		if _, isDep := cfg.Deps[name]; isDep {
			collisions = append(collisions, name)
		}
	}
	if len(collisions) > 0 {
		return nil, fmt.Errorf(
			"name collision in raioz.yaml: %v appears in both services and dependencies. "+
				"Rename one side — services are per-project code you edit, dependencies "+
				"are shared images you consume", collisions)
	}

	// Each dependency must declare exactly one of `image:` or `compose:`.
	// Both is ambiguous (which one wins?); neither leaves raioz with
	// nothing to start.
	for name, dep := range cfg.Deps {
		hasImage := dep.Image != ""
		hasCompose := len(dep.Compose) > 0
		if hasImage && hasCompose {
			return nil, fmt.Errorf(
				"dependency %q declares both `image:` and `compose:` — pick one. "+
					"Use `compose:` when you have a pre-existing docker-compose fragment "+
					"(volumes, healthchecks, custom entrypoints); `image:` when you just "+
					"want a single container from a public image", name)
		}
		if !hasImage && !hasCompose {
			return nil, fmt.Errorf(
				"dependency %q must declare either `image:` (for a single container "+
					"from a public image) or `compose:` (path to a docker-compose file "+
					"you already maintain)", name)
		}
	}

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

	// Network config. Precedence: user-declared > workspace-derived > project-derived.
	// Only promote to IsObject=true when the user actually asked for a subnet —
	// otherwise downstream compose generation omits the ipam block and lets
	// Docker pick a subnet, matching the previous default.
	deps.Network = resolveNetworkConfig(cfg)

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

	// Custom start command (host exec, overrides auto-detection)
	if svc.Command != "" {
		service.Source.Command = svc.Command
	}

	// Explicit compose file(s) — overrides auto-detection
	if len(svc.Compose) > 0 {
		service.Source.ComposeFiles = []string(svc.Compose)
	}

	// Custom stop command — stored in Service.Commands.Down so the orchestrator
	// can route `raioz down` to it instead of SIGTERMing the host PID.
	if svc.Stop != "" {
		if service.Commands == nil {
			service.Commands = &ServiceCommands{}
		}
		service.Commands.Down = svc.Stop
	}

	// Convert ports to docker config if specified
	if len(svc.Ports) > 0 {
		service.Docker = &DockerConfig{
			Ports: []string(svc.Ports),
		}
	}

	// Explicit host port declaration (raioz.yaml: `port: 3000`).
	// Separate from `ports:` which is the Docker-style publish list; `port:`
	// is what the allocator uses to guarantee host-side exclusivity.
	if svc.Port > 0 {
		service.Port = svc.Port
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
	if svc.Proxy != nil && (svc.Proxy.Target != "" || svc.Proxy.Port > 0) {
		service.ProxyOverride = &ServiceProxyOverride{
			Target: svc.Proxy.Target,
			Port:   svc.Proxy.Port,
		}
	}

	return service, nil
}

// resolveNetworkConfig returns the NetworkConfig for a RaiozConfig, honoring
// any user-supplied `network:` block and falling back to the conventional
// workspace-or-project-derived name when the block is absent.
func resolveNetworkConfig(cfg *RaiozConfig) NetworkConfig {
	name := ""
	subnet := ""
	if cfg.Network != nil {
		name = cfg.Network.Name
		subnet = cfg.Network.Subnet
	}
	if name == "" {
		if cfg.Workspace != "" {
			name = cfg.Workspace + "-net"
		} else {
			name = cfg.Project + "-net"
		}
	}
	return NetworkConfig{
		Name:     name,
		Subnet:   subnet,
		IsObject: subnet != "",
	}
}

// yamlDependencyToInfra converts a YAMLDependency to an InfraEntry.
func yamlDependencyToInfra(dep YAMLDependency) InfraEntry {
	infra := &Infra{
		Name:    dep.Name,
		Image:   extractImage(dep.Image),
		Tag:     extractTag(dep.Image),
		Ports:   []string(dep.Ports),
		Volumes: []string(dep.Volumes),
		Compose: append([]string(nil), dep.Compose...),
	}

	if len(dep.Env) > 0 {
		infra.Env = &EnvValue{
			Files: []string(dep.Env),
		}
	}

	// Copy expose (container-side declaration). A single int or a list both
	// arrive here as a slice thanks to YAMLIntSlice's custom unmarshal.
	if len(dep.Expose) > 0 {
		infra.Expose = append([]int(nil), dep.Expose...)
	}

	// Translate the publish intent into the internal PublishSpec. Only
	// populate it when the user actually wrote something — we keep nil as
	// the "internal only" sentinel so ImageRunner / allocator can branch
	// cheaply on `if infra.Publish == nil`.
	if dep.Publish.Set && (dep.Publish.Auto || len(dep.Publish.Ports) > 0) {
		infra.Publish = &PublishSpec{
			Auto:  dep.Publish.Auto,
			Ports: append([]int(nil), dep.Publish.Ports...),
		}
	}

	// Propagate routing overrides so deps can opt into proxy routing even
	// when their image matches the DB-like heuristic that otherwise skips
	// them.
	if dep.Routing != nil {
		infra.Routing = dep.Routing
	}

	return InfraEntry{Inline: infra}
}

// yamlDeprecationWarnings walks a parsed RaiozConfig looking for legacy
// fields that still work but are superseded by newer, clearer alternatives.
// Returns a flat list of human-readable warnings; callers (cli/up.go) print
// them as yellow output.prints so the dev notices without it being fatal.
func yamlDeprecationWarnings(cfg *RaiozConfig) []string {
	if cfg == nil {
		return nil
	}
	var warnings []string
	for name, dep := range cfg.Deps {
		if len(dep.Ports) > 0 {
			warnings = append(warnings,
				fmt.Sprintf(
					"dependency '%s' uses legacy `ports:`; consider migrating to "+
						"`publish:` (host-side opt-in) and `expose:` (container-side "+
						"declaration) for clearer semantics",
					name,
				),
			)
		}
	}
	return warnings
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

// LoadDepsFromYAML loads a raioz.yaml and returns a Deps struct together with
// any deprecation warnings that surfaced during the load (legacy fields that
// still parse fine but should be migrated).
func LoadDepsFromYAML(path string) (*Deps, []string, error) {
	cfg, err := LoadYAML(path)
	if err != nil {
		return nil, nil, err
	}

	warnings := yamlDeprecationWarnings(cfg)

	deps, err := YAMLToDeps(cfg)
	if err != nil {
		return nil, warnings, err
	}

	return deps, warnings, nil
}
