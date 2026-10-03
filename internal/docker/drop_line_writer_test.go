package docker

import (
	"bytes"
	"testing"
)

func TestDropLineWriter(t *testing.T) {
	var out bytes.Buffer
	w := &dropLineWriter{w: &out, drop: "No resource found"}

	_, _ = w.Write([]byte(" Container a  Stopp"))
	_, _ = w.Write([]byte("ed\nWarning: No resource found to remove for project \"x\".\n Network n  Rem"))
	_, _ = w.Write([]byte("oved"))
	w.Flush()

	want := " Container a  Stopped\n Network n  Removed"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}
