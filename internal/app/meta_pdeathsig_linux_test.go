//go:build linux

package app

import (
	"context"
	"syscall"
	"testing"
)

// Meta sub-process must inherit ADR-026 Pdeathsig wiring; otherwise a
// SIGKILL on the meta parent orphans the router + consumer tree, each
// child still holding its own project lock.
func TestMetaRunner_RunSingleAttachesPdeathsig_Linux(t *testing.T) {
	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "router")
	r := &MetaRunner{Binary: bin}

	cmd := r.buildSubCmd(context.Background(), bin, "up",
		cfg.Projects[0], nil, nil, nil, nil)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil; AttachPdeathsig was not called")
	}
	if cmd.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Errorf("Pdeathsig = %v, want SIGTERM", cmd.SysProcAttr.Pdeathsig)
	}
}
