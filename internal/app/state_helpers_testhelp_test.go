package app

import (
	"raioz/internal/state"
)

// writeFakeLocalStateForTest writes a minimal LocalState file with the
// given ProjectComposePath into projectDir. Tests use it to simulate
// the state `raioz up` would have written, without requiring Docker.
//
// Lives in a _test.go so it's only linked into tests.
func writeFakeLocalStateForTest(projectDir, projectComposePath string) error {
	ls, err := state.LoadLocalState(projectDir)
	if err != nil {
		return err
	}
	ls.ProjectComposePath = projectComposePath
	ls.Project = "test-project"
	return state.SaveLocalState(projectDir, ls)
}
