package app

import (
	"strings"

	"raioz/internal/protocol"
)

// withMetaCompleted appends RAIOZ_META_COMPLETED_PROJECTS to extraEnv
// when subCmd is "up" and completed is non-empty. Returns extraEnv
// unchanged otherwise (down/status don't carry the hint; an empty
// list would set the env to "" and be parsed as "no completed", same
// as omitting it — but omitting keeps the spawn env clean).
//
// The completed list is comma-joined. The consumer
// (upcase.metaAlreadyCompleted) splits on comma and trims surrounding
// whitespace for tolerance.
func withMetaCompleted(extraEnv, completed []string, subCmd string) []string {
	if subCmd != "up" || len(completed) == 0 {
		return extraEnv
	}
	out := append([]string(nil), extraEnv...)
	out = append(out, protocol.MetaCompletedProjects+"="+strings.Join(completed, ","))
	return out
}
