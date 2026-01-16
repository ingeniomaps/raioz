package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

const stateFileName = ".state.json"

func Save(ws *workspace.Workspace, deps *config.Deps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	// Use 0600 permissions (read/write for owner only) for security
	return os.WriteFile(filepath.Join(ws.Root, stateFileName), data, 0600)
}

func Load(ws *workspace.Workspace) (*config.Deps, error) {
	path := filepath.Join(ws.Root, stateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var deps config.Deps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}

	return &deps, nil
}

func Exists(ws *workspace.Workspace) bool {
	path := filepath.Join(ws.Root, stateFileName)
	_, err := os.Stat(path)
	return err == nil
}
