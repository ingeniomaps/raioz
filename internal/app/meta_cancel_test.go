package app

import (
	"context"
	"testing"
	"time"
)

// Issue 020-meta: cmd.Cancel must NOT kill the sub-process when its
// ctx is canceled via the normal-path subCancel (deferred at the end
// of runSingle after cmd.Run() returns ok). Only DeadlineExceeded —
// the actual timeout case — must trigger Kill. Without this gate, the
// runtime's cancel path raced against in-flight launcher grandchildren
// of the sub-process and killed them mid-deploy.
func TestMetaRunner_BuildSubCmd_CancelIsNoopOnNormalCancel(t *testing.T) {
	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "p")
	r := &MetaRunner{Binary: bin}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := r.buildSubCmd(ctx, bin, "up", cfg.Projects[0], nil, nil, nil, nil)

	// Trigger a manual cancel (the normal-path case: subCancel deferred
	// after cmd.Run() returned ok).
	cancel()

	// cmd.Cancel must be invocable without panicking and must NOT
	// touch a process that hasn't been started yet. The contract is:
	// for context.Canceled, return nil — do not call Process.Kill.
	if err := cmd.Cancel(); err != nil {
		t.Errorf("Cancel on manual ctx cancel: got %v, want nil "+
			"(only DeadlineExceeded should attempt Kill)", err)
	}
}

// The DeadlineExceeded path must still attempt the kill so genuinely
// hung sub-projects (past RAIOZ_META_SUB_TIMEOUT) don't pin the meta
// run indefinitely.
func TestMetaRunner_BuildSubCmd_CancelKillsOnDeadlineExceeded(t *testing.T) {
	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "p")
	r := &MetaRunner{Binary: bin}

	ctx, cancel := context.WithDeadline(
		context.Background(), time.Now().Add(-time.Second),
	)
	defer cancel()

	cmd := r.buildSubCmd(ctx, bin, "up", cfg.Projects[0], nil, nil, nil, nil)

	// Process hasn't been Start()ed in this unit test, so cmd.Process
	// is nil and Kill would panic. The point of the test is the
	// branching logic: Cancel takes the DeadlineExceeded branch (vs
	// the normal-cancel branch which returns nil silently).
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from Process.Kill (Process is nil); " +
				"branch took the no-op path instead of the kill path")
		}
	}()
	_ = cmd.Cancel()
}
