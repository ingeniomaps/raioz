//go:build !linux

package upcase

import "syscall"

// No-op on non-Linux. macOS would need kqueue parent-PID monitoring
// and Windows would need Job Objects with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// — both deferred. Context cancellation (ADR-026, signal handling)
// is the portable half of the orphan-prevention story.
func setPdeathsig(_ *syscall.SysProcAttr) {}
