package docker

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/runtime"
)

func TestQuietPullArgs(t *testing.T) {
	notATerminal, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer notATerminal.Close()

	prev := runtime.Binary()
	t.Cleanup(func() { runtime.SetBinary(prev) })

	runtime.SetBinary("docker")
	if got := quietPullArgs(notATerminal); len(got) != 1 || got[0] != "--quiet-pull" {
		t.Errorf("output to a file must pull quietly, got %v", got)
	}
	runtime.SetBinary("podman")
	if got := quietPullArgs(notATerminal); got != nil {
		t.Errorf("another runtime keeps its defaults, got %v", got)
	}
}
