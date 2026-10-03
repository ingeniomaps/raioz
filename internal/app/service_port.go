package app

import "raioz/internal/app/upcase"

// ServiceHostPort returns the host port the named service or dependency of
// the project at configPath takes, as raioz.yaml declares or infers it. ok
// is false when there is no config, no such name, or no host port.
func ServiceHostPort(deps *Dependencies, configPath, name string) (port int, ok bool) {
	if deps == nil || deps.ConfigLoader == nil || configPath == "" || configPath == ":auto:" {
		return 0, false
	}
	cfg, _, err := deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || cfg == nil {
		return 0, false
	}
	for _, wanted := range upcase.WantedHostPorts(cfg) {
		if wanted.Name == name {
			return wanted.Port, true
		}
	}
	return 0, false
}
