package cli

import (
	"os"
	"path/filepath"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// AutoDetectMarker is returned by ResolveConfigPath when no config file is found,
// signaling that raioz should auto-detect the project structure.
const AutoDetectMarker = ":auto:"

// configCandidates lists config file names in priority order.
var configCandidates = []string{
	"raioz.yaml",
	"raioz.yml",
	".raioz.json",
}

// ResolveConfigPath returns the path to use for the config file.
// If the given path is empty, searches for raioz.yaml, raioz.yml, then .raioz.json.
// If none found, returns AutoDetectMarker to signal zero-config mode.
// If a path is provided, it is returned as-is (absolute when possible).
func ResolveConfigPath(path string) string {
	if path == "" {
		for _, candidate := range configCandidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return AutoDetectMarker
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// ResolveProjectConfigPath is ResolveConfigPath for commands that also
// take `-p <project>`: with no explicit file, the named project's own
// raioz.yaml is used wherever the command is run from. A name that is not
// an active project falls back to the config in the cwd; with none there
// either, that is an error — acting on "nothing" must not look like
// success.
func ResolveProjectConfigPath(path, project string) (string, error) {
	if path != "" || project == "" {
		return ResolveConfigPath(path), nil
	}
	if config, ok := app.ActiveProjectConfig(project); ok {
		return config, nil
	}
	local := ResolveConfigPath("")
	if local == AutoDetectMarker {
		return "", errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.project_not_active", project),
		).WithSuggestion(i18n.T("error.project_not_active_suggestion"))
	}
	return local, nil
}
