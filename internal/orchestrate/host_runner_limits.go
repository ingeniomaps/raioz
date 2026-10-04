package orchestrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// A host service is a process, not a container: the only thing that can
// cap it is the kernel's cgroups, and the way an unprivileged user gets a
// cgroup of their own is a transient systemd scope. `systemd-run --scope`
// creates it and then execs the command in place, so the PID raioz records,
// the process group it signals and the log file it hands over are the same
// as without a cap.

// hostLimitsUnavailableFn reports why a cap cannot be enforced on a host
// process here; empty means it can. A variable so tests decide the answer.
var hostLimitsUnavailableFn = hostLimitsUnavailable

var (
	hostLimitsOnce   sync.Once
	hostLimitsReason string
)

// hostLimitPrefix returns the argv that goes in front of a host service's
// command to cap it, or nil when the service declares no cap or the host
// cannot enforce one — in which case the service still starts, and the
// user is told it runs uncapped.
func hostLimitPrefix(ctx context.Context, svc interfaces.ServiceContext) []string {
	props := scopeProperties(svc.Resources)
	if len(props) == 0 {
		return nil
	}
	if reason := hostLimitsUnavailableFn(svc.Resources); reason != "" {
		output.PrintWarning(i18n.T("warning.host_resources_unenforced", svc.Name, reason))
		logging.WarnWithContext(ctx, "Host service runs without its declared cap",
			"service", svc.Name, "reason", reason)
		return nil
	}
	prefix := []string{"systemd-run", "--user", "--scope", "--quiet", "--collect"}
	for _, p := range props {
		prefix = append(prefix, "-p", p)
	}
	return append(prefix, "--")
}

// scopeProperties translates a `resources:` block into systemd resource
// control properties. MemorySwapMax=0 is what makes the memory figure a
// ceiling, the same reason the container path pins memory+swap.
func scopeProperties(res *models.Resources) []string {
	if res.IsZero() {
		return nil
	}
	var props []string
	if bytes, ok := res.MemoryBytes(); ok {
		props = append(props, "MemoryMax="+strconv.FormatInt(bytes, 10), "MemorySwapMax=0")
	}
	if res.CPUs > 0 {
		props = append(props, "CPUQuota="+strconv.FormatFloat(res.CPUs*100, 'f', -1, 64)+"%")
	}
	return props
}

// hostLimitsUnavailable checks, once per process, that a user scope with
// resource control works on this machine: Linux, systemd-run on PATH, the
// needed cgroup controllers delegated to the user manager, and a manager
// that answers.
func hostLimitsUnavailable(res *models.Resources) string {
	if goruntime.GOOS != "linux" {
		return i18n.T("host_limits.reason_os")
	}
	if missing := missingControllers(res, delegatedControllers()); missing != "" {
		return i18n.T("host_limits.reason_controller", missing)
	}
	hostLimitsOnce.Do(func() {
		if _, err := exec.LookPath("systemd-run"); err != nil {
			hostLimitsReason = i18n.T("host_limits.reason_systemd")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		probe := exec.CommandContext(ctx, "systemd-run", "--user", "--scope", "--quiet", "--collect", "--", "true")
		if err := probe.Run(); err != nil {
			hostLimitsReason = i18n.T("host_limits.reason_systemd")
		}
	})
	return hostLimitsReason
}

// delegatedControllers reads the cgroup controllers systemd handed to the
// user's manager; a scope can only limit what is listed there.
func delegatedControllers() string {
	uid := strconv.Itoa(os.Getuid())
	path := fmt.Sprintf("/sys/fs/cgroup/user.slice/user-%s.slice/user@%s.service/cgroup.controllers", uid, uid)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// missingControllers names the first controller res needs that delegated
// does not list.
func missingControllers(res *models.Resources, delegated string) string {
	have := map[string]bool{}
	for _, c := range strings.Fields(delegated) {
		have[c] = true
	}
	if res.Memory != "" && !have["memory"] {
		return "memory"
	}
	if res.CPUs > 0 && !have["cpu"] {
		return "cpu"
	}
	return ""
}
