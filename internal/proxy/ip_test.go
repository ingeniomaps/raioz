package proxy

import (
	"strings"
	"testing"
)

func TestDefaultProxyIP_Convention(t *testing.T) {
	cases := []struct {
		subnet string
		want   string
	}{
		{"172.28.0.0/16", "172.28.1.1"},
		{"10.3.0.0/16", "10.3.1.1"},
		{"192.168.0.0/16", "192.168.1.1"},
		{"10.3.0.0/20", "10.3.1.1"}, // /20 still contains .1.1
	}
	for _, tc := range cases {
		t.Run(tc.subnet, func(t *testing.T) {
			if got := DefaultProxyIP(tc.subnet); got != tc.want {
				t.Errorf("DefaultProxyIP(%q) = %q, want %q", tc.subnet, got, tc.want)
			}
		})
	}
}

func TestDefaultProxyIP_Empty(t *testing.T) {
	if DefaultProxyIP("") != "" {
		t.Error("empty subnet must return empty (lets caller auto-assign)")
	}
}

func TestDefaultProxyIP_Malformed(t *testing.T) {
	if got := DefaultProxyIP("not a subnet"); got != "" {
		t.Errorf("malformed subnet must return empty, got %q", got)
	}
}

func TestDefaultProxyIP_TooSmallSubnet(t *testing.T) {
	// /24 only has .0–.255 in the last octet — .1.1 is outside.
	if got := DefaultProxyIP("172.28.0.0/24"); got != "" {
		t.Errorf("/24 cannot contain .1.1, expected empty fallback, got %q", got)
	}
}

func TestValidateProxyIP_Valid(t *testing.T) {
	if err := ValidateProxyIP("172.28.5.10", "172.28.0.0/16"); err != nil {
		t.Errorf("valid IP inside subnet must pass, got %v", err)
	}
}

func TestValidateProxyIP_OutsideSubnet(t *testing.T) {
	err := ValidateProxyIP("10.0.0.5", "172.28.0.0/16")
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("expected outside-subnet error, got %v", err)
	}
}

func TestValidateProxyIP_GatewayCollision(t *testing.T) {
	// 172.28.0.1 is the Docker gateway for 172.28.0.0/16.
	err := ValidateProxyIP("172.28.0.1", "172.28.0.0/16")
	if err == nil || !strings.Contains(err.Error(), "gateway") {
		t.Errorf("gateway IP must be rejected, got %v", err)
	}
}

func TestValidateProxyIP_RequiresSubnet(t *testing.T) {
	err := ValidateProxyIP("10.5.5.5", "")
	if err == nil || !strings.Contains(err.Error(), "subnet") {
		t.Errorf("explicit IP without subnet must error, got %v", err)
	}
}

func TestValidateProxyIP_MalformedIP(t *testing.T) {
	err := ValidateProxyIP("not-an-ip", "172.28.0.0/16")
	if err == nil || !strings.Contains(err.Error(), "valid IPv4") {
		t.Errorf("malformed IP must be rejected, got %v", err)
	}
}

func TestValidateProxyIP_Empty(t *testing.T) {
	if err := ValidateProxyIP("", "172.28.0.0/16"); err != nil {
		t.Error("empty IP is the 'use default' sentinel — must not error")
	}
}

func TestManager_ResolveContainerIP_ExplicitWins(t *testing.T) {
	m := NewManager("")
	m.SetNetworkSubnet("172.28.0.0/16")
	m.SetContainerIP("172.28.5.5")

	got, err := m.resolveContainerIP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "172.28.5.5" {
		t.Errorf("explicit IP must be honored verbatim, got %q", got)
	}
}

func TestManager_ResolveContainerIP_DefaultFromSubnet(t *testing.T) {
	m := NewManager("")
	m.SetNetworkSubnet("10.3.0.0/16")
	// no SetContainerIP

	got, err := m.resolveContainerIP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "10.3.1.1" {
		t.Errorf("default must follow <base>.1.1 convention, got %q", got)
	}
}

func TestManager_ResolveContainerIP_NoSubnetNoIP(t *testing.T) {
	m := NewManager("")
	got, err := m.resolveContainerIP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("no subnet + no IP must yield empty (Docker auto-assign), got %q", got)
	}
}

func TestManager_ResolveContainerIP_InvalidExplicit(t *testing.T) {
	m := NewManager("")
	m.SetNetworkSubnet("172.28.0.0/16")
	m.SetContainerIP("10.0.0.5") // outside subnet

	if _, err := m.resolveContainerIP(); err == nil {
		t.Error("invalid explicit IP must propagate the validation error")
	}
}
