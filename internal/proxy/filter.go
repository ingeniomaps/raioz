package proxy

import "strings"

// nonHTTPImageNames is the curated set of well-known bare image names whose
// servers speak binary protocols (postgres wire, redis RESP, kafka, etc.).
// Routes for these get skipped from the Caddyfile because Caddy can't
// reverse_proxy non-HTTP wire formats; emitting an https:// entry would
// just return 502 forever.
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

// bareImageName extracts the leaf image name from a full image reference.
// Strips digest (@sha256:...), tag (:N), and registry/namespace (host/ns/name).
func bareImageName(image string) string {
	if image == "" {
		return ""
	}
	lower := strings.ToLower(image)
	if at := strings.LastIndex(lower, "@"); at >= 0 {
		lower = lower[:at]
	}
	// A ":" after the last "/" is the tag separator (a ":" before the last
	// "/" would be a registry port, e.g. "registry.local:5000/...").
	if colon := strings.LastIndex(lower, ":"); colon > strings.LastIndex(lower, "/") {
		lower = lower[:colon]
	}
	if slash := strings.LastIndex(lower, "/"); slash >= 0 {
		lower = lower[slash+1:]
	}
	return lower
}
