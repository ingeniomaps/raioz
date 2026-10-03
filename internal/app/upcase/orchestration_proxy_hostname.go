package upcase

import (
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// resolveHostnameAndAliases returns the proxy hostname override and the
// alias list for name, honoring the "service first, dep (infra) last"
// precedence both share. Returns ("", nil) when nothing is declared — the
// caller keeps its own default (the entry name).
func resolveHostnameAndAliases(deps *models.Deps, name string) (string, []string) {
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

// applyProxyURL records on the endpoint where the proxy serves it, so the
// <NAME>_HTTPS_URL raioz injects is the address that actually resolves:
// the declared `hostname:` under the proxy's domain, and nothing at all
// for an entry the proxy does not route.
func applyProxyURL(ep *interfaces.ServiceEndpoint, deps *models.Deps, name string) {
	if !deps.Proxy {
		return
	}
	if !shouldProxy(deps, name) {
		ep.Unrouted = true
		return
	}
	domain := "localhost"
	if deps.ProxyConfig != nil {
		if deps.ProxyConfig.Mode == "path" {
			return // path mode has no per-service hostname
		}
		if deps.ProxyConfig.Domain != "" {
			domain = deps.ProxyConfig.Domain
		}
	}
	hostname, _ := resolveHostnameAndAliases(deps, name)
	if hostname == "" {
		hostname = name
	}
	ep.ProxyURL = "https://" + hostname + "." + domain
}
