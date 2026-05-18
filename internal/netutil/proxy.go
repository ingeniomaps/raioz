package netutil

import (
	"fmt"
	"net"
	"strings"
)

// DefaultProxyIP computes the conventional IP raioz assigns to the
// Caddy proxy when the user declares a network subnet but no explicit
// proxy.ip.
//
// Convention: keep the two highest octets of the subnet base and append
// .1.1. So 172.28.0.0/16 → 172.28.1.1, 10.3.0.0/16 → 10.3.1.1. The
// address is memorable, stays inside the subnet for any /16 or larger,
// and avoids the two reserved low slots (network address and the .1
// gateway that Docker always claims for the bridge).
//
// Returns "" (no error) when the subnet is empty, malformed, or too
// small to contain .1.1. Callers fall back to Docker auto-assignment in
// that case — a non-memorable IP beats no proxy at all.
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

// ValidateProxyIP ensures an explicit proxy IP falls inside the
// configured subnet AND is not the gateway (the .1 Docker reserves).
// Returns a descriptive error when validation fails so the user sees
// exactly what to fix in raioz.yaml.
func ValidateProxyIP(ip, subnet string) error {
	if ip == "" {
		return nil
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("proxy.ip %q is not a valid IPv4 address", ip)
	}

	if subnet == "" {
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

	gateway := dockerGatewayFor(ipnet)
	if gateway != nil && gateway.Equal(parsed) {
		return fmt.Errorf(
			"proxy.ip %s is the Docker gateway for subnet %s — pick something else (e.g. %s.1.1)",
			ip, subnet, gatewayHint(gateway))
	}
	return nil
}

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

func gatewayHint(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d", v4[0], v4[1])
	}
	return ""
}

// nonHTTPImageNames is the curated set of well-known bare image names
// whose servers speak binary protocols (postgres wire, redis RESP, kafka,
// etc.). Routes for these get skipped from the Caddyfile because Caddy
// can't reverse_proxy non-HTTP wire formats; emitting an https:// entry
// would just return 502 forever.
//
// Match is on the bare name (last path segment, before the tag) — see
// IsNonHTTPImage. That avoids false-positives for HTTP UIs that share a
// substring with the binary image they front (redisinsight, pgadmin).
var nonHTTPImageNames = map[string]bool{
	"postgres":    true,
	"postgresql":  true,
	"mariadb":     true,
	"mysql":       true,
	"redis":       true,
	"keydb":       true,
	"dragonfly":   true,
	"memcached":   true,
	"mongo":       true,
	"mongodb":     true,
	"cassandra":   true,
	"scylladb":    true,
	"etcd":        true,
	"rabbitmq":    true,
	"kafka":       true,
	"zookeeper":   true,
	"nats":        true,
	"clickhouse":  true,
	"cockroach":   true,
	"cockroachdb": true,
	"influxdb":    true,
}

// IsNonHTTPImage answers "should this image's container be blocked from
// getting a Caddy reverse_proxy entry?". Examples:
//
//	postgres:16              → "postgres"     → true
//	bitnami/postgresql:15    → "postgresql"   → true
//	redis:7-alpine           → "redis"        → true
//	redis/redisinsight:latest→ "redisinsight" → false (HTTP UI)
//	dpage/pgadmin4:latest    → "pgadmin4"     → false (HTTP UI)
func IsNonHTTPImage(image string) bool {
	return nonHTTPImageNames[bareImageName(image)]
}

// bareImageName extracts the leaf image name from a full image
// reference. Strips digest (@sha256:...), tag (:N), and
// registry/namespace (host/ns/name).
func bareImageName(image string) string {
	if image == "" {
		return ""
	}
	lower := strings.ToLower(image)
	if at := strings.LastIndex(lower, "@"); at >= 0 {
		lower = lower[:at]
	}
	if colon := strings.LastIndex(lower, ":"); colon > strings.LastIndex(lower, "/") {
		lower = lower[:colon]
	}
	if slash := strings.LastIndex(lower, "/"); slash >= 0 {
		lower = lower[slash+1:]
	}
	return lower
}
