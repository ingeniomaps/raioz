package app

import (
	"context"
	"path/filepath"

	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
)

// stopProxy stops the Caddy proxy if it's running.
func (uc *DownUseCase) stopProxy(ctx context.Context, opts DownOptions) {
	if uc.deps.ProxyManager == nil {
		return
	}

	running, err := uc.deps.ProxyManager.Status(ctx)
	if err != nil || !running {
		return
	}

	output.PrintInfo("Stopping proxy...")
	if err := uc.deps.ProxyManager.Stop(ctx); err != nil {
		logging.WarnWithContext(ctx, "Failed to stop proxy", "error", err.Error())
		output.PrintWarning("Failed to stop proxy: " + err.Error())
	} else {
		output.PrintSuccess("Proxy stopped")
	}
}

// cleanLocalState removes the .raioz.state.json from the project directory.
func (uc *DownUseCase) cleanLocalState(ctx context.Context, opts DownOptions) {
	if opts.ConfigPath == "" {
		return
	}

	projectDir, err := filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		return
	}

	if err := state.RemoveLocalState(projectDir); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove local state", "error", err.Error())
	}
}
