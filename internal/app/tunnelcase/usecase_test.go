package tunnelcase

import (
	"context"
	"errors"
	"testing"

	"raioz/internal/domain/interfaces"
)

type mockTunnelManager struct {
	startCalls    int
	startInfo     *interfaces.TunnelInfo
	startErr      error
	stopCalled    bool
	stopErr       error
	stopAllCalled bool
	stopAllErr    error
	listResult    []interfaces.TunnelInfo
}

func (m *mockTunnelManager) Start(
	_ context.Context, serviceName string, localPort int,
) (*interfaces.TunnelInfo, error) {
	m.startCalls++
	if m.startErr != nil {
		return nil, m.startErr
	}
	if m.startInfo != nil {
		return m.startInfo, nil
	}
	return &interfaces.TunnelInfo{
		ServiceName: serviceName,
		LocalPort:   localPort,
		PublicURL:   "https://example.test",
		PID:         42,
	}, nil
}

func (m *mockTunnelManager) Stop(_ context.Context, _ string) error {
	m.stopCalled = true
	return m.stopErr
}

func (m *mockTunnelManager) StopAll(_ context.Context) error {
	m.stopAllCalled = true
	return m.stopAllErr
}

func (m *mockTunnelManager) List(_ context.Context) []interfaces.TunnelInfo {
	return m.listResult
}

func TestStart_DelegatesToManager(t *testing.T) {
	mgr := &mockTunnelManager{}
	uc := StartUseCase{Deps: &Dependencies{TunnelManager: mgr}}
	info, err := uc.Execute(context.Background(), StartOptions{ServiceName: "api", LocalPort: 8080})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.ServiceName != "api" {
		t.Errorf("expected info for api, got %+v", info)
	}
	if mgr.startCalls != 1 {
		t.Errorf("expected one Start call, got %d", mgr.startCalls)
	}
}

func TestStart_PropagatesError(t *testing.T) {
	mgr := &mockTunnelManager{startErr: errors.New("no backend")}
	uc := StartUseCase{Deps: &Dependencies{TunnelManager: mgr}}
	_, err := uc.Execute(context.Background(), StartOptions{ServiceName: "api"})
	if err == nil {
		t.Fatal("expected error from manager")
	}
}

func TestStop_DelegatesToManager(t *testing.T) {
	mgr := &mockTunnelManager{}
	uc := StopUseCase{Deps: &Dependencies{TunnelManager: mgr}}
	if err := uc.Execute(context.Background(), StopOptions{ServiceName: "api"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mgr.stopCalled {
		t.Error("expected Stop to be invoked")
	}
}

func TestStopAll_DelegatesToManager(t *testing.T) {
	mgr := &mockTunnelManager{}
	uc := StopAllUseCase{Deps: &Dependencies{TunnelManager: mgr}}
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mgr.stopAllCalled {
		t.Error("expected StopAll to be invoked")
	}
}

func TestList_ReturnsManagerResult(t *testing.T) {
	mgr := &mockTunnelManager{
		listResult: []interfaces.TunnelInfo{{ServiceName: "api"}, {ServiceName: "web"}},
	}
	uc := ListUseCase{Deps: &Dependencies{TunnelManager: mgr}}
	got := uc.Execute(context.Background())
	if len(got) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(got))
	}
}
