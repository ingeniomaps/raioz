package interfaces

import (
	"context"
)

// ProxyRoute defines how traffic reaches a service through the proxy.
type ProxyRoute struct {
	ServiceName   string
	Hostname      string // e.g., "api" → "api.localhost"
	Target        string // e.g., "api:3000" (container) or "host.docker.internal:3001" (host)
	Port          int
	WebSocket     bool
	Stream        bool
	GRPC          bool
}

// ProxyManager defines operations for managing the reverse proxy (Caddy).
type ProxyManager interface {
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
}
