package config

import (
	"sort"
	"strings"

	"raioz/internal/i18n"
)

// checkImagePinning returns a warning string when image is not pinned
// to a specific tag, "" otherwise. ADR-036 hygiene rule H3: a dep
// pulled at :latest (implicit or explicit) drifts across machines and
// over time; raioz nudges the user toward a stable tag or digest.
//
// "Pinned" means either an explicit non-"latest" tag (e.g. ":16",
// ":16.3-alpine") or a digest reference (`@sha256:...`). Empty image
// is treated as pinned by omission — it's typically a compose-backed
// dep where the tag lives in the compose file.
//
// Tag parsing handles registry host:port prefixes: the last
// "/"-separated segment is where the optional tag lives, so
// "registry:5000/foo/bar" is correctly read as "no tag on bar".
func checkImagePinning(depName, image string) string {
	if image == "" {
		return ""
	}
	if strings.Contains(image, "@sha256:") {
		return ""
	}
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	tagIdx := strings.LastIndex(last, ":")
	if tagIdx == -1 {
		return i18n.T("warning.image_no_tag", depName, image)
	}
	tag := last[tagIdx+1:]
	if tag == "latest" {
		return i18n.T("warning.image_latest", depName, image)
	}
	return ""
}

// imagePinningWarnings walks cfg.Deps in sorted name order and
// accumulates H3 warnings. Sorted iteration keeps caller output and
// tests deterministic; map iteration order in Go is randomized.
func imagePinningWarnings(cfg *RaiozConfig) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Deps))
	for name := range cfg.Deps {
		names = append(names, name)
	}
	sort.Strings(names)
	var warnings []string
	for _, name := range names {
		if w := checkImagePinning(name, cfg.Deps[name].Image); w != "" {
			warnings = append(warnings, w)
		}
	}
	return warnings
}
