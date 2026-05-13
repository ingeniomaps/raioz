package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/domain/models"
	raiozErrors "raioz/internal/errors"
	"raioz/internal/workspace"
)

const stateFileName = ".state.json"

// Save persists the entire models.Deps to .state.json as a post-up snapshot.
//
// Deprecated: This duplicates information that Docker labels and raioz.yaml
// already encode, and silently drifts whenever models.Deps grows a field.
// New callers MUST use LocalState (see internal/domain/models/state.go and
// the LoadLocalState / SaveLocalState helpers in project_state.go) and
// store only the minimal projection that cannot be recovered from Docker
// or the raioz.yaml. See ADR-011 for the rationale and the migration plan
// across issues 029/030/031.
func Save(ws *workspace.Workspace, deps *models.Deps) error {
	path := filepath.Join(ws.Root, stateFileName)
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, fmt.Sprintf("failed to marshal state: %v", err)).
			WithContext("path", path).
			WithError(err).
			WithSuggestion("The state data may be corrupted. Try running 'raioz down' and then 'raioz up' again")
	}
	// Use 0600 permissions (read/write for owner only) for security
	if err := os.WriteFile(path, data, 0600); err != nil {
		return raiozErrors.New(raiozErrors.ErrCodeStateSaveError, fmt.Sprintf("failed to write state file: %v", err)).
			WithContext("path", path).
			WithError(err).
			WithSuggestion("Verify file permissions and available disk space")
	}
	return nil
}

// Load reads the .state.json snapshot written by Save. See the Save
// docstring for why this whole-Deps snapshot is being phased out.
//
// Deprecated: Use LocalState (LoadLocalState in project_state.go) for
// runtime state and re-read raioz.yaml + Docker labels for everything
// else. See ADR-011.
func Load(ws *workspace.Workspace) (*models.Deps, error) {
	path := filepath.Join(ws.Root, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, raiozErrors.New(raiozErrors.ErrCodeStateLoadError, fmt.Sprintf("failed to read state file: %v", err)).
			WithContext("path", path).
			WithError(err).
			WithSuggestion("Check that the state file exists and is readable")
	}

	var deps models.Deps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, raiozErrors.New(raiozErrors.ErrCodeStateLoadError, fmt.Sprintf("failed to parse state file: %v", err)).
			WithContext("path", path).
			WithError(err).
			WithSuggestion("The state file may be corrupted. Try deleting it and running 'raioz up' again")
	}

	return &deps, nil
}

// Exists reports whether the legacy .state.json file is present. Used as
// a fast-path "have we ever run up here?" probe.
//
// Deprecated: New code should derive "is the project up?" from Docker
// labels via internal/docker.IsProjectActive (or
// container-listing equivalents). The presence of the JSON file is a
// proxy that diverges whenever the file fails to write or is manually
// removed. See ADR-011.
func Exists(ws *workspace.Workspace) bool {
	path := filepath.Join(ws.Root, stateFileName)
	_, err := os.Stat(path)
	return err == nil
}
