//go:build linux

package upcase

import "syscall"

// setPdeathsig wires the child's parent-death signal so when the
// raioz parent process exits (clean or killed), the kernel sends
// SIGTERM to the spawned sibling. Without this, Ctrl+C on the parent
// leaves recursive `raioz up` children running with their own
// half-spawned containers. Issue 057 / ADR-026.
//
// Linux-only because `Pdeathsig` is a prctl(PR_SET_PDEATHSIG) wrapper.
// Non-Linux platforms fall back to context cancellation alone (the
// no-op companion is in sibling_spawn_other.go).
func setPdeathsig(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGTERM
}
