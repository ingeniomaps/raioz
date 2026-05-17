package config

import "raioz/internal/domain/models"

// ProxyConfig lives canonically in internal/domain/models; the alias keeps
// `models.ProxyConfig` callers compiling (see ADR-009).
type ProxyConfig = models.ProxyConfig

// CurrentSchemaVersion is the version stamp raioz writes into newly
// generated raioz.yaml files (init, migrate). Existing configs without
// a version: field still load — a warning is surfaced via
// LoadDepsFromYAML. See ADR / docs/CONFIG_REFERENCE.md#versioning.
const CurrentSchemaVersion = "1"

// RaiozConfig represents the new minimal raioz.yaml configuration format.
// This is the user-facing config that gets converted to Deps via the bridge layer.
type RaiozConfig struct {
	// Version declares which raioz.yaml schema this file targets. Optional
	// today; the loader emits a warning when absent. Future releases may
	// require it. Currently the only valid value is "1".
	Version   string                    `yaml:"version,omitempty"`      // since: v0.5.0
	Workspace string                    `yaml:"workspace,omitempty"`    // since: v0.1.0
	Project   string                    `yaml:"project"`                // since: v0.1.0
	Network   *YAMLNetwork              `yaml:"network,omitempty"`      // since: v0.1.0
	Proxy     *ProxyConfig              `yaml:"proxy,omitempty"`        // since: v0.1.0
	Pre       YAMLStringOrSlice         `yaml:"pre,omitempty"`          // since: v0.1.0
	PreUp     YAMLStringOrSlice         `yaml:"preUp,omitempty"`        // since: v0.5.0 — ADR-024
	Post      YAMLStringOrSlice         `yaml:"post,omitempty"`         // since: v0.1.0
	Services  map[string]YAMLService    `yaml:"services,omitempty"`     // since: v0.1.0
	Deps      map[string]YAMLDependency `yaml:"dependencies,omitempty"` // since: v0.1.0

	// Router replaces raioz's internal Caddy with a sibling raioz project
	// acting as the workspace's edge router. When set, raioz brings the
	// router project up first, polls its `health:`, then starts consumers;
	// the bundled Caddy is skipped entirely. The `--router-off` flag on
	// `raioz up` bypasses this for a single invocation. See ADR-037.
	Router *YAMLRouter `yaml:"router,omitempty"` // since: v0.8.0

	// Kind discriminates the config shape. Empty / "project" (default) means
	// the regular shape with services/dependencies. "meta" means this file
	// is a meta-orchestrator that delegates to sub-projects.
	Kind string `yaml:"kind,omitempty"` // since: v0.4.0
	// Projects is the list of sub-projects this meta config orchestrates.
	// Each path is resolved relative to the meta raioz.yaml. Used only when
	// Kind == "meta".
	Projects []YAMLMetaProject `yaml:"projects,omitempty"` // since: v0.4.0
	// StartOrder optionally pins the order in which sub-projects are brought
	// up. Each entry must match a `path:` from `projects:`. Down runs in
	// reverse. When omitted, the order of `projects:` is used.
	StartOrder []string `yaml:"startOrder,omitempty"` // since: v0.4.0
}

// YAMLMetaProject is one entry in a meta-orchestrator config's `projects:`
// list. The path is relative to the meta raioz.yaml file.
type YAMLMetaProject struct {
	Path string `yaml:"path"` // since: v0.4.0
	// Optional sub-projects don't abort the meta `up` on failure — the meta
	// run logs a warning and continues. Useful for repos that aren't always
	// checked out (ad-service, internal tools, work-in-progress migrations).
	Optional bool `yaml:"optional,omitempty"` // since: v0.4.0
	// Profiles are opt-in tags. A project with non-empty Profiles is
	// skipped from `raioz up` / `raioz status` unless the user passes
	// `--meta-profile <name>` matching one of them. Empty Profiles means
	// always-on. `raioz down` ignores Profiles and tears down whatever is
	// declared so the user can't strand a sub-project that was started
	// with a different profile set.
	Profiles YAMLStringSlice `yaml:"profiles,omitempty"` // since: v0.6.0
}

// YAMLRouter replaces the bundled Caddy with a sibling raioz project. V1
// carries only the project path; the router project owns its own routing
// templates (no service-discovery contract from raioz to the router). See
// ADR-037 for the rationale.
type YAMLRouter struct {
	// Project is the relative path (from this raioz.yaml) to the sibling
	// raioz project that should serve as the workspace's edge router. The
	// target directory must contain a raioz.yaml. The same path MAY also
	// appear under `projects:` in a meta config — `router:` just upgrades
	// it from "another sub-project" to "first up, last down".
	Project string `yaml:"project"` // since: v0.8.0
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
	Name   string `yaml:"name,omitempty"`   // since: v0.1.0
	Subnet string `yaml:"subnet,omitempty"` // since: v0.1.0
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

// YAMLService represents a service in the new raioz.yaml format.
type YAMLService struct {
	Path      string          `yaml:"path"`                // since: v0.1.0
	DependsOn YAMLStringSlice `yaml:"dependsOn,omitempty"` // since: v0.1.0
	Env       YAMLStringSlice `yaml:"env,omitempty"`       // since: v0.1.0
	Ports     YAMLStringSlice `yaml:"ports,omitempty"`     // since: v0.1.0

	// Port is the explicit host port the service should listen on. When set,
	// raioz guarantees the service gets exactly this port or `raioz up` fails
	// with a conflict error. When unset, raioz infers a port (.env PORT,
	// runtime default) and, if that collides with another service, picks the
	// next free port deterministically and injects it via $PORT.
	Port int `yaml:"port,omitempty"` // since: v0.1.0

	Watch           YAMLWatch       `yaml:"watch,omitempty"`           // since: v0.1.0
	Health          string          `yaml:"health,omitempty"`          // since: v0.1.0
	Hostname        string          `yaml:"hostname,omitempty"`        // since: v0.1.0
	HostnameAliases YAMLStringSlice `yaml:"hostnameAliases,omitempty"` // since: v0.3.0
	Routing         *RoutingConfig  `yaml:"routing,omitempty"`         // since: v0.1.0
	Profiles        YAMLStringSlice `yaml:"profiles,omitempty"`        // since: v0.1.0
	Git             string          `yaml:"git,omitempty"`             // since: v0.1.0
	Branch          string          `yaml:"branch,omitempty"`          // since: v0.1.0
	Auth            string          `yaml:"auth,omitempty"`            // since: v0.7.0

	// Command overrides auto-detection: raioz runs this command verbatim on the
	// host via HostRunner, passing env vars from `env` as process environment.
	// Use it when your project has a non-standard launch script (e.g. `make dev`)
	// that internally does docker compose / build / whatever you need.
	Command string `yaml:"command,omitempty"` // since: v0.1.0

	// Stop is the command to tear down the service. Required when `command`
	// launches background work (e.g. `make start` spawning compose containers)
	// because killing the PID of the parent process won't clean up children.
	// If empty, HostRunner falls back to SIGTERM-then-SIGKILL of the PID.
	Stop string `yaml:"stop,omitempty"` // since: v0.1.0

	// Compose points raioz at one or more existing docker-compose files for
	// this service. Overrides auto-detection. Accepts a single string or a
	// list (merged in order, matching `docker compose -f a -f b`).
	Compose YAMLStringSlice `yaml:"compose,omitempty"` // since: v0.1.0

	// Runtime forces the runtime classification for this service,
	// overriding filesystem auto-detection. Useful when a service has
	// multiple manifests (Go service with a prod Dockerfile, Python
	// service with a docker-compose.yml). The value must be one of
	// the runtimes raioz knows (`raioz doctor` lists them).
	Runtime string `yaml:"runtime,omitempty"` // since: v0.9.0

	// Proxy overrides how the shared HTTPS proxy reaches this service.
	// Normally raioz picks a target from detection (container DNS for Docker
	// services, host.docker.internal for host processes) and a port from
	// `port:` / .env. That heuristic breaks when `command:` launches its
	// own compose stack whose containers raioz can't see (e.g. `make start`
	// spawning hypixo-keycloak on 8080) — raioz classifies the service as
	// "host" and the proxy ends up pointing at host.docker.internal with no
	// port. Setting `proxy:` bypasses the heuristic entirely.
	Proxy *YAMLServiceProxy `yaml:"proxy,omitempty"` // since: v0.1.0
}

// YAMLServiceProxy tells the proxy exactly where to forward traffic for a
// service. Both fields optional; raioz falls back to detection for whichever
// the user leaves out.
type YAMLServiceProxy struct {
	// Target is the DNS name or IP the proxy should reverse_proxy to. Use
	// the container name when the service lives on the shared network
	// (e.g. "hypixo-keycloak"), or a hostname reachable from the proxy
	// network (e.g. "host.docker.internal").
	Target string `yaml:"target,omitempty"` // since: v0.1.0
	// Port is the port to dial on Target.
	Port int `yaml:"port,omitempty"` // since: v0.1.0
}

// YAMLDependency represents a dependency (infrastructure/external service) in raioz.yaml.
type YAMLDependency struct {
	// Name is an optional container-name override. When set, raioz uses this
	// literal string as the Docker container name instead of generating one.
	// Useful when you need the dep to match a name that other tooling (IDEs,
	// backup scripts, external clients) already expects.
	Name string `yaml:"name,omitempty"` // since: v0.1.0

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
	Compose YAMLStringSlice `yaml:"compose,omitempty"` // since: v0.1.0

	Image string `yaml:"image,omitempty"` // since: v0.1.0

	// Ports is the legacy publish list (Docker-compose style). Keeps working
	// for backwards compatibility but emits a deprecation warning at load:
	// prefer `publish:` for scarce host ports and `expose:` for documenting
	// internal container ports. See yaml_bridge.go for the migration path.
	Ports YAMLStringSlice `yaml:"ports,omitempty"` // since: v0.1.0

	// Expose lists the container ports this dependency listens on internally.
	// Purely informational for raioz (not passed to Docker as the separate
	// `expose:` key, which is redundant on a shared network). When combined
	// with `publish: true` it defines which container ports get a host-side
	// binding; without publish it just documents what the image serves so
	// discovery can build correct URLs.
	Expose YAMLIntSlice `yaml:"expose,omitempty"` // since: v0.1.0

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
	Publish YAMLPublish `yaml:"publish,omitempty"` // since: v0.1.0

	Env             YAMLStringSlice `yaml:"env,omitempty"`             // since: v0.1.0
	Volumes         YAMLStringSlice `yaml:"volumes,omitempty"`         // since: v0.1.0
	Hostname        string          `yaml:"hostname,omitempty"`        // since: v0.1.0
	HostnameAliases YAMLStringSlice `yaml:"hostnameAliases,omitempty"` // since: v0.3.0
	Routing         *RoutingConfig  `yaml:"routing,omitempty"`         // since: v0.1.0
	Dev             *YAMLDevConfig  `yaml:"dev,omitempty"`             // since: v0.1.0

	// Proxy overrides how the shared HTTPS proxy reaches this dependency.
	// Same semantics as services.<name>.proxy — useful when `compose:`
	// launches a stack whose target container raioz can't infer, or when
	// the image's default port doesn't match what your process actually
	// listens on. Both fields optional; raioz falls back to detection for
	// whichever is left out.
	Proxy *YAMLServiceProxy `yaml:"proxy,omitempty"` // since: v0.1.1

	// Project points at a sibling raioz project that *is* this dependency
	// (mode A of issue #26). Path is relative to this raioz.yaml. When
	// set, the dep has no image/compose of its own — `raioz up` reads the
	// sibling's raioz.yaml and brings it up via a recursive `raioz up` in
	// the sibling's cwd if it's not already running. `raioz down` of the
	// consumer never touches the sibling.
	//
	// Mutually exclusive with Image / Compose / SiblingProject.
	Project string `yaml:"project,omitempty"` // since: v0.4.0

	// SiblingProject is the fallback variant (mode B of issue #26): pair
	// it with Image (or Compose) and raioz will skip the local image
	// declaration whenever the sibling project is active, but fall back
	// to the image when the sibling isn't running. Useful for CI and
	// contributors without the sibling clone.
	//
	// Mutually exclusive with Project.
	SiblingProject string `yaml:"siblingProject,omitempty"` // since: v0.4.0

	// RequiredHostname asks raioz to verify that the sibling actually
	// publishes this hostname before treating the dep as satisfied. Only
	// meaningful with Project or SiblingProject; ignored otherwise.
	RequiredHostname string `yaml:"requiredHostname,omitempty"` // since: v0.4.0
}
