package upcase

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdoutForLog(t *testing.T, fn func()) string {
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

func TestPrefixedCopy_PrintsLinesWithPrefix(t *testing.T) {
	src := strings.NewReader("line one\nline two\n")
	out := captureStdoutForLog(t, func() {
		prefixedCopy(context.Background(), src, "api", "", 0)
	})
	if !strings.Contains(out, "line one") || !strings.Contains(out, "line two") {
		t.Errorf("expected both lines in output, got %q", out)
	}
	if !strings.Contains(out, "api") {
		t.Errorf("expected service prefix in output, got %q", out)
	}
}
