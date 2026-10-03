package upcase

import (
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/netutil"
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

// shouldProxy decides whether to create a proxy route for a given service or
// dependency name. Services always get routed (they're the user's app). Deps
// get routed unless the image is on the known non-HTTP list AND the user
// didn't explicitly declare routing on it. Explicit `routing:` in raioz.yaml
// is the opt-in escape hatch for deps where the user knows they do speak
// HTTP (e.g. a custom image that happens to reuse a DB name).
func shouldProxy(deps *models.Deps, name string) bool {
	if svc, isService := deps.Services[name]; isService {
		// `proxy: false` opts a service out of routing — used for host-net
		// services with no UI (Prometheus, exporters) where a route would be
		// dead and misleading. Absence of the override keeps the default.
		return svc.ProxyOverride == nil || !svc.ProxyOverride.Disabled
	}
	entry, isDep := deps.Infra[name]
	if !isDep || entry.Inline == nil {
		return true
	}
	if entry.Inline.Routing != nil {
		return true
	}
	return !isNonHTTPImage(entry.Inline.Image)
}

// isNonHTTPImage delegates to the shared classifier in proxy/filter.go.
// Local alias kept for readability of nearby call sites.
func isNonHTTPImage(image string) bool {
	return netutil.IsNonHTTPImage(image)
}

// proxyTargetOverride returns the user's explicit (target, port) for a
// service or dependency if declared in raioz.yaml (`<kind>.<name>.proxy:`).
// Empty strings / zero port signal "not set" and caller must fall back to
// detection. This is the escape hatch for entries whose runtime raioz can't
// fully introspect — services with `command:` that launches a hidden compose
// stack, or dependencies using `compose:` / a non-default port.
func proxyTargetOverride(deps *models.Deps, name string) (string, int) {
	if svc, ok := deps.Services[name]; ok && svc.ProxyOverride != nil {
		return svc.ProxyOverride.Target, svc.ProxyOverride.Port
	}
	if entry, ok := deps.Infra[name]; ok && entry.Inline != nil && entry.Inline.ProxyOverride != nil {
		return entry.Inline.ProxyOverride.Target, entry.Inline.ProxyOverride.Port
	}
	return "", 0
}
