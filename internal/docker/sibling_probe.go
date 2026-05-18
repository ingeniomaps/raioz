package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// IsProjectActive reports whether (workspace, project) currently has at
// least one running raioz-managed container. Used by the sibling-deps
// flow (issue #26) to decide whether the consumer needs to spawn a
// recursive `raioz up` (mode A) or skip the dep entirely (mode B).
//
// `workspace` may be empty for projects without a workspace declaration —
// the workspace filter is then omitted, leaving project name as the sole
// discriminator. That's acceptable because workspace-less projects don't
// share docker networks anyway, so a name collision between a worskpace
// and a workspace-less project would never occur in practice.
//
// `fallbackContainerNames` covers the launcher-pattern gap: services
// whose `command:` shells out to make / docker compose produce
// containers without raioz labels, so the label probe misses them. The
// caller (sibling_dispatch) passes `proxy.target` values declared in
// the sibling yaml — if the label probe is empty AND any of those
// names resolves to a running container, the sibling counts as
// active. See docs/issues/020.
//
// Errors are propagated unchanged; callers (the orchestrator) decide
// fail-policy. We do NOT fail-open here — a docker outage is its own bug
// to surface, not a reason to silently spawn the sibling unnecessarily.
func IsProjectActive(ctx context.Context, workspace, project string,
	fallbackContainerNames ...string,
) (bool, error) {
	if project == "" {
		return false, fmt.Errorf("project name is required")
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(
		ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	args := []string{
		"ps",
		"--filter", "label=" + naming.LabelManaged + "=true",
		"--filter", "label=" + naming.LabelProject + "=" + project,
	}
	if workspace != "" {
		args = append(args, "--filter",
			"label="+naming.LabelWorkspace+"="+workspace)
	}
	args = append(args, "--format", "{{.Names}}")

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return false, fmt.Errorf("docker ps timed out after %v",
				exectimeout.DockerInspectTimeout)
		}
		return false, wrapDaemonError("docker ps", out, err)
	}

	if strings.TrimSpace(string(out)) != "" {
		return true, nil
	}

	return probeByContainerNames(ctx, fallbackContainerNames)
}

// probeByContainerNames queries `docker ps` by exact name match for
// each candidate. Returns true on the first running hit; absent or
// stopped containers don't count. Empty input returns (false, nil) —
// a sibling without proxy.target hints in its yaml just relies on the
// label probe.
func probeByContainerNames(ctx context.Context, names []string) (bool, error) {
	if len(names) == 0 {
		return false, nil
	}
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(
		ctx, exectimeout.DockerInspectTimeout)
	defer cancel()
	for _, name := range names {
		if name == "" {
			continue
		}
		cmd := exec.CommandContext(timeoutCtx, runtime.Binary(),
			"ps", "--filter", "name=^"+name+"$",
			"--filter", "status=running",
			"--format", "{{.Names}}")
		out, err := cmd.CombinedOutput()
		if err != nil {
			if exectimeout.IsTimeoutError(timeoutCtx, err) {
				return false, fmt.Errorf(
					"docker ps (launcher fallback) timed out after %v",
					exectimeout.DockerInspectTimeout)
			}
			return false, wrapDaemonError("docker ps", out, err)
		}
		if strings.TrimSpace(string(out)) != "" {
			return true, nil
		}
	}
	return false, nil
}
