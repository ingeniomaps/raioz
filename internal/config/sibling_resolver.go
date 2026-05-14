package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// SiblingInfo describes a sibling raioz project referenced by a
// dependency through `project:` (mode A) or `siblingProject:` (mode B)
// of issue #26. It carries just enough metadata to drive the
// orchestration decisions in later phases — no docker state, no I/O
// beyond the initial config read.
type SiblingInfo struct {
	// Path is the absolute path to the sibling's raioz.yaml file.
	Path string
	// Dir is the absolute path to the directory holding the sibling's
	// raioz.yaml — the cwd raioz uses for the recursive `up` spawn in
	// mode A.
	Dir string
	// Project is the sibling's `project:` field. Always non-empty when
	// ResolveSibling returns nil error.
	Project string
	// Workspace is the sibling's `workspace:` field. Empty string when
	// the sibling is workspace-less.
	Workspace string
	// Hostnames lists every hostname the sibling exposes — services
	// (explicit `hostname:` or the service key as fallback) plus deps
	// that override their hostname, plus any hostnameAliases. Used by
	// the requiredHostname check in Phase 7.
	Hostnames []string
}

// ResolveSibling reads the raioz.yaml at dir and extracts the metadata
// needed for cross-project dependency resolution. The path must be a
// directory — the upstream loader normalizes relative `project:` /
// `siblingProject:` values to absolute via resolveYAMLPaths, so callers
// in production always pass an absolute path.
func ResolveSibling(dir string) (*SiblingInfo, error) {
	if dir == "" {
		return nil, fmt.Errorf("sibling path is empty")
	}
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("sibling path %q must be absolute", dir)
	}

	cfgPath := filepath.Join(dir, "raioz.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"sibling raioz.yaml not found at %s — check the 'project:' / "+
					"'siblingProject:' path in the consumer's raioz.yaml",
				cfgPath)
		}
		return nil, fmt.Errorf("stat sibling raioz.yaml at %s: %w", cfgPath, err)
	}

	// Reject meta-orchestrator configs up front — LoadYAML rejects them
	// too, but with a misleading "at least one service or dependency"
	// message. LoadMetaConfig is the canonical detector and reads the
	// file once; the parsed payload is discarded.
	if _, isMeta, _ := LoadMetaConfig(cfgPath); isMeta {
		return nil, fmt.Errorf(
			"sibling raioz.yaml at %s is a meta-orchestrator (kind: meta) — "+
				"point 'project:' / 'siblingProject:' at a regular project, "+
				"not at the meta config",
			cfgPath)
	}

	cfg, err := LoadYAML(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load sibling raioz.yaml at %s: %w", cfgPath, err)
	}
	if cfg.Project == "" {
		return nil, fmt.Errorf(
			"sibling raioz.yaml at %s does not declare 'project:' — "+
				"a sibling must be a regular raioz project",
			cfgPath)
	}

	return &SiblingInfo{
		Path:      cfgPath,
		Dir:       dir,
		Project:   cfg.Project,
		Workspace: cfg.Workspace,
		Hostnames: collectSiblingHostnames(cfg),
	}, nil
}

// collectSiblingHostnames gathers every hostname the sibling exposes.
// Order: services first (alphabetic by map iteration is non-deterministic;
// callers that care about order should sort), then deps. Duplicates are
// dropped while preserving first-seen ordering for stability across
// repeated calls within one process.
func collectSiblingHostnames(cfg *RaiozConfig) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(h string) {
		if h == "" {
			return
		}
		if _, dup := seen[h]; dup {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}

	for name, svc := range cfg.Services {
		if svc.Hostname != "" {
			add(svc.Hostname)
		} else {
			add(name)
		}
		for _, alias := range svc.HostnameAliases {
			add(alias)
		}
	}
	for name, dep := range cfg.Deps {
		if dep.Hostname != "" {
			add(dep.Hostname)
		} else if dep.Image != "" || len(dep.Compose) > 0 {
			// Image/compose deps are reachable as `<name>` by default.
			// Sibling-only deps (project:) have no container of their
			// own, so we don't claim a hostname for them.
			add(name)
		}
		for _, alias := range dep.HostnameAliases {
			add(alias)
		}
	}
	return out
}

// ValidateSiblingWorkspace asserts that the consumer and sibling share
// a workspace — they need the same docker network for DNS resolution
// to work. Hard error; no escape hatch today.
func ValidateSiblingWorkspace(consumerWorkspace string, sib *SiblingInfo) error {
	if sib == nil {
		return fmt.Errorf("nil sibling info")
	}
	if consumerWorkspace == sib.Workspace {
		return nil
	}
	switch {
	case consumerWorkspace == "":
		return fmt.Errorf(
			"sibling project %q declares workspace %q but the consumer "+
				"declares none — add `workspace: %s` to the consumer's "+
				"raioz.yaml so they share a docker network",
			sib.Project, sib.Workspace, sib.Workspace)
	case sib.Workspace == "":
		return fmt.Errorf(
			"sibling project %q declares no workspace but the consumer "+
				"is in workspace %q — add `workspace: %s` to the sibling's "+
				"raioz.yaml so they share a docker network",
			sib.Project, consumerWorkspace, consumerWorkspace)
	default:
		return fmt.Errorf(
			"sibling project %q is in workspace %q but the consumer is in %q — "+
				"siblings must share a workspace to share the docker network",
			sib.Project, sib.Workspace, consumerWorkspace)
	}
}

// SiblingHasHostname reports whether a hostname is present in the
// sibling's declared set. Used by the requiredHostname check (Phase 7).
func (s *SiblingInfo) SiblingHasHostname(host string) bool {
	if s == nil {
		return false
	}
	return slices.Contains(s.Hostnames, host)
}
