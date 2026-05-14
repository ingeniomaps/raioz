// Package tunnelcase wraps the tunnel port (ADR-015) in
// use cases so the CLI follows the same wiring as every other
// command. Tests stub the port directly and never shell out to
// cloudflared or bore.
package tunnelcase

import (
	"context"

	"raioz/internal/domain/interfaces"
)

// Dependencies is the narrow set the tunnel use cases need.
type Dependencies struct {
	TunnelManager interfaces.TunnelManager
}

// StartOptions controls Start. Port 0 means "use the default" — the
// CLI today substitutes 3000 to keep behavior unchanged; the use case
// itself stays agnostic so a future caller (TUI, API) can pick its
// own default.
type StartOptions struct {
	ServiceName string
	LocalPort   int
}

// StartUseCase brings up a tunnel for a single service.
type StartUseCase struct {
	Deps *Dependencies
}

func (uc *StartUseCase) Execute(ctx context.Context, opts StartOptions) (*interfaces.TunnelInfo, error) {
	return uc.Deps.TunnelManager.Start(ctx, opts.ServiceName, opts.LocalPort)
}

// StopOptions controls Stop.
type StopOptions struct {
	ServiceName string
}

// StopUseCase kills the tunnel for one service.
type StopUseCase struct {
	Deps *Dependencies
}

func (uc *StopUseCase) Execute(ctx context.Context, opts StopOptions) error {
	return uc.Deps.TunnelManager.Stop(ctx, opts.ServiceName)
}

// StopAllUseCase kills every tunnel raioz is tracking.
type StopAllUseCase struct {
	Deps *Dependencies
}

func (uc *StopAllUseCase) Execute(ctx context.Context) error {
	return uc.Deps.TunnelManager.StopAll(ctx)
}

// ListUseCase returns the currently-live tunnels.
type ListUseCase struct {
	Deps *Dependencies
}

func (uc *ListUseCase) Execute(ctx context.Context) []interfaces.TunnelInfo {
	return uc.Deps.TunnelManager.List(ctx)
}
