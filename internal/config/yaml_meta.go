package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// MetaConfig is the resolved meta-orchestrator view of a raioz.yaml whose
// kind is "meta". The Workspace + Path of each sub-project are absolute.
type MetaConfig struct {
	Workspace string
	BaseDir   string
	Projects  []MetaProject
	// Router is the workspace's edge router project (ADR-037). When non-nil,
	// raioz brings this project up before any consumer and skips the bundled
	// Caddy. Resolved like a regular project — Path is absolute, Name is the
	// directory basename. Lifecycle wiring lives in internal/app/upcase.
	Router *MetaProject
}

// MetaProjectMode is the bootstrap mode resolved for a meta sub-project at
// load time. Drives MetaRunner.Up's pre-spawn pass (ADR-048, ADR-049).
type MetaProjectMode string

const (
	// MetaModeLocal — path exists; no bootstrap needed.
	MetaModeLocal MetaProjectMode = "local"
	// MetaModeClone — path missing and Git declared; bootstrap will clone.
	MetaModeClone MetaProjectMode = "clone"
	// MetaModeSkip — path missing, no Git, and Optional=true. Bootstrap
	// warns and drops the project from the run.
	MetaModeSkip MetaProjectMode = "skip"
	// MetaModeRemote — sub-project is proxied to an external URL (ADR-049).
	// No local spawn; bootstrap writes a workspace Caddy route file that
	// maps the project's hostname to the declared Remote URL. Reached
	// either by declaring `remote:` without `git:`, by --force-remote, or
	// as a clone fallback when both `git:` and `remote:` are declared and
	// the clone fails.
	MetaModeRemote MetaProjectMode = "remote"
)

// MetaProject is one resolved sub-project entry.
type MetaProject struct {
	// Name is the directory base name of Path — used purely for log / status
	// labels, not for matching.
	Name string
	// Path is the absolute path to the sub-project directory (where its
	// raioz.yaml lives).
	Path     string
	Optional bool
	// Profiles list the opt-in tags. Empty = always-on. Non-empty means
	// the project is skipped unless one of the user's active profiles
	// matches. See YAMLMetaProject.Profiles for the user-facing semantics.
	Profiles []string
	// Git carries through the user-declared clone source. Empty when the
	// sub-project is always expected to be present on disk. See ADR-048.
	Git string
	// Branch carries the optional git ref the bootstrap should checkout
	// after clone. Empty = remote default branch.
	Branch string
	// Auth selects the git auth provider for the clone (see
	// internal/git/auth/). Empty = strict.
	Auth string
	// Mode is the bootstrap mode resolved at load time. local / clone /
	// skip / remote — see MetaProjectMode for the contract.
	Mode MetaProjectMode
	// RelPath is Path relative to the meta config's BaseDir. Used by the
	// bootstrap to pass git.EnsureRepo a base+rel pair without re-deriving
	// it. Equal to Path when Path was absolute in the yaml.
	RelPath string
	// Remote carries the user-declared remote URL (ADR-049). Empty when
	// no remote fallback was declared. Validated at load time as parseable.
	Remote string
	// RemoteHostname is the hostname the workspace Caddy maps to Remote.
	// Empty after load = filepath.Base(Path); the bootstrap resolves this
	// default before writing the routes file so the on-disk state is
	// explicit.
	RemoteHostname string
}

// LoadMetaConfig parses the file at path as a meta-orchestrator config.
// Returns (nil, false, nil) when the file is a regular project config — the
// caller should fall back to the standard loader. Returns an error only on
// IO/parse failures or when `kind: meta` is set but the rest of the schema
// is invalid (no projects, missing path, startOrder doesn't match).
func LoadMetaConfig(path string) (*MetaConfig, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read %q: %w", path, err)
	}

	var raw RaiozConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, false, fmt.Errorf("parse %q: %w", path, err)
	}

	if raw.Kind != "meta" {
		return nil, false, nil
	}

	if len(raw.Projects) == 0 {
		return nil, true, fmt.Errorf(
			"%q: kind: meta requires a non-empty `projects:` list", path,
		)
	}

	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, true, fmt.Errorf("resolve base dir for %q: %w", path, err)
	}

	resolved := make([]MetaProject, 0, len(raw.Projects))
	byPath := make(map[string]int, len(raw.Projects))
	for i, p := range raw.Projects {
		mp, err := resolveMetaProject(path, baseDir, i, p)
		if err != nil {
			return nil, true, err
		}
		resolved = append(resolved, mp)
		byPath[p.Path] = len(resolved) - 1
	}

	if len(raw.StartOrder) > 0 {
		// Reorder `resolved` to match raw.StartOrder. Every entry in
		// StartOrder must reference a known `projects.path:` (string match,
		// not absolute path — the user wrote `keycloak`, not the full
		// absolute path).
		ordered := make([]MetaProject, 0, len(resolved))
		seen := make(map[int]bool, len(resolved))
		for _, key := range raw.StartOrder {
			idx, ok := byPath[key]
			if !ok {
				return nil, true, fmt.Errorf(
					"%q: startOrder entry %q does not match any projects.path",
					path, key,
				)
			}
			if seen[idx] {
				return nil, true, fmt.Errorf(
					"%q: startOrder entry %q listed more than once", path, key,
				)
			}
			seen[idx] = true
			ordered = append(ordered, resolved[idx])
		}
		// Append any project not explicitly listed (preserving the order
		// of the original `projects:` list). This makes startOrder a
		// "pin these first" hint instead of an exhaustive enumeration.
		for i, p := range resolved {
			if !seen[i] {
				ordered = append(ordered, p)
			}
		}
		resolved = ordered
	}

	router, err := resolveRouter(path, baseDir, raw.Router)
	if err != nil {
		return nil, true, err
	}

	return &MetaConfig{
		Workspace: raw.Workspace,
		BaseDir:   baseDir,
		Projects:  resolved,
		Router:    router,
	}, true, nil
}

// resolveMetaProject validates a single YAMLMetaProject entry and returns
// its resolved MetaProject. Mode resolution lives in decideMetaMode; this
// function owns the structural validation (path required, branch/auth/
// remoteHostname require their gating field, remote URL parses).
func resolveMetaProject(
	configPath, baseDir string, idx int, p YAMLMetaProject,
) (MetaProject, error) {
	if p.Path == "" {
		return MetaProject{}, fmt.Errorf(
			"%q: projects[%d] missing required `path:`", configPath, idx,
		)
	}
	if p.Git == "" && (p.Branch != "" || p.Auth != "") {
		return MetaProject{}, fmt.Errorf(
			"%q: projects[%d] (%q) declares branch/auth without git:",
			configPath, idx, p.Path,
		)
	}
	if p.RemoteHostname != "" && p.Remote == "" {
		return MetaProject{}, fmt.Errorf(
			"%q: projects[%d] (%q) declares remoteHostname without remote:",
			configPath, idx, p.Path,
		)
	}
	if err := validateRemoteURL(p.Remote); err != nil {
		return MetaProject{}, fmt.Errorf(
			"%q: projects[%d] (%q): %w", configPath, idx, p.Path, err,
		)
	}

	abs := p.Path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(baseDir, p.Path)
	}

	mode := decideMetaMode(abs, p)

	return MetaProject{
		Name:           filepath.Base(p.Path),
		Path:           abs,
		Optional:       p.Optional,
		Profiles:       append([]string(nil), p.Profiles...),
		Git:            p.Git,
		Branch:         p.Branch,
		Auth:           p.Auth,
		Mode:           mode,
		RelPath:        p.Path,
		Remote:         p.Remote,
		RemoteHostname: p.RemoteHostname,
	}, nil
}

// validateRemoteURL asserts the user-declared remote: URL parses to an
// http/https URL. Returns nil for empty (no remote declared). Other
// schemes (ws://, ftp://) are rejected — meta remote-mode is HTTP only
// (ADR-049). The exact host-side reachability check is deferred to
// runtime; load-time validation catches typos, not network issues.
func validateRemoteURL(remote string) error {
	if remote == "" {
		return nil
	}
	u, err := url.Parse(remote)
	if err != nil {
		return fmt.Errorf("remote: %q is not a parseable URL: %w", remote, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf(
			"remote: %q must use http or https scheme, got %q",
			remote, u.Scheme,
		)
	}
	if u.Host == "" {
		return fmt.Errorf("remote: %q is missing a host", remote)
	}
	return nil
}

// decideMetaMode applies the load-time cascade described in ADR-048.
// When no opt-in field (Git/Remote/Optional) matches and the path is
// missing, falls through to MetaModeLocal so read-only callers (status,
// lint, --dry-run) can parse the config without requiring the directory
// on disk. The actual "missing path" error surfaces at runSingle.
func decideMetaMode(abs string, p YAMLMetaProject) MetaProjectMode {
	if _, err := os.Stat(abs); err == nil {
		return MetaModeLocal
	}
	if p.Git != "" {
		// Bootstrap may downgrade to Remote when the clone fails and
		// Remote was declared as fallback (ADR-049 rule 1).
		return MetaModeClone
	}
	if p.Remote != "" {
		return MetaModeRemote
	}
	if p.Optional {
		return MetaModeSkip
	}
	return MetaModeLocal
}

// resolveRouter validates a top-level `router:` block and resolves its path
// to an absolute MetaProject. Returns nil when no router is declared. The
// router path may overlap with an entry in `projects:` — see ADR-037.
func resolveRouter(configPath, baseDir string, r *YAMLRouter) (*MetaProject, error) {
	if r == nil {
		return nil, nil
	}
	abs, err := validateRouterRef(configPath, baseDir, r)
	if err != nil {
		return nil, err
	}
	return &MetaProject{
		Name: filepath.Base(r.Project),
		Path: abs,
	}, nil
}

// validateRouterRef enforces the ADR-037 structural rules for a `router:`
// block: project is required and resolves to an absolute path. Existence
// of the target directory + its raioz.yaml is deliberately deferred to
// up-time (matching the sibling-dep contract from ADR-008): static
// fixtures and offline tooling parse a config that points at a path not
// yet checked out, and only `raioz up` needs the target to be present.
// Returns the absolute path on success so callers can persist it.
func validateRouterRef(configPath, baseDir string, r *YAMLRouter) (string, error) {
	if r.Project == "" {
		return "", fmt.Errorf(
			"%q: router.project is required when `router:` is declared", configPath,
		)
	}
	abs := r.Project
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(baseDir, r.Project)
	}
	return abs, nil
}
