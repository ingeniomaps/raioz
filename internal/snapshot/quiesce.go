package snapshot

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/runtime"
)

// runningContainersUsing returns the names of the running containers that
// mount volume. A package var so tests can answer without a daemon.
var runningContainersUsing = func(volume string) ([]string, error) {
	out, err := exec.Command(runtime.Binary(), "ps",
		"--filter", "volume="+volume, "--format", "{{.Names}}").Output()
	if err != nil {
		return nil, fmt.Errorf("list containers using volume %s: %w", volume, err)
	}
	return strings.Fields(string(out)), nil
}

// containerAction runs `docker stop|start` on the given containers.
var containerAction = func(action string, containers []string) error {
	args := append([]string{action}, containers...)
	if out, err := exec.Command(runtime.Binary(), args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", action, strings.Join(containers, " "), err, string(out))
	}
	return nil
}

// quiesce stops every running container that mounts one of the volumes and
// returns the function that starts them again.
//
// A restore under a live container is not a restore: the engine holds the
// data in memory and writes it back over the restored files the next time
// it flushes or shuts down, and the command would still report success.
func quiesce(volumes []VolumeSnapshot) (resume func(), err error) {
	seen := map[string]bool{}
	var containers []string
	for _, vol := range volumes {
		using, err := runningContainersUsing(vol.VolumeName)
		if err != nil {
			return nil, err
		}
		for _, c := range using {
			if !seen[c] {
				seen[c] = true
				containers = append(containers, c)
			}
		}
	}
	if len(containers) == 0 {
		return func() {}, nil
	}
	sort.Strings(containers)

	output.PrintInfo(i18n.T("snapshot.stopping_for_restore", strings.Join(containers, ", ")))
	if err := containerAction("stop", containers); err != nil {
		return nil, err
	}
	return func() {
		if err := containerAction("start", containers); err != nil {
			output.PrintWarning(i18n.T("snapshot.restart_failed", strings.Join(containers, ", "), err))
			return
		}
		output.PrintInfo(i18n.T("snapshot.restarted_after_restore", strings.Join(containers, ", ")))
	}, nil
}
