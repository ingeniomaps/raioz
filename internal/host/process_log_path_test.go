package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/workspace"
)

// This path used to write host logs to <workspace>/logs/host/<svc>.stdout.log
// while `up`'s runner wrote to naming.LogFile — and every reader (`raioz
// logs`, up's streaming, the early-exit tail) reads naming.LogFile. A
// restarted service therefore stopped updating the file anyone would open,
// and the stale one still held a successful startup from days earlier.
func TestStartServiceWritesToSharedLogPath(t *testing.T) {
	skipIfNoBinary(t, "sh")

	prev := startSettleWindow
	startSettleWindow = 200 * time.Millisecond
	t.Cleanup(func() { startSettleWindow = prev })

	// No `.sh` suffix: that routes through shouldWaitForCommand's
	// synchronous path, which is a different branch of this function.
	dir := t.TempDir()
	script := filepath.Join(dir, "printer")
	body := "#!/bin/sh\necho log-path-marker\nsleep 5\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	wsRoot := t.TempDir()
	ws := &workspace.Workspace{Root: wsRoot}
	deps := &models.Deps{Project: models.Project{Name: "logpath"}}
	svc := models.Service{
		Source: models.SourceConfig{Kind: "local", Path: ".", Command: script},
	}

	info, err := StartService(context.Background(), ws, deps, "svc", svc, dir)
	if err != nil {
		t.Fatalf("StartService: %v", err)
	}
	t.Cleanup(func() {
		if info != nil && info.PID > 0 {
			_ = KillProcessTree(info.PID)
		}
	})

	shared := naming.LogFile("logpath", "svc")
	content, err := os.ReadFile(shared)
	if err != nil {
		t.Fatalf("expected the log at the shared path %s: %v", shared, err)
	}
	if !strings.Contains(string(content), "log-path-marker") {
		t.Errorf("log at %s = %q, want the service output in it", shared, content)
	}

	// The old private path must be gone, not merely unused: a directory
	// that still gets created is a directory a dev will still open.
	legacy := filepath.Join(wsRoot, "logs", "host")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy log dir %s still exists (err=%v), want it never created", legacy, err)
	}
}
