package checkcase

import (
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/state"
	workspacepkg "raioz/internal/workspace"
)

// handleNoState handles the case when no state exists
func (uc *UseCase) handleNoState() error {
	fmt.Println("ℹ️  No saved state found. This is normal for new projects.")
	fmt.Println("   Run 'raioz up' to create initial state.")
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
			"Failed to check alignment",
		).WithSuggestion(
			"This may indicate a problem with the state file or configuration. " +
				"Try running 'raioz down' and then 'raioz up' again.",
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
