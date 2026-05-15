//go:build !linux

package app

import (
	"context"
	"testing"
)

// On non-Linux AttachPdeathsig is a no-op that still initializes
// SysProcAttr — the meta wiring just has to call it. The Linux test
// asserts the actual Pdeathsig signal.
func TestMetaRunner_RunSingleAttachesPdeathsig_NonLinux(t *testing.T) {
	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "router")
	r := &MetaRunner{Binary: bin}

	cmd := r.buildSubCmd(context.Background(), bin, "up",
		cfg.Projects[0], nil, nil, nil, nil)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil; AttachPdeathsig was not called")
	}
}
