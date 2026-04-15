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
		// A dep needs either `image:` (raioz generates a minimal compose)
		// or `compose:` (user supplies existing fragments). YAMLToDeps
		// re-validates mutual exclusion; here we just check "at least
		// one" so the simpler error message surfaces first.
		if dep.Image == "" && len(dep.Compose) == 0 {
			return fmt.Errorf(
				"dependency '%s' must have either 'image:' or 'compose:' in %s", name, path)
		}
	}

	return validateDependsOnRefs(cfg)
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
