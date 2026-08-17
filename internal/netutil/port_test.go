package netutil

import (
	"fmt"
	"net"
	"testing"
)

// The port allocator asks this before publishing, so a wrong answer
// either steals a port from another process or refuses a free one.
func TestCheckPortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind a local port: %v", err)
	}
	defer func() { _ = ln.Close() }()
	busy := ln.Addr().(*net.TCPAddr).Port

	inUse, err := CheckPortInUse(fmt.Sprintf("%d", busy))
	if err != nil {
		t.Fatalf("CheckPortInUse: %v", err)
	}
	if !inUse {
		t.Errorf("port %d is bound by this test but reported free", busy)
	}

	// Host:container form: only the host half is probed.
	inUse, err = CheckPortInUse(fmt.Sprintf("%d:5432", busy))
	if err != nil {
		t.Fatalf("CheckPortInUse with mapping: %v", err)
	}
	if !inUse {
		t.Errorf("host port %d in a mapping reported free", busy)
	}

	_ = ln.Close()
	if inUse, err = CheckPortInUse(fmt.Sprintf("%d", busy)); err != nil || inUse {
		t.Errorf("after closing the listener: inUse=%v err=%v, want free", inUse, err)
	}
}

func TestCheckPortInUseRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "abc", ":5432", "8080a"} {
		if _, err := CheckPortInUse(bad); err == nil {
			t.Errorf("CheckPortInUse(%q) = nil error, want a parse failure", bad)
		}
	}
}
