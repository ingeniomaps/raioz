package interfaces

import (
	"context"
	"time"
)

// TunnelInfo describes a live tunnel raioz is keeping alive on behalf
// of a service. Lives in the domain layer so callers don't have to
// import internal/tunnel.
type TunnelInfo struct {
	ServiceName string
	LocalPort   int
	PublicURL   string
	Backend     string // "cloudflared" or "bore" today
	PID         int
	StartedAt   time.Time
}

// TunnelManager is the port covering tunnel lifecycle. Introduced in
// ADR-015 so the `raioz tunnel` CLI runs through the
// use-case layer like every other command. Backend selection
// (cloudflared vs bore vs future frp/ngrok) is the adapter's concern.
type TunnelManager interface {
	// Start brings up a tunnel for serviceName pointing at localPort.
	// The adapter picks the backend based on which binary is on
	// PATH; callers don't choose.
	Start(ctx context.Context, serviceName string, localPort int) (*TunnelInfo, error)

	// Stop kills the tunnel process for serviceName and drops the
	// registry entry. Returns an error when no tunnel is active for
	// the given service.
	Stop(ctx context.Context, serviceName string) error

	// StopAll kills every tunnel raioz is tracking and clears the
	// registry. Best-effort: failure to kill an individual PID is
	// logged but doesn't abort.
	StopAll(ctx context.Context) error

	// List returns every tunnel raioz believes is alive, sweeping
	// dead PIDs as a side effect.
	List(ctx context.Context) []TunnelInfo
}
