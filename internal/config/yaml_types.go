package config

// RaiozConfig represents the new minimal raioz.yaml configuration format.
// This is the user-facing config that gets converted to Deps via the bridge layer.
type RaiozConfig struct {
	Workspace string                    `yaml:"workspace,omitempty"`
	Project   string                    `yaml:"project"`
	Network   *YAMLNetwork              `yaml:"network,omitempty"`
	Proxy     *ProxyConfig              `yaml:"proxy,omitempty"`
	Pre       YAMLStringOrSlice         `yaml:"pre,omitempty"`
	Post      YAMLStringOrSlice         `yaml:"post,omitempty"`
	Services  map[string]YAMLService    `yaml:"services,omitempty"`
	Deps      map[string]YAMLDependency `yaml:"dependencies,omitempty"`
}

// YAMLNetwork lets the user override the Docker network raioz manages for a
// project. Polymorphic in YAML so the common case stays terse:
//
//	network: my-existing-net            # string form: just a name
//	network:                            # object form: name and/or subnet
//	  name: acme-net
//	  subnet: 172.28.0.0/16
//	network:                            # subnet-only: name derived as usual
//	  subnet: 150.150.0.0/16
//
// When omitted, raioz falls back to <workspace>-net or <project>-net and lets
// Docker pick any subnet.
type YAMLNetwork struct {
	Name   string `yaml:"name,omitempty"`
	Subnet string `yaml:"subnet,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for YAMLNetwork so both the
// string shorthand and the object form parse into the same struct.
func (n *YAMLNetwork) UnmarshalYAML(unmarshal func(any) error) error {
	var asString string
	if err := unmarshal(&asString); err == nil && asString != "" {
		n.Name = asString
		return nil
	}

	// Alias avoids infinite recursion back into this UnmarshalYAML.
	type yamlNetworkAlias YAMLNetwork
	var obj yamlNetworkAlias
	if err := unmarshal(&obj); err != nil {
		return err
	}
	*n = YAMLNetwork(obj)
	return nil
}

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

// YAMLService represents a service in the new raioz.yaml format.
type YAMLService struct {
	Path      string          `yaml:"path"`
	DependsOn YAMLStringSlice `yaml:"dependsOn,omitempty"`
	Env       YAMLStringSlice `yaml:"env,omitempty"`
	Ports     YAMLStringSlice `yaml:"ports,omitempty"`

	// Port is the explicit host port the service should listen on. When set,
	// raioz guarantees the service gets exactly this port or `raioz up` fails
	// with a conflict error. When unset, raioz infers a port (.env PORT,
	// runtime default) and, if that collides with another service, picks the
	// next free port deterministically and injects it via $PORT.
	Port int `yaml:"port,omitempty"`

	Watch           YAMLWatch       `yaml:"watch,omitempty"`
	Health          string          `yaml:"health,omitempty"`
	Hostname        string          `yaml:"hostname,omitempty"`
	HostnameAliases YAMLStringSlice `yaml:"hostnameAliases,omitempty"`
	Routing         *RoutingConfig  `yaml:"routing,omitempty"`
	Profiles        YAMLStringSlice `yaml:"profiles,omitempty"`
	Git             string          `yaml:"git,omitempty"`
	Branch          string          `yaml:"branch,omitempty"`

	// Command overrides auto-detection: raioz runs this command verbatim on the
	// host via HostRunner, passing env vars from `env` as process environment.
	// Use it when your project has a non-standard launch script (e.g. `make dev`)
	// that internally does docker compose / build / whatever you need.
	Command string `yaml:"command,omitempty"`

	// Stop is the command to tear down the service. Required when `command`
	// launches background work (e.g. `make start` spawning compose containers)
	// because killing the PID of the parent process won't clean up children.
	// If empty, HostRunner falls back to SIGTERM-then-SIGKILL of the PID.
	Stop string `yaml:"stop,omitempty"`

	// Compose points raioz at one or more existing docker-compose files for
	// this service. Overrides auto-detection. Accepts a single string or a
	// list (merged in order, matching `docker compose -f a -f b`).
	Compose YAMLStringSlice `yaml:"compose,omitempty"`

	// Proxy overrides how the shared HTTPS proxy reaches this service.
	// Normally raioz picks a target from detection (container DNS for Docker
	// services, host.docker.internal for host processes) and a port from
	// `port:` / .env. That heuristic breaks when `command:` launches its
	// own compose stack whose containers raioz can't see (e.g. `make start`
	// spawning hypixo-keycloak on 8080) — raioz classifies the service as
	// "host" and the proxy ends up pointing at host.docker.internal with no
	// port. Setting `proxy:` bypasses the heuristic entirely.
	Proxy *YAMLServiceProxy `yaml:"proxy,omitempty"`
}

// YAMLServiceProxy tells the proxy exactly where to forward traffic for a
// service. Both fields optional; raioz falls back to detection for whichever
// the user leaves out.
type YAMLServiceProxy struct {
	// Target is the DNS name or IP the proxy should reverse_proxy to. Use
	// the container name when the service lives on the shared network
	// (e.g. "hypixo-keycloak"), or a hostname reachable from the proxy
	// network (e.g. "host.docker.internal").
	Target string `yaml:"target,omitempty"`
	// Port is the port to dial on Target.
	Port int `yaml:"port,omitempty"`
}

// YAMLDependency represents a dependency (infrastructure/external service) in raioz.yaml.
type YAMLDependency struct {
	// Name is an optional container-name override. When set, raioz uses this
	// literal string as the Docker container name instead of generating one.
	// Useful when you need the dep to match a name that other tooling (IDEs,
	// backup scripts, external clients) already expects.
	Name string `yaml:"name,omitempty"`

	// Compose points raioz at one or more existing docker-compose files for
	// this dependency. Mutually exclusive with `image:` — use compose when
	// you already maintain a production-grade fragment (healthchecks,
	// volumes, custom entrypoints, multi-container cluster) and want raioz
	// to orchestrate it rather than re-declare with a minimal `image:`.
	//
	// Raioz adds a network+labels overlay so the containers join the
	// workspace network and get swept cleanly on `raioz down`. Env
	// interpolation (${VAR} in your compose) resolves against the files
	// listed in `env:`, which raioz passes as --env-file to docker compose.
	Compose YAMLStringSlice `yaml:"compose,omitempty"`

	Image string `yaml:"image,omitempty"`

	// Ports is the legacy publish list (Docker-compose style). Keeps working
	// for backwards compatibility but emits a deprecation warning at load:
	// prefer `publish:` for scarce host ports and `expose:` for documenting
	// internal container ports. See yaml_bridge.go for the migration path.
	Ports YAMLStringSlice `yaml:"ports,omitempty"`

	// Expose lists the container ports this dependency listens on internally.
	// Purely informational for raioz (not passed to Docker as the separate
	// `expose:` key, which is redundant on a shared network). When combined
	// with `publish: true` it defines which container ports get a host-side
	// binding; without publish it just documents what the image serves so
	// discovery can build correct URLs.
	Expose YAMLIntSlice `yaml:"expose,omitempty"`

	// Publish is the opt-in for host-side exposure of this dependency.
	// Accepts three shapes:
	//   publish: true     → raioz auto-allocates a free host port and maps
	//                        it to the container port (from Expose, or the
	//                        image default). Persisted in state so subsequent
	//                        runs keep the same port.
	//   publish: 5432     → raioz maps host:5432 → container:5432. Fails at
	//                        pre-flight if 5432 is already bound by someone
	//                        else (another project, external tool).
	//   publish: false    → no host binding. The dependency only exists on
	//   (or unset)          the shared Docker network; containers reach it
	//                        by DNS name, host tools cannot see it.
	// For multi-port images (e.g. redis + metrics), pass a list:
	//   publish: [5432, 9090]
	Publish YAMLPublish `yaml:"publish,omitempty"`

	Env             YAMLStringSlice `yaml:"env,omitempty"`
	Volumes         YAMLStringSlice `yaml:"volumes,omitempty"`
	Hostname        string          `yaml:"hostname,omitempty"`
	HostnameAliases YAMLStringSlice `yaml:"hostnameAliases,omitempty"`
	Routing         *RoutingConfig  `yaml:"routing,omitempty"`
	Dev             *YAMLDevConfig  `yaml:"dev,omitempty"`

	// Proxy overrides how the shared HTTPS proxy reaches this dependency.
	// Same semantics as services.<name>.proxy — useful when `compose:`
	// launches a stack whose target container raioz can't infer, or when
	// the image's default port doesn't match what your process actually
	// listens on. Both fields optional; raioz falls back to detection for
	// whichever is left out.
	Proxy *YAMLServiceProxy `yaml:"proxy,omitempty"`
}

// YAMLIntSlice accepts either a single int (`expose: 5432`) or a list
// (`expose: [5432, 9090]`) so the common single-port case stays tidy.
type YAMLIntSlice []int

// UnmarshalYAML implements yaml.Unmarshaler for YAMLIntSlice.
func (s *YAMLIntSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single int
	if err := unmarshal(&single); err == nil {
		*s = []int{single}
		return nil
	}
	var slice []int
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

// YAMLPublish is the polymorphic shape of `publish:`. Zero value means unset
// (internal only). Auto=true means the user asked for automatic allocation
// (bool form). Ports lists specific host ports requested explicitly.
//
// Mutually exclusive: Auto and Ports cannot both be set. The unmarshaller
// enforces this; later code can assume at most one is populated.
type YAMLPublish struct {
	// Set is true when the field was present in YAML (even as `publish: false`).
	// Distinguishes "user said internal-only" from "user didn't say anything".
	// Mostly useful for future semantics; today both mean internal-only.
	Set bool
	// Auto is true when the user wrote `publish: true` — raioz picks host ports.
	Auto bool
	// Ports is the list of explicit host ports the user requested.
	Ports []int
}

// UnmarshalYAML implements yaml.Unmarshaler for YAMLPublish, accepting bool,
// int, or []int so the YAML stays readable in the common cases.
func (p *YAMLPublish) UnmarshalYAML(unmarshal func(any) error) error {
	p.Set = true

	// bool form: publish: true / publish: false
	var b bool
	if err := unmarshal(&b); err == nil {
		p.Auto = b
		return nil
	}

	// single int form: publish: 5432
	var single int
	if err := unmarshal(&single); err == nil {
		p.Ports = []int{single}
		return nil
	}

	// list form: publish: [5432, 9090]
	var slice []int
	if err := unmarshal(&slice); err == nil {
		p.Ports = slice
		return nil
	}

	return nil
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
