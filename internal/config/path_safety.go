package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// systemBlocklist enumerates absolute paths that raioz refuses to
// touch via raioz.yaml as defense-in-depth. The list is Linux-shaped;
// on macOS / Windows these strings simply don't resolve to anything,
// which is harmless.
//
// We do NOT block /tmp, /var/tmp, or /home/* because those are
// legitimate locations for sibling project paths, snapshot output, or
// scratch dirs declared by users who know what they're doing.
var systemBlocklist = []string{
	"/etc",
	"/root",
	"/var/lib",
	"/sys",
	"/proc",
	"/dev",
	"/boot",
}

// validatePathSafety enforces ADR-036 hygiene rule H2: every path
// referenced by raioz.yaml must resolve inside the containment root and
// must not point to a known sensitive system directory. baseDir must
// already be absolute.
//
// The containment root defaults to baseDir (the yaml's own directory).
// When the config declares workspaceRoot, the root widens to that
// directory so a multi-repo workspace (yaml in a sub-repo, services in
// sibling repos via `../`) validates as contained — paths are still
// HARD-contained, just against a boundary the dev declared explicitly.
// Relative paths always resolve against baseDir regardless of root.
//
// The check is purely textual — symlinks are not resolved. An attacker
// who can write a malicious symlink already has repo write access,
// which is a separate threat model.
//
// Sibling project paths (dep.Project, dep.SiblingProject) are
// intentionally NOT validated against the root: by design (ADR-008)
// they point at other raioz projects elsewhere on the dev's machine.
// They ARE still checked against the system blocklist.
func validatePathSafety(cfg *RaiozConfig, baseDir string) error {
	root := baseDir
	if cfg.WorkspaceRoot != "" {
		// The declared root itself must not be a system dir — otherwise
		// `workspaceRoot: /etc` would re-open everything H2 closes.
		if err := checkSystemBlocklist(cfg.WorkspaceRoot, baseDir,
			"workspaceRoot"); err != nil {
			return err
		}
		root = resolveAbs(cfg.WorkspaceRoot, baseDir)
	}

	for name, svc := range cfg.Services {
		if err := checkInsideRoot(svc.Path, baseDir, root,
			"services."+name+".path"); err != nil {
			return err
		}
		for i, env := range svc.Env {
			if err := checkInsideRoot(env, baseDir, root,
				fmt.Sprintf("services.%s.env[%d]", name, i)); err != nil {
				return err
			}
		}
		for i, comp := range svc.Compose {
			if err := checkInsideRoot(comp, baseDir, root,
				fmt.Sprintf("services.%s.compose[%d]", name, i)); err != nil {
				return err
			}
		}
		if p, ok := pathFromCommand(svc.Command); ok {
			if err := checkInsideRoot(p, baseDir, root,
				"services."+name+".command"); err != nil {
				return err
			}
		}
		if p, ok := pathFromCommand(svc.Stop); ok {
			if err := checkInsideRoot(p, baseDir, root,
				"services."+name+".stop"); err != nil {
				return err
			}
		}
	}

	for name, dep := range cfg.Deps {
		for i, env := range dep.Env {
			if err := checkInsideRoot(env, baseDir, root,
				fmt.Sprintf("dependencies.%s.env[%d]", name, i)); err != nil {
				return err
			}
		}
		for i, comp := range dep.Compose {
			if err := checkInsideRoot(comp, baseDir, root,
				fmt.Sprintf("dependencies.%s.compose[%d]", name, i)); err != nil {
				return err
			}
		}
		if dep.Dev != nil {
			if err := checkInsideRoot(dep.Dev.Path, baseDir, root,
				"dependencies."+name+".dev.path"); err != nil {
				return err
			}
		}
		// Sibling project paths: blocklist-only (see comment above).
		if err := checkSystemBlocklist(dep.Project, baseDir,
			"dependencies."+name+".project"); err != nil {
			return err
		}
		if err := checkSystemBlocklist(dep.SiblingProject, baseDir,
			"dependencies."+name+".siblingProject"); err != nil {
			return err
		}
	}

	if cfg.Router != nil {
		// Router project paths follow the same blocklist-only rule as
		// sibling project paths (ADR-008): they point at another raioz
		// project that may legitimately live outside baseDir.
		if err := checkSystemBlocklist(cfg.Router.Project, baseDir,
			"router.project"); err != nil {
			return err
		}
	}

	for i, cmd := range cfg.Pre {
		if p, ok := pathFromCommand(cmd); ok {
			if err := checkInsideRoot(p, baseDir, root,
				fmt.Sprintf("pre[%d]", i)); err != nil {
				return err
			}
		}
	}
	for i, cmd := range cfg.PreUp {
		if p, ok := pathFromCommand(cmd); ok {
			if err := checkInsideRoot(p, baseDir, root,
				fmt.Sprintf("preUp[%d]", i)); err != nil {
				return err
			}
		}
	}
	for i, cmd := range cfg.Post {
		if p, ok := pathFromCommand(cmd); ok {
			if err := checkInsideRoot(p, baseDir, root,
				fmt.Sprintf("post[%d]", i)); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkInsideRoot runs the full H2 check: system blocklist AND
// containment within root. rawPath is resolved relative to baseDir
// (where the yaml lives) but contained against root (which equals
// baseDir by default, or the declared workspaceRoot). Empty rawPath is
// a no-op so callers don't need to guard.
func checkInsideRoot(rawPath, baseDir, root, field string) error {
	if rawPath == "" {
		return nil
	}
	abs := resolveAbs(rawPath, baseDir)
	if err := blocklistError(rawPath, abs, field); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return errors.New(
			errors.ErrCodeUnsafePath,
			i18n.T("error.path_invalid", rawPath),
		).WithContext("field", field).
			WithContext("path", rawPath).
			WithError(err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New(
			errors.ErrCodeUnsafePath,
			i18n.T("error.path_escapes_repo", rawPath, abs),
		).WithSuggestion(
			i18n.T("error.path_escapes_repo_suggestion"),
		).WithContext("field", field).
			WithContext("path", rawPath).
			WithContext("resolved", abs)
	}
	return nil
}

// checkSystemBlocklist runs ONLY the system-dir check (no containment
// check). Used for sibling project paths that legitimately escape the
// caller's project dir.
func checkSystemBlocklist(rawPath, baseDir, field string) error {
	if rawPath == "" {
		return nil
	}
	abs := resolveAbs(rawPath, baseDir)
	return blocklistError(rawPath, abs, field)
}

// resolveAbs turns rawPath into an absolute, cleaned form using
// baseDir as the resolution root for relative inputs.
func resolveAbs(rawPath, baseDir string) string {
	if filepath.IsAbs(rawPath) {
		return filepath.Clean(rawPath)
	}
	return filepath.Clean(filepath.Join(baseDir, rawPath))
}

// blocklistError returns a structured error when abs targets a known
// sensitive system directory, nil otherwise.
func blocklistError(rawPath, abs, field string) error {
	for _, b := range systemBlocklist {
		if abs == b || strings.HasPrefix(abs, b+string(filepath.Separator)) {
			return errors.New(
				errors.ErrCodeUnsafePath,
				i18n.T("error.path_in_system_dir", rawPath, b),
			).WithSuggestion(
				i18n.T("error.path_in_system_dir_suggestion"),
			).WithContext("field", field).
				WithContext("path", rawPath).
				WithContext("resolved", abs).
				WithContext("system_dir", b)
		}
	}
	return nil
}

// pathFromCommand extracts the first token of cmd when it looks like
// a path. Returns "", false for bare commands resolved via PATH like
// "make build" or "bash ./scripts/foo.sh" — in the latter case the
// first token is "bash", not a path, so we don't try to validate the
// inner path. The accepted miss is documented and intentional: shell
// constructions are the user's responsibility.
func pathFromCommand(cmd string) (string, bool) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", false
	}
	firstToken := strings.SplitN(cmd, " ", 2)[0]
	if strings.HasPrefix(firstToken, "./") ||
		strings.HasPrefix(firstToken, "../") ||
		strings.HasPrefix(firstToken, "/") {
		return firstToken, true
	}
	return "", false
}
