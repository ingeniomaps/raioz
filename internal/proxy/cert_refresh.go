package proxy

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/runtime"
)

// mountedCertCovers reports whether the cert currently bind-mounted into the
// running proxy already carries domain, *.domain and every wantSAN. When it
// does a cheap reload suffices; when it doesn't — or the mount can't be read —
// the caller recreates the container so the read-only /certs mount is swapped.
//
// Coverage (not path identity) is the test on purpose: after a broader cert is
// minted under one per-domain dir, an alternate-domain re-up whose own dir
// already holds an equally broad cert must NOT trigger a needless recreate.
func (m *Manager) mountedCertCovers(ctx context.Context, containerName string, wantSANs []string) bool {
	src := m.mountedCertSource(ctx, containerName)
	if src == "" {
		return false
	}
	return certMatchesDomain(filepath.Join(src, certFileName), m.domain, wantSANs)
}

// mountedCertSource returns the host path bind-mounted at /certs inside the
// running container, or "" when there is no such mount or inspect fails.
func (m *Manager) mountedCertSource(ctx context.Context, containerName string) string {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", `{{range .Mounts}}{{if eq .Destination "/certs"}}{{.Source}}{{end}}{{end}}`,
		containerName)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// forceRemoveContainer removes a container even when it is running. Used to
// recreate the proxy when its read-only cert mount went stale — removeStale-
// Container deliberately skips running containers, so it can't do this.
func (m *Manager) forceRemoveContainer(ctx context.Context, containerName string) error {
	rm := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", containerName)
	if out, err := rm.CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm -f %s: %w\n%s", containerName, err, string(out))
	}
	return nil
}
