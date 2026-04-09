package config

// ApplyOverrides is a no-op in the new architecture.
// The override system has been replaced by `raioz dev` which manages
// dependency-to-local promotion via the LocalState in .raioz.state.json.
//
// This function is kept for backward compatibility with the bootstrap flow
// which calls it during `raioz up`. It returns the deps unchanged.
func ApplyOverrides(deps *Deps) (*Deps, []string, error) {
	return deps, nil, nil
}
