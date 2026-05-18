package upcase

import (
	"os"
	"strings"

	"raioz/internal/protocol"
)

// metaAlreadyCompleted reports whether the meta runner has already
// brought projectName up successfully in the current run. Reads
// RAIOZ_META_COMPLETED_PROJECTS — a comma-separated list of project
// names the meta runner appends to after each successful sub-up. See
// internal/protocol/childenv.go for the producer contract.
//
// Empty project name never matches (defensive against misconfigured
// sibling info). Empty env var means standalone `raioz up` — every
// match is false and the regular IsProjectActive probe takes over.
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
