package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadYAML loads a raioz.yaml file and returns the parsed RaiozConfig.
func LoadYAML(path string) (*RaiozConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}

	// ADR-036 hygiene rule H1: reject the yaml before parsing if it
	// contains anything that matches a known credential format. The
	// rest of the loader assumes the bytes are safe to surface in
	// error messages, which only holds after this gate.
	if err := ScanForSecrets(data); err != nil {
		return nil, err
	}

	var cfg RaiozConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf(
			"invalid YAML in %s: %w\n\n"+
				"  Tip: check indentation (use spaces, not tabs) "+
				"and ensure all colons have values",
			path, err,
		)
	}

	if err := validateYAMLConfig(&cfg, path); err != nil {
		return nil, err
	}

	absPath, _ := filepath.Abs(path)
	resolveYAMLPaths(&cfg, filepath.Dir(absPath))

	return &cfg, nil
}

// validateYAMLConfig checks required fields and basic structure.
func validateYAMLConfig(cfg *RaiozConfig, path string) error {
	if cfg.Project == "" {
		return fmt.Errorf("'project' is required in %s", path)
	}

	if len(cfg.Services) == 0 && len(cfg.Deps) == 0 {
		return fmt.Errorf("at least one service or dependency is required in %s", path)
	}

	for name, svc := range cfg.Services {
		if svc.Path == "" && svc.Git == "" {
			return fmt.Errorf("service '%s' must have either 'path' or 'git' field in %s", name, path)
		}
	}

	for name, dep := range cfg.Deps {
		if err := validateSiblingDependency(name, dep, path); err != nil {
			return err
		}
		// `project:` makes the sibling the runtime; `image:`/`compose:`
		// would be ignored, so the validator above already rejects them.
		// Here we only need the "at least one source" check for the
		// common (no-sibling) case.
		if dep.Project == "" && dep.Image == "" && len(dep.Compose) == 0 {
			return fmt.Errorf(
				"dependency '%s' must have one of 'image:', 'compose:', or 'project:' in %s",
				name, path)
		}
	}

	return validateDependsOnRefs(cfg)
}

// validateSiblingDependency enforces the mutual-exclusion rules around the
// sibling-project fields introduced in issue #26. Returns nil when the dep
// declares no sibling fields (the common case).
func validateSiblingDependency(name string, dep YAMLDependency, path string) error {
	hasProject := dep.Project != ""
	hasSibling := dep.SiblingProject != ""
	hasRequired := dep.RequiredHostname != ""

	if hasProject && hasSibling {
		return fmt.Errorf(
			"dependency '%s' in %s: 'project:' and 'siblingProject:' are mutually exclusive — "+
				"use 'project:' when the sibling is the only source, or 'siblingProject:' "+
				"with 'image:'/'compose:' for a fallback",
			name, path)
	}
	if hasProject && (dep.Image != "" || len(dep.Compose) > 0) {
		return fmt.Errorf(
			"dependency '%s' in %s: 'project:' is mutually exclusive with 'image:' and "+
				"'compose:' — drop the image/compose declaration, or switch to 'siblingProject:' "+
				"if you want a fallback",
			name, path)
	}
	if hasSibling && dep.Image == "" && len(dep.Compose) == 0 {
		return fmt.Errorf(
			"dependency '%s' in %s: 'siblingProject:' requires 'image:' or 'compose:' as the "+
				"fallback — use 'project:' instead if the sibling is the only source",
			name, path)
	}
	if hasRequired && !hasProject && !hasSibling {
		return fmt.Errorf(
			"dependency '%s' in %s: 'requiredHostname:' is only valid alongside 'project:' or "+
				"'siblingProject:'",
			name, path)
	}
	return nil
}

// validateDependsOnRefs checks that all dependsOn references point to defined services or dependencies.
func validateDependsOnRefs(cfg *RaiozConfig) error {
	known := make(map[string]bool)
	for name := range cfg.Services {
		known[name] = true
	}
	for name := range cfg.Deps {
		known[name] = true
	}

	for name, svc := range cfg.Services {
		for _, dep := range svc.DependsOn {
			if !known[dep] {
				return fmt.Errorf("service '%s' depends on '%s' which is not defined", name, dep)
			}
		}
	}

	return nil
}

// resolveYAMLPaths converts relative paths to be relative to the config file directory.
func resolveYAMLPaths(cfg *RaiozConfig, baseDir string) {
	for name, svc := range cfg.Services {
		if svc.Path != "" && !filepath.IsAbs(svc.Path) {
			svc.Path = filepath.Join(baseDir, svc.Path)
		}
		for i, envFile := range svc.Env {
			if !filepath.IsAbs(envFile) {
				svc.Env[i] = filepath.Join(baseDir, envFile)
			}
		}
		cfg.Services[name] = svc
	}

	for name, dep := range cfg.Deps {
		for i, envFile := range dep.Env {
			if !filepath.IsAbs(envFile) {
				dep.Env[i] = filepath.Join(baseDir, envFile)
			}
		}
		if dep.Dev != nil && dep.Dev.Path != "" && !filepath.IsAbs(dep.Dev.Path) {
			dep.Dev.Path = filepath.Join(baseDir, dep.Dev.Path)
		}
		// Sibling project paths (issue #26) point at another raioz.yaml
		// directory. Normalize to absolute so the resolver doesn't have
		// to track the consumer's cwd.
		if dep.Project != "" && !filepath.IsAbs(dep.Project) {
			dep.Project = filepath.Join(baseDir, dep.Project)
		}
		if dep.SiblingProject != "" && !filepath.IsAbs(dep.SiblingProject) {
			dep.SiblingProject = filepath.Join(baseDir, dep.SiblingProject)
		}
		cfg.Deps[name] = dep
	}

	for i, cmd := range cfg.Pre {
		if !strings.HasPrefix(cmd, "/") && !strings.HasPrefix(cmd, "~") &&
			(strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "../")) {
			cfg.Pre[i] = filepath.Join(baseDir, cmd)
		}
	}

	for i, cmd := range cfg.Post {
		if !strings.HasPrefix(cmd, "/") && !strings.HasPrefix(cmd, "~") &&
			(strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "../")) {
			cfg.Post[i] = filepath.Join(baseDir, cmd)
		}
	}
}

// IsYAMLConfig returns true if the file path looks like a YAML config.
func IsYAMLConfig(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// unknownFieldWarnings runs a second, strict decode purely to surface
// unknown fields as advisory warnings. Lenient decoding (LoadYAML) remains
// the source of truth for populating the config — yaml.v3's KnownFields
// mode promotes "unknown field" into an error we'd otherwise have to
// hard-fail on, which would break configs carrying custom annotations or
// fields added by future raioz versions.
//
// We rely on the invariant that if the lenient decode succeeded, any
// *yaml.TypeError from the strict pass is specifically an unknown-field
// error: genuine type mismatches (e.g. int into string) would have failed
// lenient decoding too.
func unknownFieldWarnings(path string, data []byte) []string {
	var discard RaiozConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	err := dec.Decode(&discard)
	if err == nil {
		return nil
	}
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return nil
	}
	base := filepath.Base(path)
	warnings := make([]string, 0, len(typeErr.Errors))
	for _, msg := range typeErr.Errors {
		warnings = append(warnings, fmt.Sprintf(
			"%s: %s — field ignored. Check for typos or fields from a newer raioz version.",
			base, msg))
	}
	return warnings
}

// readYAMLBytes is a small helper so callers that need both the parsed
// config AND the raw bytes (for the strict second pass) avoid reading
// the file twice.
func readYAMLBytes(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %s: %w", path, err)
	}
	return data, nil
}
