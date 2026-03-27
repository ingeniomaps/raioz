package checkcase

import (
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/state"
	workspacepkg "raioz/internal/workspace"
)

// handleNoState handles the case when no state exists
func (uc *UseCase) handleNoState() error {
	fmt.Println(i18n.T("check.no_state_found"))
	fmt.Println(i18n.T("check.run_up_hint"))
	os.Exit(0)
	return nil
}

// checkAndDisplayAlignment checks alignment and displays results
func (uc *UseCase) checkAndDisplayAlignment(ws *interfaces.Workspace, currentDeps *config.Deps) error {
	// Convert interfaces.Workspace to concrete workspace.Workspace for state.CheckAlignment
	wsConcrete := (*workspacepkg.Workspace)(ws)

	// Check alignment
	issues, err := state.CheckAlignment(wsConcrete, currentDeps)
	if err != nil {
		return errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.check_alignment"),
		).WithSuggestion(
			i18n.T("error.check_alignment_suggestion"),
		).WithError(err)
	}

	// Display issues
	fmt.Println(state.FormatIssues(issues))

	// Exit with appropriate code
	if state.HasCriticalIssues(issues) || state.HasWarningOrCriticalIssues(issues) {
		// Exit code 1 for critical or warning issues
		os.Exit(1)
		return nil
	}

	// Exit code 0 for no issues or only info issues (branch drift)
	return nil
}
