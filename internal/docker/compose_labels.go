package docker

import "sort"

// sharedAwareProject returns the project name to stamp on a dependency
// container, or "" when an explicit workspace makes that container
// workspace-scoped and therefore shared between projects (ADR-002).
func sharedAwareProject(project string, hasExplicitWorkspace bool) string {
	if hasExplicitWorkspace {
		return ""
	}
	return project
}

// mergeLabels adds the raioz identity labels to a compose service config
// coming from a user-supplied fragment, without dropping labels the user
// already declared. Compose accepts both shapes: a mapping and a list of
// "k=v" strings.
func mergeLabels(config map[string]any, ours map[string]string) {
	switch existing := config["labels"].(type) {
	case map[string]any:
		for k, v := range ours {
			existing[k] = v
		}
	case map[string]string:
		for k, v := range ours {
			existing[k] = v
		}
	case []any:
		for _, k := range sortedKeys(ours) {
			existing = append(existing, k+"="+ours[k])
		}
		config["labels"] = existing
	case []string:
		for _, k := range sortedKeys(ours) {
			existing = append(existing, k+"="+ours[k])
		}
		config["labels"] = existing
	default:
		config["labels"] = ours
	}
}

// sortedKeys keeps the emitted order stable; Go map iteration is not.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
