package mocks

import (
	"context"

	"raioz/internal/domain/interfaces"
)

// Compile-time check
var _ interfaces.ProxyManager = (*MockProxyManager)(nil)

// MockProxyManager is a mock implementation of interfaces.ProxyManager.
type MockProxyManager struct {
	StartFunc               func(ctx context.Context, networkName string) error
	StopFunc                func(ctx context.Context) error
	AddRouteFunc            func(ctx context.Context, route interfaces.ProxyRoute) error
	RemoveRouteFunc         func(ctx context.Context, serviceName string) error
	GetURLFunc              func(serviceName string) string
	ReloadFunc              func(ctx context.Context) error
	StatusFunc              func(ctx context.Context) (bool, error)
	SetDomainFunc           func(domain string)
	SetTLSModeFunc          func(mode string)
	SetBindHostFunc         func(host string)
	SetProjectNameFunc      func(name string)
	SetNetworkSubnetFunc    func(cidr string)
	SetContainerIPFunc      func(ip string)
	SetWorkspaceFunc        func(name string)
	SaveProjectRoutesFunc   func() error
	RemoveProjectRoutesFunc func() error
	RemainingProjectsFunc   func() int
	SetPublishFunc          func(*bool)
	HostsLineFunc           func() string

	// Track calls
	AddedRoutes               []interfaces.ProxyRoute
	StartCalled               bool
	ProjectName               string
	Domain                    string
	TLSMode                   string
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

func (m *MockProxyManager) SetDomain(domain string) {
	m.Domain = domain
	if m.SetDomainFunc != nil {
		m.SetDomainFunc(domain)
	}
}

func (m *MockProxyManager) SetTLSMode(mode string) {
	m.TLSMode = mode
	if m.SetTLSModeFunc != nil {
		m.SetTLSModeFunc(mode)
	}
}

func (m *MockProxyManager) SetBindHost(host string) {
	if m.SetBindHostFunc != nil {
		m.SetBindHostFunc(host)
	}
}

func (m *MockProxyManager) SetProjectName(name string) {
	m.ProjectName = name
	if m.SetProjectNameFunc != nil {
		m.SetProjectNameFunc(name)
	}
}

func (m *MockProxyManager) SetNetworkSubnet(cidr string) {
	m.NetworkSubnet = cidr
	if m.SetNetworkSubnetFunc != nil {
		m.SetNetworkSubnetFunc(cidr)
	}
}

func (m *MockProxyManager) SetContainerIP(ip string) {
	m.ContainerIP = ip
	if m.SetContainerIPFunc != nil {
		m.SetContainerIPFunc(ip)
	}
}

func (m *MockProxyManager) SetWorkspace(name string) {
	m.Workspace = name
	if m.SetWorkspaceFunc != nil {
		m.SetWorkspaceFunc(name)
	}
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

func (m *MockProxyManager) SetPublish(publish *bool) {
	if publish != nil {
		m.Publish = *publish
		m.PublishExplicit = true
	}
	if m.SetPublishFunc != nil {
		m.SetPublishFunc(publish)
	}
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
