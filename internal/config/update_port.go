package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// UpdatePort rewrites a single port value inside a raioz.yaml file using
// line-level manipulation so comments and formatting are preserved.
//
// kind is "service" or "dep". name is the service/dependency key.
// oldPort and newPort are the host-side ports. The function locates the
// YAML block for `name` under the appropriate top-level key (services or
// dependencies) and replaces the first occurrence of oldPort with newPort
// in a `port:` (services) or `publish:` (deps) line.
func UpdatePort(configPath, name, kind string, oldPort, newPort int) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	updated := updatePortLines(lines, name, kind, oldPort, newPort)
	if updated == nil {
		return nil // nothing changed
	}

	if err := os.WriteFile(configPath, []byte(strings.Join(updated, "\n")), 0644); err != nil {
		return fmt.Errorf("write updated config %q: %w", configPath, err)
	}
	return nil
}

// updatePortLines does the line-level search-and-replace. Returns nil when
// no substitution was made.
func updatePortLines(lines []string, name, kind string, oldPort, newPort int) []string {
	// Determine the top-level section and the field we are looking for.
	var section, field string
	switch kind {
	case "service":
		section = "services"
		field = "port"
	case "dep":
		section = "dependencies"
		field = "publish"
	default:
		return nil
	}

	// Phase 1: find the top-level section line (e.g. "services:")
	sectionRe := regexp.MustCompile(`^` + section + `\s*:`)
	inSection := false
	inTarget := false
	targetRe := regexp.MustCompile(`^\s+` + regexp.QuoteMeta(name) + `\s*:`)
	// Match field: <old> where <old> is the integer to replace.
	fieldRe := regexp.MustCompile(
		`^(\s+` + field + `\s*:\s*)` + fmt.Sprintf("%d", oldPort) + `(\s*(?:#.*)?)$`,
	)

	result := make([]string, len(lines))
	copy(result, lines)
	changed := false

	for i, line := range result {
		// Detect top-level keys (no leading whitespace).
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			inSection = sectionRe.MatchString(line)
			inTarget = false
			continue
		}

		if !inSection {
			continue
		}

		// Detect second-level keys (the service/dep name).
		if targetRe.MatchString(line) {
			inTarget = true
			continue
		}

		// A different second-level key inside the same section ends our target.
		if inTarget && len(line) > 0 && line[0] != '#' {
			trimmed := strings.TrimLeft(line, " \t")
			if len(trimmed) > 0 && trimmed[0] != '#' {
				indent := len(line) - len(trimmed)
				// Second-level keys typically have 2-space indent.
				if indent <= 2 && strings.Contains(trimmed, ":") {
					inTarget = false
				}
			}
		}

		if !inTarget {
			continue
		}

		if fieldRe.MatchString(line) {
			result[i] = fieldRe.ReplaceAllString(line,
				fmt.Sprintf("${1}%d${2}", newPort))
			changed = true
			break
		}
	}

	if !changed {
		return nil
	}
	return result
}
