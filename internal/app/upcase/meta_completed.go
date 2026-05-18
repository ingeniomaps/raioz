package upcase

import (
	"os"
	"strings"

	"raioz/internal/protocol"
)

// metaAlreadyCompleted reports whether projectName is on the meta
// runner's completed list (see protocol.MetaCompletedProjects).
// Tolerant of surrounding whitespace in the env value.
func metaAlreadyCompleted(projectName string) bool {
	if projectName == "" {
		return false
	}
	raw := os.Getenv(protocol.MetaCompletedProjects)
	if raw == "" {
		return false
	}
	for _, name := range strings.Split(raw, ",") {
		if strings.TrimSpace(name) == projectName {
			return true
		}
	}
	return false
}
