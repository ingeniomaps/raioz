//go:build linux

package host

import "syscall"

// setPdeathsig wires PR_SET_PDEATHSIG = SIGTERM. The kernel signals
// the child as soon as its parent dies, so a SIGKILL on raioz reaps
// the spawn tree instead of orphaning it. ADR-026.
func setPdeathsig(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGTERM
}
