package app

// validateFastPreflight performs only critical preflight checks
func (uc *CIUseCase) validateFastPreflight() error {
	// Only check Docker (required for CI)
	if err := uc.deps.Validator.CheckDockerInstalled(); err != nil {
		return err
	}
	if err := uc.deps.Validator.CheckDockerRunning(); err != nil {
		return err
	}
	// Skip disk space, network, git checks for speed in CI
	return nil
}
