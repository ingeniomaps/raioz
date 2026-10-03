package proxy

import (
	"context"
	"testing"
	"time"
)

func TestPublishHostAndListenHost(t *testing.T) {
	tests := []struct {
		name        string
		publish     bool
		bindHost    string
		containerIP string
		wantHost    string
		wantAddr    string
	}{
		{"published defaults to loopback", true, "", "10.0.1.1", "127.0.0.1", "127.0.0.1"},
		{"explicit host is honored", true, "192.168.1.5", "", "192.168.1.5", "192.168.1.5"},
		{"all interfaces is probed on loopback", true, "0.0.0.0", "", "0.0.0.0", "127.0.0.1"},
		{"unpublished is reached on the container", false, "", "10.0.1.1", "127.0.0.1", "10.0.1.1"},
		{"unpublished without an IP has no address", false, "", "", "127.0.0.1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager("")
			m.publish, m.bindHost = tt.publish, tt.bindHost
			if got := m.publishHost(); got != tt.wantHost {
				t.Errorf("publishHost = %q, want %q", got, tt.wantHost)
			}
			if got := m.listenHost(tt.containerIP); got != tt.wantAddr {
				t.Errorf("listenHost = %q, want %q", got, tt.wantAddr)
			}
		})
	}
}

func TestWaitUntilListening_ReturnsOnceReachable(t *testing.T) {
	prev := proxyAnswers
	t.Cleanup(func() { proxyAnswers = prev })

	calls := 0
	proxyAnswers = func(context.Context, string) bool {
		calls++
		return calls >= 3
	}
	m := NewManager("")
	m.publish = true

	start := time.Now()
	m.waitUntilListening(context.Background(), "")
	if calls != 3 {
		t.Errorf("dial attempts = %d, want 3 (two refusals, then accepted)", calls)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("returned after %v; it must stop as soon as the proxy answers", time.Since(start))
	}
}
