//go:build !linux

package host

import "syscall"

// Non-Linux no-op. macOS would need kqueue parent-PID monitoring and
// Windows would need Job Objects; both deferred. Context cancellation
// covers the portable half (ADR-026).
func setPdeathsig(_ *syscall.SysProcAttr) {}
