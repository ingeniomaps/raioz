package upcase

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// preHookExec runs pre-hooks before starting services. A pre-hook failure
// aborts `raioz up` — use it for critical setup (secrets, env rendering).
func (uc *UseCase) preHookExec(ctx context.Context, deps *config.Deps, projectDir string) error {
	if deps.PreHook == "" {
		return nil
	}

	output.PrintProgress(i18n.T("up.running_pre_hook"))
	logging.InfoWithContext(ctx, "Executing pre-hook", "command", deps.PreHook)

	commands := strings.Split(deps.PreHook, " && ")
	for _, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.Dir = projectDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return errors.PreHookFailed(cmdStr, fmt.Errorf("%w\n%s", err, string(out)))
		}
	}

	output.PrintProgressDone(i18n.T("up.pre_hook_done"))
	return nil
}

// postHookExec runs post-hooks after starting services. Post-hook failures
// are logged as warnings and do NOT fail `raioz up` — services are already
// running and the user can inspect the warning.
func (uc *UseCase) postHookExec(ctx context.Context, deps *config.Deps, projectDir string) {
	if deps.PostHook == "" {
		return
	}

	output.PrintProgress(i18n.T("up.running_post_hook"))
	logging.InfoWithContext(ctx, "Executing post-hook", "command", deps.PostHook)

	commands := strings.Split(deps.PostHook, " && ")
	for _, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.Dir = projectDir
		if out, err := cmd.CombinedOutput(); err != nil {
			logging.WarnWithContext(ctx, "Post-hook failed",
				"command", cmdStr, "error", err.Error(), "output", string(out))
			output.PrintWarning(fmt.Sprintf(i18n.T("up.post_hook_failed"), cmdStr))
			return
		}
	}

	output.PrintProgressDone(i18n.T("up.post_hook_done"))
}
