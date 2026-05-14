package mocks

import (
	"context"

	"raioz/internal/domain/interfaces"
)

// Compile-time check
var _ interfaces.ProxyManager = (*MockProxyManager)(nil)

// MockProxyManager is a mock implementation of interfaces.ProxyManager.
// ADR-032: the 8 per-field SetXXX methods are gone; tests inspect the
// captured fields after Configure to verify the surface that used to
// be observed via individual setter calls.
type MockProxyManager struct {
	StartFunc               func(ctx context.Context, networkName string) error
	StopFunc                func(ctx context.Context) error
	AddRouteFunc            func(ctx context.Context, route interfaces.ProxyRoute) error
	RemoveRouteFunc         func(ctx context.Context, serviceName string) error
	GetURLFunc              func(serviceName string) string
	ReloadFunc              func(ctx context.Context) error
	StatusFunc              func(ctx context.Context) (bool, error)
	SaveProjectRoutesFunc   func() error
	RemoveProjectRoutesFunc func() error
	RemainingProjectsFunc   func() int
	HostsLineFunc           func() string
	ConfigureFunc           func(cfg interfaces.ProxyConfig)

	// Captured state — tests assert against these after Configure.
	AddedRoutes               []interfaces.ProxyRoute
	StartCalled               bool
	ProjectName               string
	Domain                    string
	TLSMode                   interfaces.TLSMode
	NetworkSubnet             string
	ContainerIP               string
	Workspace                 string
	SaveProjectRoutesCalled   bool
	RemoveProjectRoutesCalled bool
	Publish                   bool
	PublishExplicit           bool
}

func (m *MockProxyManager) Start(ctx context.Context, networkName string) error {
	m.StartCalled = true
	if m.StartFunc != nil {
		return m.StartFunc(ctx, networkName)
	}
	return nil
}

func (m *MockProxyManager) Stop(ctx context.Context) error {
	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}
	return nil
}

func (m *MockProxyManager) AddRoute(ctx context.Context, route interfaces.ProxyRoute) error {
	m.AddedRoutes = append(m.AddedRoutes, route)
	if m.AddRouteFunc != nil {
		return m.AddRouteFunc(ctx, route)
	}
	return nil
}

func (m *MockProxyManager) RemoveRoute(ctx context.Context, serviceName string) error {
	if m.RemoveRouteFunc != nil {
		return m.RemoveRouteFunc(ctx, serviceName)
	}
	return nil
}

func (m *MockProxyManager) GetURL(serviceName string) string {
	if m.GetURLFunc != nil {
		return m.GetURLFunc(serviceName)
	}
	return "https://" + serviceName + ".localhost"
}

func (m *MockProxyManager) Reload(ctx context.Context) error {
	if m.ReloadFunc != nil {
		return m.ReloadFunc(ctx)
	}
	return nil
}

func (m *MockProxyManager) Status(ctx context.Context) (bool, error) {
	if m.StatusFunc != nil {
		return m.StatusFunc(ctx)
	}
	return false, nil
}

func (m *MockProxyManager) SaveProjectRoutes() error {
	m.SaveProjectRoutesCalled = true
	if m.SaveProjectRoutesFunc != nil {
		return m.SaveProjectRoutesFunc()
	}
	return nil
}

func (m *MockProxyManager) RemoveProjectRoutes() error {
	m.RemoveProjectRoutesCalled = true
	if m.RemoveProjectRoutesFunc != nil {
		return m.RemoveProjectRoutesFunc()
	}
	return nil
}

func (m *MockProxyManager) RemainingProjects() int {
	if m.RemainingProjectsFunc != nil {
		return m.RemainingProjectsFunc()
	}
	return 0
}

func (m *MockProxyManager) IsPublished() bool {
	if !m.PublishExplicit {
		return true // default
	}
	return m.Publish
}

func (m *MockProxyManager) HostsLine() string {
	if m.HostsLineFunc != nil {
		return m.HostsLineFunc()
	}
	return ""
}

func (m *MockProxyManager) Configure(cfg interfaces.ProxyConfig) {
	m.Domain = cfg.Domain
	m.TLSMode = cfg.TLSMode
	m.ProjectName = cfg.ProjectName
	m.Workspace = cfg.Workspace
	m.NetworkSubnet = cfg.NetworkSubnet
	m.ContainerIP = cfg.ContainerIP
	if cfg.Publish != nil {
		m.Publish = *cfg.Publish
		m.PublishExplicit = true
	}
	if m.ConfigureFunc != nil {
		m.ConfigureFunc(cfg)
	}
}
