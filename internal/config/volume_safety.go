package config

import (
	"path/filepath"
	"sort"
	"strings"

	"raioz/internal/i18n"
)

// systemVolumeWarnings returns one warning per dependency bind mount whose
// host side is a sensitive system directory (the H2 blocklist). It warns
// instead of rejecting: mounting a host path into a container is the point
// of `volumes:`, and a dev may have a reason — but handing /etc or /root to
// an image pulled from a registry should never pass in silence.
//
// Named volumes (`pgdata:/var/lib/postgresql/data`) have no host path and
// are skipped. baseDir resolves relative sources.
func systemVolumeWarnings(cfg *RaiozConfig, baseDir string) []string {
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
		for _, spec := range cfg.Deps[name].Volumes {
			source, ok := bindMountSource(spec)
			if !ok {
				continue
			}
			abs := resolveAbs(source, baseDir)
			if blocklistError(source, abs, "") != nil {
				warnings = append(warnings, i18n.T("warning.volume_system_dir", name, abs))
			}
		}
	}
	return warnings
}

// bindMountSource returns the host side of a `source:target[:mode]` volume
// spec when the source is a path. A bare name is a Docker named volume.
func bindMountSource(spec string) (string, bool) {
	source, _, found := strings.Cut(spec, ":")
	if !found || source == "" {
		return "", false
	}
	if filepath.IsAbs(source) || strings.HasPrefix(source, ".") {
		return source, true
	}
	return "", false
}
