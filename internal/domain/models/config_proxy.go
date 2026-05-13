package models

// ProxyConfig can be a simple bool or a detailed object.
type ProxyConfig struct {
	Enabled bool   `yaml:"-"`
	Mode    string `yaml:"mode,omitempty"`   // "subdomain" (default) | "path"
	Domain  string `yaml:"domain,omitempty"` // custom domain (default: "localhost")
	TLS     string `yaml:"tls,omitempty"`    // "mkcert" (default) | "letsencrypt"

	// IP pins the proxy container to a specific address inside the Docker
	// network. Useful for scripts / /etc/hosts entries that need a stable
	// IP to reach the proxy. When empty AND network.subnet is set, raioz
	// defaults to <subnet-base>.1.1 — a memorable, reserved-slot address
	// that stays free across reinstalls. Requires network.subnet to be set
	// (Docker won't honor --ip without a user-defined subnet).
	IP string `yaml:"ip,omitempty"`

	// Publish controls whether the proxy binds host ports 80/443. Default
	// (nil/true) keeps the legacy behavior — accessible from the host via
	// localhost. Set to false to skip the host binding entirely; the proxy
	// is then only reachable via its container IP inside the Docker
	// network. Useful for running multiple workspaces in parallel without
	// fighting over 80/443 — each gets its own subnet + IP and you map
	// them via /etc/hosts.
	//
	// Requires a deterministic IP (network.subnet or proxy.ip) so the user
	// knows what to put in /etc/hosts. Linux only: macOS and Windows route
	// Docker traffic through a VM whose bridge IPs aren't reachable from
	// the host, so publish:false is functionally broken there.
	Publish *bool `yaml:"publish,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for ProxyConfig to support both bool and object.
func (p *ProxyConfig) UnmarshalYAML(unmarshal func(any) error) error {
	var b bool
	if err := unmarshal(&b); err == nil {
		p.Enabled = b
		return nil
	}

	type proxyAlias ProxyConfig
	var obj proxyAlias
	if err := unmarshal(&obj); err != nil {
		return err
	}
	*p = ProxyConfig(obj)
	p.Enabled = true
	return nil
}

// RoutingConfig defines proxy routing behavior for a service.
type RoutingConfig struct {
	WS     bool `yaml:"ws,omitempty" json:"ws,omitempty"`
	Stream bool `yaml:"stream,omitempty" json:"stream,omitempty"`
	GRPC   bool `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	Tunnel bool `yaml:"tunnel,omitempty" json:"tunnel,omitempty"`
}
