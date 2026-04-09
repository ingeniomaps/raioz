package config

// RaiozConfig represents the new minimal raioz.yaml configuration format.
// This is the user-facing config that gets converted to Deps via the bridge layer.
type RaiozConfig struct {
	Workspace string                    `yaml:"workspace,omitempty"`
	Project   string                    `yaml:"project"`
	Proxy     *ProxyConfig              `yaml:"proxy,omitempty"`
	Pre       YAMLStringOrSlice         `yaml:"pre,omitempty"`
	Post      YAMLStringOrSlice         `yaml:"post,omitempty"`
	Services  map[string]YAMLService    `yaml:"services,omitempty"`
	Deps      map[string]YAMLDependency `yaml:"dependencies,omitempty"`
}

// ProxyConfig can be a simple bool or a detailed object.
type ProxyConfig struct {
	Enabled bool   `yaml:"-"`
	Mode    string `yaml:"mode,omitempty"`    // "subdomain" (default) | "path"
	Domain  string `yaml:"domain,omitempty"`  // custom domain (default: "localhost")
	TLS     string `yaml:"tls,omitempty"`     // "mkcert" (default) | "letsencrypt"
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

// YAMLService represents a service in the new raioz.yaml format.
type YAMLService struct {
	Path      string          `yaml:"path"`
	DependsOn YAMLStringSlice `yaml:"dependsOn,omitempty"`
	Env       YAMLStringSlice `yaml:"env,omitempty"`
	Ports     YAMLStringSlice `yaml:"ports,omitempty"`
	Watch     YAMLWatch       `yaml:"watch,omitempty"`
	Health    string          `yaml:"health,omitempty"`
	Hostname  string          `yaml:"hostname,omitempty"`
	Routing   *RoutingConfig  `yaml:"routing,omitempty"`
	Profiles  YAMLStringSlice `yaml:"profiles,omitempty"`
	Git       string          `yaml:"git,omitempty"`
	Branch    string          `yaml:"branch,omitempty"`
}

// YAMLDependency represents a dependency (infrastructure/external service) in raioz.yaml.
type YAMLDependency struct {
	Image    string          `yaml:"image"`
	Ports    YAMLStringSlice `yaml:"ports,omitempty"`
	Env      YAMLStringSlice `yaml:"env,omitempty"`
	Volumes  YAMLStringSlice `yaml:"volumes,omitempty"`
	Hostname string          `yaml:"hostname,omitempty"`
	Routing  *RoutingConfig  `yaml:"routing,omitempty"`
	Dev      *YAMLDevConfig  `yaml:"dev,omitempty"`
}

// YAMLDevConfig allows a dependency to specify a local path for development override.
type YAMLDevConfig struct {
	Path string `yaml:"path"`
}

// RoutingConfig defines proxy routing behavior for a service.
type RoutingConfig struct {
	WS     bool `yaml:"ws,omitempty" json:"ws,omitempty"`
	Stream bool `yaml:"stream,omitempty" json:"stream,omitempty"`
	GRPC   bool `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	Tunnel bool `yaml:"tunnel,omitempty" json:"tunnel,omitempty"`
}

// YAMLWatch can be a bool (true/false) or a string ("native").
type YAMLWatch struct {
	Enabled bool
	Mode    string // "" (raioz watches), "native" (service has its own hot-reload)
}

// UnmarshalYAML implements yaml.Unmarshaler for YAMLWatch.
func (w *YAMLWatch) UnmarshalYAML(unmarshal func(any) error) error {
	var b bool
	if err := unmarshal(&b); err == nil {
		w.Enabled = b
		w.Mode = ""
		return nil
	}

	var s string
	if err := unmarshal(&s); err == nil {
		w.Enabled = true
		w.Mode = s
		return nil
	}

	return nil
}

// YAMLStringSlice is a helper type that allows a YAML field to be either a single string or a list.
type YAMLStringSlice []string

// UnmarshalYAML implements yaml.Unmarshaler for YAMLStringSlice.
func (s *YAMLStringSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}

	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

// YAMLStringOrSlice is a helper that allows pre/post hooks to be a single string or a list of commands.
type YAMLStringOrSlice []string

// UnmarshalYAML implements yaml.Unmarshaler for YAMLStringOrSlice.
func (s *YAMLStringOrSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}

	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}
