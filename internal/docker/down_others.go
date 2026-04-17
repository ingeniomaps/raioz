package docker

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	exectimeout "raioz/internal/exec"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// ListActiveProjects returns the deduplicated set of project names that
// currently have at least one running raioz-managed container, regardless
// of workspace. Containers are identified by the LabelManaged stamp;
// project names come from LabelProject. Containers without a project
// label (workspace-shared deps) are intentionally skipped — they have no
// owning project to "stop".
func ListActiveProjects(ctx context.Context) ([]string, error) {
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(
		ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "ps",
		"--filter", "label="+naming.LabelManaged+"=true",
		"--format", "{{.Label \""+naming.LabelProject+"\"}}")
	out, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return nil, fmt.Errorf("docker ps timed out after %v",
				exectimeout.DockerInspectTimeout)
		}
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// StopProjectContainers stops and removes every container labeled with the
// given project. Returns the names of containers that were torn down. Used
// by `raioz down --conflicting` / `--all-projects` to free host ports held
// by sibling projects without needing to know their on-disk location.
//
// Tradeoff: this bypasses the per-project `post:` hook and leaves the
// other project's `.raioz.state.json` stale. The next `raioz up` in that
// repo reconciles via state-vs-docker diff. Acceptable for the conflict-
// resolution use case; users who want a clean down should `cd` to the
// other project and run `raioz down` themselves.
func StopProjectContainers(ctx context.Context, projectName string) ([]string, error) {
	if projectName == "" {
		return nil, fmt.Errorf("project name is required")
	}

	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(
		ctx, exectimeout.DockerComposeDownTimeout)
	defer cancel()

	psCmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "ps",
		"--filter", "label="+naming.LabelProject+"="+projectName,
		"--format", "{{.Names}}")
	psOut, err := psCmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return nil, fmt.Errorf("docker ps timed out after %v",
				exectimeout.DockerComposeDownTimeout)
		}
		return nil, fmt.Errorf("docker ps failed for project %q: %w",
			projectName, err)
	}

	var names []string
	for _, line := range strings.Split(string(psOut), "\n") {
		n := strings.TrimSpace(line)
		if n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}

	// `docker rm -f` stops + removes in one shot, mirroring what `compose
	// down` does for these containers. Pass everything in one invocation
	// so partial failures still report correctly.
	rmArgs := append([]string{"rm", "-f"}, names...)
	rmCmd := exec.CommandContext(timeoutCtx, runtime.Binary(), rmArgs...)
	if out, err := rmCmd.CombinedOutput(); err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return nil, fmt.Errorf("docker rm timed out after %v",
				exectimeout.DockerComposeDownTimeout)
		}
		return nil, fmt.Errorf("docker rm failed for project %q: %w "+
			"(output: %s)", projectName, err, strings.TrimSpace(string(out)))
	}
	return names, nil
}
