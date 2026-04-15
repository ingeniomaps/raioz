package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout reroutes os.Stdout for the duration of fn and returns whatever
// was written. Used to assert visible output without coupling to the exact
// ANSI sequence emitted by the helpers.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close()
	<-done
	return buf.String()
}

func TestPrintWorkspaceCreated_NoOp(t *testing.T) {
	out := captureStdout(t, PrintWorkspaceCreated)
	if out != "" {
		t.Errorf("expected silent, got %q", out)
	}
}

func TestPrintProgressStep(t *testing.T) {
	out := captureStdout(t, func() { PrintProgressStep(2, 5, "starting api") })
	if !strings.Contains(out, "[2/5]") || !strings.Contains(out, "starting api") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPrintProgressDone(t *testing.T) {
	out := captureStdout(t, func() { PrintProgressDone("api ready") })
	if !strings.Contains(out, "[ok]") || !strings.Contains(out, "api ready") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPrintProgressError(t *testing.T) {
	out := captureStdout(t, func() { PrintProgressError("api failed") })
	if !strings.Contains(out, "[fail]") || !strings.Contains(out, "api failed") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPrintKeyValue(t *testing.T) {
	out := captureStdout(t, func() { PrintKeyValue("port", "5432") })
	if !strings.Contains(out, "port") || !strings.Contains(out, "5432") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPrintTableHeaderAndRow(t *testing.T) {
	out := captureStdout(t, func() {
		PrintTableHeader("NAME", "STATUS")
		PrintTableRow("api", "running")
	})
	for _, want := range []string{"NAME", "STATUS", "api", "running"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestPrintEmptyState(t *testing.T) {
	out := captureStdout(t, func() { PrintEmptyState("services") })
	if !strings.Contains(out, "no services") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPrintPrompt(t *testing.T) {
	out := captureStdout(t, func() { PrintPrompt("? ") })
	if out != "? " {
		t.Errorf("got %q, want %q", out, "? ")
	}
}

func TestGetTableWriter_Singleton(t *testing.T) {
	a := getTableWriter()
	b := getTableWriter()
	if a != b {
		t.Error("getTableWriter should return the same instance on repeated calls")
	}
}
