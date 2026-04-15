package proxy

import (
	"fmt"
	"net"
)

// DefaultProxyIP computes the conventional IP raioz assigns to the Caddy
// proxy when the user declared a network subnet but no explicit proxy IP.
//
// Convention: keep the two highest octets of the subnet base and append
// .1.1. So 172.28.0.0/16 → 172.28.1.1, 10.3.0.0/16 → 10.3.1.1. The
// address is memorable, stays inside the subnet for any /16 or larger, and
// avoids the two reserved low slots (network address and the .1 gateway
// that Docker always claims for the bridge).
//
// Returns "" (no error) when the subnet is empty, malformed, or too small
// to contain .1.1. Callers fall back to Docker auto-assignment in that
// case — a non-memorable IP beats no proxy at all.
func DefaultProxyIP(subnet string) string {
	if subnet == "" {
		return ""
	}
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return ""
	}
	base := ipnet.IP.To4()
	if base == nil {
		return "" // IPv6 subnets need their own scheme — not supported yet.
	}
	candidate := net.IPv4(base[0], base[1], 1, 1)
	if !ipnet.Contains(candidate) {
		return ""
	}
	return candidate.String()
}

// ValidateProxyIP ensures an explicit proxy IP falls inside the configured
// subnet AND is not the gateway (the .1 Docker reserves). Returns a
// descriptive error when validation fails so the user sees exactly what to
// fix in raioz.yaml.
func ValidateProxyIP(ip, subnet string) error {
	if ip == "" {
		return nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("proxy.ip %q is not a valid IPv4 address", ip)
	}

	if subnet == "" {
		// Without a subnet, Docker would reject --ip anyway. Tell the user
		// upfront instead of letting docker run fail cryptically.
		return fmt.Errorf(
			"proxy.ip requires network.subnet to be set (otherwise Docker " +
				"assigns from its own pool and cannot honor a specific IP)")
	}

	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return fmt.Errorf("network.subnet %q is not valid CIDR: %w", subnet, err)
	}
	if !ipnet.Contains(parsed) {
		return fmt.Errorf("proxy.ip %s is outside network.subnet %s", ip, subnet)
	}

	// Docker always assigns the first host address of the subnet to its
	// bridge. Using that IP for a container fails at docker run time; catch
	// it here for a clearer message.
	gateway := dockerGatewayFor(ipnet)
	if gateway != nil && gateway.Equal(parsed) {
		return fmt.Errorf(
			"proxy.ip %s is the Docker gateway for subnet %s — pick something else (e.g. %s.1.1)",
			ip, subnet, gatewayHint(gateway))
	}
	return nil
}

// dockerGatewayFor returns Docker's bridge gateway IP for a subnet: the
// first usable host address, which is base + 1.
func dockerGatewayFor(ipnet *net.IPNet) net.IP {
	base := ipnet.IP.To4()
	if base == nil {
		return nil
	}
	gw := make(net.IP, 4)
	copy(gw, base)
	gw[3]++
	if !ipnet.Contains(gw) {
		return nil
	}
	return gw
}

// gatewayHint returns the first two octets of an IP as "a.b" for use in
// user-facing error messages (e.g. "e.g. 172.28.1.1").
func gatewayHint(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d", v4[0], v4[1])
	}
	return ""
}
