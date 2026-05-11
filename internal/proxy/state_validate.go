package proxy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	rerrors "raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/naming"
)

// assertProxyDirWritable defends against the "Docker bind-mount source
// auto-created as root" trap (issue 015). Symptoms in the field:
//
//   - WriteFile returns `is a directory` because Caddyfile is now a dir.
//   - MkdirAll returns `permission denied` because the parent is root-owned.
//
// The recovery prompt's Retry option can't fix either one — only deleting
// the corrupt tree (which itself needs `sudo` because it's root-owned)
// recovers. This pre-flight check inspects the dir before generateCaddyfile
// and surfaces a structured error pointing at the exact remediation
// command. Failures are advisory, not destructive: we never touch the dir
// here, just refuse to continue.
func (m *Manager) assertProxyDirWritable() error {
	dir := m.activeProxyDir()
	if dir == "" {
		return nil
	}
	caddyfile := filepath.Join(dir, "Caddyfile")

	// Caddyfile must be a regular file when present. Docker's bind-mount
	// auto-create is the most common way it ends up as a directory, but
	// any external tool that pre-creates the path the wrong way trips
	// the same failure.
	if info, err := os.Stat(caddyfile); err == nil && !info.Mode().IsRegular() {
		return proxyDirCorruptedError(dir, fmt.Errorf("%s is not a regular file", caddyfile))
	}

	// Probe writability with a real create+remove. Stat-mode bits lie on
	// shared filesystems (NFS, fuse, bind mounts with squash_root, etc.)
	// — a successful temp file is the only honest signal that the next
	// WriteFile call will succeed.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return proxyDirCorruptedError(dir, err)
	}
	probe, err := os.CreateTemp(dir, ".raioz-probe-*")
	if err != nil {
		return proxyDirCorruptedError(dir, err)
	}
	probePath := probe.Name()
	_ = probe.Close()
	if err := os.Remove(probePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return proxyDirCorruptedError(dir, err)
	}
	return nil
}

// activeProxyDir returns the same dir generateCaddyfile would write to.
// Mirrors caddyfile.go's branch on workspace-shared mode.
func (m *Manager) activeProxyDir() string {
	if m.isWorkspaceShared() {
		return naming.WorkspaceProxyDir()
	}
	return naming.ProxyDir(m.networkName)
}

// proxyDirCorruptedError builds the user-facing error for issue 015.
// The suggestion includes the exact path so the user can paste the
// `sudo rm -rf` command verbatim.
func proxyDirCorruptedError(dir string, cause error) error {
	return rerrors.New(
		rerrors.ErrCodePermissionDenied,
		i18n.T("error.proxy_dir_corrupted", dir),
	).WithSuggestion(
		i18n.T("error.proxy_dir_corrupted_suggestion", dir),
	).WithContext("dir", dir).WithError(cause)
}
