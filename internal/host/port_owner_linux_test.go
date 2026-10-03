//go:build linux

package host

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestProcessGroupListensOn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	own := syscall.Getpgrp()

	// A second group that holds no socket on that port.
	other := exec.Command("sleep", "30")
	SetNewProcessGroup(other)
	if err := other.Start(); err != nil {
		t.Skipf("sleep not available: %v", err)
	}
	t.Cleanup(func() { _ = other.Process.Kill(); _, _ = other.Process.Wait() })

	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freePort := free.Addr().(*net.TCPAddr).Port
	free.Close()

	tests := []struct {
		name string
		pgid int
		port int
		want bool
	}{
		{"own group holds its listener", own, port, true},
		{"another group does not", other.Process.Pid, port, false},
		{"nobody listens on a free port", own, freePort, false},
		{"invalid pgid", 0, port, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			held, known := ProcessGroupListensOn(tc.pgid, tc.port)
			if !known {
				t.Skip("/proc not readable here")
			}
			if held != tc.want {
				t.Errorf("held = %v, want %v", held, tc.want)
			}
		})
	}
}

// A child that listens in its own group — the shape of a host service
// (`sh -c` leader, the real server one level down).
func TestProcessGroupListensOn_Grandchild(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	cmd := exec.Command("sh", "-c", py+" -m http.server "+strconv.Itoa(port)+" --bind 127.0.0.1")
	cmd.Stdout, cmd.Stderr = os.NewFile(0, os.DevNull), nil
	SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = KillProcessTree(cmd.Process.Pid); _, _ = cmd.Process.Wait() })

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if held, known := ProcessGroupListensOn(cmd.Process.Pid, port); known && held {
			ports, _ := GroupListeningPorts(cmd.Process.Pid)
			if len(ports) != 1 || ports[0] != port {
				t.Errorf("GroupListeningPorts = %v, want [%d]", ports, port)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("group %d never seen listening on %d", cmd.Process.Pid, port)
}
