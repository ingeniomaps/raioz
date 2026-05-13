package interfaces

import (
	"context"
)

// ProxyRoute defines how traffic reaches a service through the proxy.
type ProxyRoute struct {
	ServiceName string
	Hostname    string // e.g., "api" → "api.localhost"
	// Aliases lists extra subdomains that must resolve to the same
	// upstream. Each alias becomes a sibling hostname in the Caddy site
	// block and an additional Docker `--network-alias` so containers can
	// reach the service by any of them. Empty means "no aliases".
	Aliases   []string
	Target    string // e.g., "api:3000" (container) or "host.docker.internal:3001" (host)
	Port      int
	WebSocket bool
	Stream    bool
	GRPC      bool
}

// ProxyConfig captures every option that used to be applied to a
// ProxyManager through individual setters (SetDomain, SetTLSMode,
// SetPublish, …). Building the value once and passing it to Configure
// replaces the "remember to call all eight setters" orchestration that
// pre-dated ADR-013.
//
// Zero-value ProxyConfig is valid: it means "defaults" (localhost
// domain, mkcert TLS, host publish enabled). Each field's zero-value
// semantics are documented inline.
type ProxyConfig struct {
	Domain        string // empty → "localhost"
	TLSMode       string // empty → "mkcert"; accepts "mkcert" | "letsencrypt"
	BindHost      string // empty → manager default (127.0.0.1)
	ProjectName   string // empty → no project scope (one-shot CLI ops)
	Workspace     string // non-empty → workspace-shared mode
	NetworkSubnet string // CIDR; empty → Docker auto-assigns
	ContainerIP   string // empty + NetworkSubnet → derive <base>.1.1
	Publish       *bool  // nil → default (true); *false → no host binding
}

// ProxyManager defines operations for managing the reverse proxy (Caddy).
type ProxyManager interface {
	// Configure applies a ProxyConfig to the manager in one call.
	// Canonical entry point for configuration (ADR-013); the per-field
	// setters below remain for backward compatibility but are
	// deprecated.
	Configure(cfg ProxyConfig)
	// Start starts the proxy container on the given network
	Start(ctx context.Context, networkName string) error
	// Stop stops the proxy container
	Stop(ctx context.Context) error
	// AddRoute adds or updates a route for a service
	AddRoute(ctx context.Context, route ProxyRoute) error
	// RemoveRoute removes a route for a service
	RemoveRoute(ctx context.Context, serviceName string) error
	// GetURL returns the HTTPS URL for a service (e.g., "https://api.localhost")
	GetURL(serviceName string) string
	// Reload regenerates the proxy config and applies it without downtime
	Reload(ctx context.Context) error
	// Status returns whether the proxy is running
	Status(ctx context.Context) (bool, error)
	// SetDomain sets a custom domain (e.g., "acme.localhost")
	SetDomain(domain string)
	// SetTLSMode sets TLS: "mkcert" (local) or "letsencrypt" (production)
	SetTLSMode(mode string)
	// SetBindHost sets the bind address (e.g., "0.0.0.0")
	SetBindHost(host string)
	// SetProjectName sets the project name for container/volume naming
	SetProjectName(name string)
	// SetNetworkSubnet records the CIDR of the Docker network the proxy
	// will attach to. Used to derive the default proxy IP and to validate
	// any user-declared `proxy.ip:` against the subnet range.
	SetNetworkSubnet(cidr string)
	// SetContainerIP pins the proxy container to a specific address inside
	// the Docker network. Empty string means "let raioz pick the
	// convention (<subnet>.1.1) when subnet is set, else auto-assign".
	SetContainerIP(ip string)
	// SetWorkspace switches the proxy to workspace-shared mode (a single
	// Caddy per workspace fronting every project) when non-empty. Empty
	// reverts to per-project mode.
	SetWorkspace(name string)
	// SaveProjectRoutes persists this project's currently-known routes to
	// the workspace's shared routes directory so the next Caddyfile
	// generation includes them. No-op in per-project mode.
	SaveProjectRoutes() error
	// RemoveProjectRoutes deletes this project's persisted routes file.
	// Idempotent. No-op in per-project mode.
	RemoveProjectRoutes() error
	// RemainingProjects returns the number of persisted project routes
	// files still in the workspace. Used by the down flow to decide
	// between Reload (siblings remain) and Stop (last one out).
	RemainingProjects() int
	// SetPublish toggles whether the proxy binds host ports 80/443. nil
	// or true keeps the legacy host-published behavior; false skips the
	// binding so the proxy is reachable only via its container IP.
	SetPublish(publish *bool)
	// IsPublished reports the current publish flag (true when host ports
	// are bound).
	IsPublished() bool
	// HostsLine returns an /etc/hosts-style line mapping the proxy's
	// container IP to every route hostname. Returns "" when the IP
	// can't be resolved or there are no routes.
	HostsLine() string
}
