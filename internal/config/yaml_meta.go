package config

import (
	"fmt"
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
}

// MetaProject is one resolved sub-project entry.
type MetaProject struct {
	// Name is the directory base name of Path — used purely for log / status
	// labels, not for matching.
	Name string
	// Path is the absolute path to the sub-project directory (where its
	// raioz.yaml lives).
	Path     string
	Optional bool
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
		if p.Path == "" {
			return nil, true, fmt.Errorf(
				"%q: projects[%d] missing required `path:`", path, i,
			)
		}
		abs := p.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(baseDir, p.Path)
		}
		resolved = append(resolved, MetaProject{
			Name:     filepath.Base(p.Path),
			Path:     abs,
			Optional: p.Optional,
		})
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

	return &MetaConfig{
		Workspace: raw.Workspace,
		BaseDir:   baseDir,
		Projects:  resolved,
	}, true, nil
}
