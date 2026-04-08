package config

// ResolveDependencies takes a list of requested service/infra names and returns
// the full set of names needed (including transitive dependencies).
// It walks the dependency graph: if "api" depends on "database", and you request "api",
// the result includes both "api" and "database".
func ResolveDependencies(deps *Deps, requested []string) (services []string, infra []string) {
	serviceSet := make(map[string]bool)
	infraSet := make(map[string]bool)
	visited := make(map[string]bool)

	var resolve func(name string)
	resolve = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		// Check if it's a service
		if svc, ok := deps.Services[name]; ok {
			serviceSet[name] = true
			for _, dep := range svc.GetDependsOn() {
				resolve(dep)
			}
			return
		}

		// Check if it's infra
		if _, ok := deps.Infra[name]; ok {
			infraSet[name] = true
			return
		}
	}

	for _, name := range requested {
		resolve(name)
	}

	for name := range serviceSet {
		services = append(services, name)
	}
	for name := range infraSet {
		infra = append(infra, name)
	}

	return services, infra
}

// FilterByServices returns a new Deps with only the specified services and infra.
// Other fields (project, env, network, etc.) are preserved.
func FilterByServices(deps *Deps, serviceNames []string, infraNames []string) *Deps {
	serviceSet := make(map[string]bool)
	for _, name := range serviceNames {
		serviceSet[name] = true
	}
	infraSet := make(map[string]bool)
	for _, name := range infraNames {
		infraSet[name] = true
	}

	filtered := *deps
	filtered.Services = make(map[string]Service)
	filtered.Infra = make(map[string]InfraEntry)

	for name, svc := range deps.Services {
		if serviceSet[name] {
			filtered.Services[name] = svc
		}
	}
	for name, entry := range deps.Infra {
		if infraSet[name] {
			filtered.Infra[name] = entry
		}
	}

	return &filtered
}
