package proxy

import (
	"context"
	"os/exec"

	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// ensureDataVolume creates Caddy's /data volume with the raioz labels
// before the container mounts it. Left to `docker run -v`, the volume is
// created bare: nothing says raioz owns it, so `raioz clean --volumes`
// walks past it and it outlives every project that used the proxy.
//
// Creating a volume that already exists is a no-op, and Docker cannot
// label one after the fact — a volume from an older raioz stays bare.
// Best-effort: on failure `docker run` still creates the volume itself.
func (m *Manager) ensureDataVolume(ctx context.Context) {
	labelProject := m.projectName
	if m.isWorkspaceShared() {
		labelProject = ""
	}
	args := []string{"volume", "create"}
	for k, v := range naming.Labels(m.workspaceName, labelProject, "proxy", naming.KindProxy) {
		args = append(args, "--label", k+"="+v)
	}
	args = append(args, m.caddyVolume())
	if err := exec.CommandContext(ctx, runtime.Binary(), args...).Run(); err != nil {
		logging.WarnWithContext(ctx, "Could not pre-create the proxy data volume",
			"volume", m.caddyVolume(), "error", err.Error())
	}
}
