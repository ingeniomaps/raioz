package upcase

import "raioz/internal/config"

// resolveHostnameAndAliases returns the proxy hostname override and the
// alias list for name, honoring the "service first, dep (infra) last"
// precedence both share. Returns ("", nil) when nothing is declared — the
// caller keeps its own default (the entry name).
func resolveHostnameAndAliases(deps *config.Deps, name string) (string, []string) {
	var hostname string
	var aliases []string
	if svc, ok := deps.Services[name]; ok {
		if svc.Hostname != "" {
			hostname = svc.Hostname
		}
		if len(svc.HostnameAliases) > 0 {
			aliases = append([]string(nil), svc.HostnameAliases...)
		}
	}
	if entry, ok := deps.Infra[name]; ok && entry.Inline != nil {
		if entry.Inline.Hostname != "" {
			hostname = entry.Inline.Hostname
		}
		if len(entry.Inline.HostnameAliases) > 0 {
			aliases = append([]string(nil), entry.Inline.HostnameAliases...)
		}
	}
	return hostname, aliases
}
