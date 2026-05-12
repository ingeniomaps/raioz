package upcase

import (
	"fmt"
	"os"
	"sort"

	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// findFreePort returns wanted (or the next available port above it) given
// the set of already-taken ports and an external bind probe. Caller passes
// `owner` for the error message when the search exhausts the port space.
//
// Implicit allocations bump on conflicts; explicit allocations treat
// external conflicts as a hard error. That difference matches the design:
// implicit is negotiable, explicit is sacred.
func findFreePort(wanted int, taken map[int]string, owner string) (int, error) {
	final := wanted
	// Non-root cannot bind to privileged ports, so iterating through 1..1023
	// would just burn ~944 doomed net.Listen() calls. Jump to the first
	// unprivileged port on the first iteration when that's our situation.
	if final < privilegedPortCeiling && os.Geteuid() != 0 {
		final = privilegedPortCeiling
	}
	for {
		if _, clash := taken[final]; !clash {
			inUse, err := portInUseProbe(fmt.Sprintf("%d", final))
			if err != nil || !inUse {
				return final, nil
			}
			// Externally bound (another raioz project, psql, anything). Bump.
		}
		final++
		if final > 65535 {
			return 0, errors.New(
				errors.ErrCodePortConflict,
				fmt.Sprintf(i18n.T("error.port_allocation_exhausted"), owner),
			)
		}
	}
}

// sortedKeys returns the keys of a map[string]T sorted alphabetically.
// Small helper so the allocator's determinism story stays obvious.
func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
