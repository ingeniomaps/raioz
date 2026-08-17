package netutil

import (
	"strings"
	"testing"
)

// With proxy.publish:false the user reaches the proxy by IP, so that IP
// has to be predictable. The default is <base>.1.1 — deliberately not
// .0.1, which Docker keeps as the gateway.
func TestDefaultProxyIP(t *testing.T) {
	cases := []struct {
		name   string
		subnet string
		want   string
	}{
		{name: "no subnet means no deterministic IP", subnet: ""},
		{name: "class B", subnet: "172.28.0.0/16", want: "172.28.1.1"},
		{name: "another class B", subnet: "150.150.0.0/16", want: "150.150.1.1"},
		{name: "malformed CIDR", subnet: "not-a-subnet"},
		{name: "missing mask", subnet: "172.28.0.0"},
		{name: "IPv6 is not supported", subnet: "fd00::/64"},
		{
			// A /24 cannot hold x.x.1.1, so there is no default to offer
			// and the caller must fall back to Docker's own assignment.
			name:   "subnet too small for the convention",
			subnet: "192.168.5.0/24",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DefaultProxyIP(tc.subnet); got != tc.want {
				t.Errorf("DefaultProxyIP(%q) = %q, want %q", tc.subnet, got, tc.want)
			}
		})
	}
}

// An unreachable proxy IP is a silent failure at runtime — every URL
// times out — so the config gate has to reject the bad ones up front.
func TestValidateProxyIP(t *testing.T) {
	cases := []struct {
		name     string
		ip       string
		subnet   string
		wantErr  bool
		contains string
	}{
		{name: "no IP declared", ip: "", subnet: "172.28.0.0/16"},
		{name: "valid inside the subnet", ip: "172.28.1.1", subnet: "172.28.0.0/16"},
		{
			name: "IP without a subnet", ip: "172.28.1.1", subnet: "",
			wantErr: true, contains: "network.subnet",
		},
		{
			name: "not an IPv4 address", ip: "banana", subnet: "172.28.0.0/16",
			wantErr: true, contains: "not a valid IPv4",
		},
		{
			name: "IPv6 address", ip: "fd00::1", subnet: "172.28.0.0/16",
			wantErr: true, contains: "not a valid IPv4",
		},
		{
			name: "outside the subnet", ip: "10.0.0.5", subnet: "172.28.0.0/16",
			wantErr: true, contains: "outside",
		},
		{
			name: "malformed subnet", ip: "172.28.1.1", subnet: "172.28.0.0",
			wantErr: true, contains: "valid CIDR",
		},
		{
			// Docker owns .0.1; taking it would collide with the bridge.
			name: "the Docker gateway itself", ip: "172.28.0.1", subnet: "172.28.0.0/16",
			wantErr: true, contains: "gateway",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProxyIP(tc.ip, tc.subnet)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateProxyIP(%q, %q) = nil, want an error", tc.ip, tc.subnet)
				}
				if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
					t.Errorf("error %q should mention %q", err, tc.contains)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateProxyIP(%q, %q) = %v, want nil", tc.ip, tc.subnet, err)
			}
		})
	}
}

// The gateway error names an alternative, and that hint has to be a real
// address in the user's own subnet — a wrong one sends them in circles.
func TestValidateProxyIPGatewayErrorHintsTheSubnet(t *testing.T) {
	err := ValidateProxyIP("150.150.0.1", "150.150.0.0/16")
	if err == nil {
		t.Fatal("expected the gateway to be rejected")
	}
	if !strings.Contains(err.Error(), "150.150.1.1") {
		t.Errorf("hint should point inside the user's subnet, got %q", err)
	}
}

// Caddy cannot reverse_proxy the postgres wire protocol or RESP, so those
// images get no route. The match is on the bare name: an HTTP UI that
// merely shares a substring with a datastore must still get one.
func TestIsNonHTTPImage(t *testing.T) {
	nonHTTP := []string{
		"postgres:16", "bitnami/postgresql:15", "redis:7-alpine",
		"mongo", "kafka:3", "docker.io/library/mysql:8",
	}
	for _, img := range nonHTTP {
		if !IsNonHTTPImage(img) {
			t.Errorf("IsNonHTTPImage(%q) = false, want true", img)
		}
	}

	http := []string{
		"redis/redisinsight:latest", "dpage/pgadmin4:latest",
		"adminer", "nginx:alpine", "", "mongo-express:1.0",
	}
	for _, img := range http {
		if IsNonHTTPImage(img) {
			t.Errorf("IsNonHTTPImage(%q) = true, want false", img)
		}
	}
}
