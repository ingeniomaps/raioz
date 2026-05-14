package tunnel

import (
	"context"

	"raioz/internal/domain/interfaces"
	tunnelpkg "raioz/internal/tunnel"
)

// Compile-time check
var _ interfaces.TunnelManager = (*ManagerImpl)(nil)

// ManagerImpl adapts internal/tunnel to the TunnelManager port. The
// backend selection logic (cloudflared vs bore) stays inside the
// underlying package; this wrapper exists purely so the app layer can
// program against the port — see ADR-015.
type ManagerImpl struct {
	mgr *tunnelpkg.Manager
}

// NewManager builds a ManagerImpl backed by the default registry at
// ~/.raioz/tunnels.json.
func NewManager() *ManagerImpl {
	return &ManagerImpl{mgr: tunnelpkg.NewManager()}
}

func (m *ManagerImpl) Start(
	ctx context.Context, serviceName string, localPort int,
) (*interfaces.TunnelInfo, error) {
	info, err := m.mgr.Start(ctx, serviceName, localPort)
	if err != nil {
		return nil, err
	}
	return convertInfo(info), nil
}

func (m *ManagerImpl) Stop(_ context.Context, serviceName string) error {
	return m.mgr.Stop(serviceName)
}

func (m *ManagerImpl) StopAll(_ context.Context) error {
	m.mgr.StopAll()
	return nil
}

func (m *ManagerImpl) List(_ context.Context) []interfaces.TunnelInfo {
	src := m.mgr.List()
	out := make([]interfaces.TunnelInfo, 0, len(src))
	for i := range src {
		out = append(out, *convertInfo(&src[i]))
	}
	return out
}

func convertInfo(info *tunnelpkg.Info) *interfaces.TunnelInfo {
	if info == nil {
		return nil
	}
	return &interfaces.TunnelInfo{
		ServiceName: info.ServiceName,
		LocalPort:   info.LocalPort,
		PublicURL:   info.PublicURL,
		Backend:     info.Backend,
		PID:         info.PID,
		StartedAt:   info.StartedAt,
	}
}
