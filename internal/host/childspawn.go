package host

import (
	"os/exec"
	"syscall"
)

// AttachPdeathsig wires the child's parent-death signal so the kernel
// SIGTERMs the spawn when the raioz parent dies, clean exit or KILL.
// Portable wrapper around prctl(PR_SET_PDEATHSIG); a no-op on macOS
// and Windows. Pair with context cancellation for the cross-platform
// half (ADR-026).
//
// Initializes SysProcAttr if nil so callers stop reinventing the
// two-line dance at every spawn site.
func AttachPdeathsig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	setPdeathsig(cmd.SysProcAttr)
}
