package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// dashboardActionTimeout bounds a host-service action started from the
// dashboard: a restart can sit in the launcher wait for a minute.
const dashboardActionTimeout = 2 * time.Minute

// dashboardVerbs maps a dashboard action to the raioz command that does it.
var dashboardVerbs = map[string]string{"restart": "restart", "stop": "down"}

// selfBinaryFn resolves the raioz binary the dashboard re-invokes. A
// variable so tests point it at a stand-in: under `go test` the running
// executable is the test binary.
var selfBinaryFn = os.Executable

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// HostServiceAction restarts or stops one host service on behalf of the
// dashboard by running `raioz restart <service>` / `raioz down <service>`
// as a child process (ADR-044). The command takes the workspace lock,
// rewrites the recorded PID and reports a lock held by someone else, so
// the dashboard gets the CLI's coordination instead of a second copy of
// it — and the command's output stays off the dashboard's screen.
func HostServiceAction(ctx context.Context, configPath, action, service string) error {
	verb, ok := dashboardVerbs[action]
	if !ok {
		return fmt.Errorf("unknown dashboard action %q", action)
	}
	self, err := selfBinaryFn()
	if err != nil {
		return fmt.Errorf("resolve raioz binary: %w", err)
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, dashboardActionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, verb, "--file", abs, service)
	cmd.Dir = filepath.Dir(abs)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if reason := lastOutputLine(string(out)); reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return fmt.Errorf("raioz %s %s: %w", verb, service, err)
}

// lastOutputLine returns the last non-empty line of a command's output,
// without colour codes: the one line the dashboard has room to show.
func lastOutputLine(out string) string {
	lines := strings.Split(ansiSequence.ReplaceAllString(out, ""), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return strings.TrimSpace(strings.TrimPrefix(line, "[error]"))
		}
	}
	return ""
}
