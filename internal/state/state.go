package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	raiozErrors "raioz/internal/errors"
	"raioz/internal/workspace"
)

const stateFileName = ".state.json"

func Save(ws *workspace.Workspace, deps *config.Deps) error {
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

func Load(ws *workspace.Workspace) (*config.Deps, error) {
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

	var deps config.Deps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, raiozErrors.New(raiozErrors.ErrCodeStateLoadError, fmt.Sprintf("failed to parse state file: %v", err)).
			WithContext("path", path).
			WithError(err).
			WithSuggestion("The state file may be corrupted. Try deleting it and running 'raioz up' again")
	}

	return &deps, nil
}

func Exists(ws *workspace.Workspace) bool {
	path := filepath.Join(ws.Root, stateFileName)
	_, err := os.Stat(path)
	return err == nil
}
