package app

import (
	"strings"

	"raioz/internal/protocol"
)

// withMetaCompleted stamps RAIOZ_META_COMPLETED_PROJECTS onto extraEnv
// on `up` when there's anything to report. Other commands and empty
// lists pass through unchanged so the spawn env stays minimal.
func withMetaCompleted(extraEnv, completed []string, subCmd string) []string {
	if subCmd != "up" || len(completed) == 0 {
		return extraEnv
	}
	out := append([]string(nil), extraEnv...)
	out = append(out, protocol.MetaCompletedProjects+"="+strings.Join(completed, ","))
	return out
}
