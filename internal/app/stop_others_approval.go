package app

import (
	"bufio"
	"os"
	"strings"

	"raioz/internal/errors"
	"raioz/internal/fsutil"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// stopApproval decides whether projects other than the cwd one may be
// stopped. Another project is not ours to touch: every path that would
// stop one names the projects and the reason first, and goes ahead only on
// an explicit yes — typed at the prompt, or given up front with --yes.
type stopApproval func(names []string) (bool, error)

// Reasons a command gives for stopping other projects. Each is an i18n
// key, shown above the list so the user knows why they are being asked.
const (
	reasonConflicting = "output.stop_others_reason.conflicting"
	reasonAllProjects = "output.stop_others_reason.all_projects"
	reasonExclusive   = "output.stop_others_reason.exclusive"
	reasonWorkspace   = "output.stop_others_reason.workspace"
)

// stdinIsTerminalFn is a package var so tests can stand in for a terminal.
var stdinIsTerminalFn = func() bool { return fsutil.IsTerminal(os.Stdin) }

// approveStopping builds the approval for one command. With yes the list
// is still printed — pre-approved is not the same as unannounced. Without
// a terminal and without yes it refuses: a script must opt in explicitly.
func approveStopping(yes bool, reasonKey string) stopApproval {
	return func(names []string) (bool, error) {
		if len(names) == 0 {
			return true, nil
		}
		output.PrintSectionHeader(i18n.T("output.stop_others_header"))
		output.PrintInfo(i18n.T(reasonKey))
		for _, name := range names {
			output.PrintInfo(i18n.T("output.stop_others_item", name))
		}
		if yes {
			return true, nil
		}
		if !stdinIsTerminalFn() {
			return false, errors.New(
				errors.ErrCodeInvalidField,
				i18n.T("error.stop_others_needs_confirmation"),
			).WithSuggestion(i18n.T("error.stop_others_needs_confirmation_suggestion"))
		}

		output.PrintPrompt(i18n.T("output.stop_others_confirm"))
		response, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return false, errors.New(
				errors.ErrCodeInternalError,
				i18n.T("error.read_input"),
			).WithError(err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "y" || response == "yes" {
			return true, nil
		}
		output.PrintInfo(i18n.T("output.stop_others_cancelled"))
		return false, nil
	}
}
