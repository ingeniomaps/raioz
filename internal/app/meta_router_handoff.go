package app

import (
	"net"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"raioz/internal/config"
	"raioz/internal/protocol"
)

// routerHandoffEnv computes the env vars the meta runner injects when
// spawning the router project, so the swap between the bundled Caddy
// and the router project preserves the operator's /etc/hosts setup
// (ADR-037 + issue 020).
//
// Today only RAIOZ_ROUTER_ASSIGNED_IP is computed. Other handoff vars
// (TLS mode, hostname domain) may join later — keep the helper so the
// list grows in one place.
//
// The IP is derived from the FIRST consumer that declares
// `network.subnet` (consumers in a meta workspace share the same
// CIDR by convention; the router yaml typically inherits via
// reference). When no subnet is declared anywhere, the helper returns
// no handoff vars — the router falls back to Docker auto-IP and the
// operator handles /etc/hosts manually (existing behaviour).
func routerHandoffEnv(cfg *config.MetaConfig) []string {
	subnet := firstConsumerSubnet(cfg)
	if subnet == "" {
		return nil
	}
	ip := defaultProxyIPLocal(subnet)
	if ip == "" {
		return nil
	}
	return []string{protocol.RouterAssignedIP + "=" + ip}
}

// defaultProxyIPLocal mirrors proxy.DefaultProxyIP without importing
// the proxy package (the app→proxy import would land on the ADR-029
// baseline that the drain plan is shrinking, not growing). Keep this
// in sync with internal/proxy/ip.go — if the convention changes,
// update both.
func defaultProxyIPLocal(subnet string) string {
	if subnet == "" {
		return ""
	}
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return ""
	}
	base := ipnet.IP.To4()
	if base == nil {
		return ""
	}
	candidate := net.IPv4(base[0], base[1], 1, 1)
	if !ipnet.Contains(candidate) {
		return ""
	}
	return candidate.String()
}

// firstConsumerSubnet returns the network.subnet declared by the
// first consumer that has one. Reads consumer yamls raw (yaml.Unmarshal
// without the H1/H2 gates) — this is best-effort handoff, not
// validation; a secret-tripped yaml downstream will surface its own
// error during spawn. Returns "" when no consumer declares a subnet.
func firstConsumerSubnet(cfg *config.MetaConfig) string {
	for _, p := range cfg.Projects {
		if s := readSubnetRaw(filepath.Join(p.Path, "raioz.yaml")); s != "" {
			return s
		}
		if s := readSubnetRaw(filepath.Join(p.Path, "raioz.yml")); s != "" {
			return s
		}
	}
	return ""
}

// readSubnetRaw reads yamlPath and returns the value at
// `network.subnet`, or "" on any error / missing field. Best-effort.
func readSubnetRaw(yamlPath string) string {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return ""
	}
	var raw config.RaiozConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	if raw.Network != nil {
		return raw.Network.Subnet
	}
	return ""
}
