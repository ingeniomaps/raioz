package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTerminal(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()

	regular, err := os.Create(filepath.Join(t.TempDir(), "f"))
	if err != nil {
		t.Fatal(err)
	}
	defer regular.Close()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	tests := []struct {
		name string
		f    *os.File
	}{
		{"nil file", nil},
		{"the null device is a char device, not a terminal", null},
		{"regular file", regular},
		{"pipe", r},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if IsTerminal(tc.f) {
				t.Error("IsTerminal = true, want false")
			}
		})
	}
}
