package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestYAMLServiceProxy_Unmarshal_Issue068 covers the polymorphic `proxy:`
// field on a service: the boolean shorthand (`false` opts out, `true` keeps
// the default) and the object form ({target, port}).
func TestYAMLServiceProxy_Unmarshal_Issue068(t *testing.T) {
	cases := []struct {
		name         string
		yaml         string
		wantNil      bool
		wantDisabled bool
		wantTarget   string
		wantPort     int
	}{
		{
			name:         "false opts out",
			yaml:         "proxy: false",
			wantDisabled: true,
		},
		{
			name:         "true keeps default",
			yaml:         "proxy: true",
			wantDisabled: false,
		},
		{
			name:       "object form sets target and port",
			yaml:       "proxy:\n  target: hypixo-keycloak\n  port: 8080",
			wantTarget: "hypixo-keycloak",
			wantPort:   8080,
		},
		{
			name:    "absent leaves proxy nil",
			yaml:    "path: ./api",
			wantNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var svc YAMLService
			if err := yaml.Unmarshal([]byte(tc.yaml), &svc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if tc.wantNil {
				if svc.Proxy != nil {
					t.Fatalf("Proxy = %+v, want nil", svc.Proxy)
				}
				return
			}
			if svc.Proxy == nil {
				t.Fatal("Proxy = nil, want non-nil")
			}
			if svc.Proxy.Disabled != tc.wantDisabled {
				t.Errorf("Disabled = %v, want %v", svc.Proxy.Disabled, tc.wantDisabled)
			}
			if svc.Proxy.Target != tc.wantTarget {
				t.Errorf("Target = %q, want %q", svc.Proxy.Target, tc.wantTarget)
			}
			if svc.Proxy.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", svc.Proxy.Port, tc.wantPort)
			}
		})
	}
}

// TestYAMLServiceToService_ProxyDisabled_Issue068 asserts the bridge carries
// the opt-out into models.ServiceProxyOverride.Disabled. The pre-fix guard
// only built the override when Target/Port were set, so `proxy: false` would
// have been silently dropped.
func TestYAMLServiceToService_ProxyDisabled_Issue068(t *testing.T) {
	svc := YAMLService{Path: "./prometheus", Proxy: &YAMLServiceProxy{Disabled: true}}
	out, err := yamlServiceToService("prometheus", svc)
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}
	if out.ProxyOverride == nil {
		t.Fatal("ProxyOverride = nil, want non-nil for proxy:false")
	}
	if !out.ProxyOverride.Disabled {
		t.Error("ProxyOverride.Disabled = false, want true")
	}
}
