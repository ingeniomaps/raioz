//go:build linux

package upcase

import (
	"net"
	"path/filepath"
	"syscall"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/state"
)

func TestOwnHostServicePID(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "raioz.yaml")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freePort := free.Addr().(*net.TCPAddr).Port
	free.Close()

	// The test binary's own process group stands in for a host service:
	// its leader is alive and one of its members holds the listener.
	pgid := syscall.Getpgrp()
	if err := state.SaveLocalState(projectDir, &models.LocalState{
		Project:  "bencha",
		HostPIDs: map[string]int{"web": pgid, "gone": 1 << 30},
	}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		conflict PortBindConflict
		config   string
		want     int
	}{
		{"recorded service holding the port", PortBindConflict{Kind: "service", Name: "web", Port: port}, configPath, pgid},
		{"recorded service not on that port", PortBindConflict{Kind: "service", Name: "web", Port: freePort}, configPath, 0},
		{"dead recorded pid", PortBindConflict{Kind: "service", Name: "gone", Port: port}, configPath, 0},
		{"service without a recorded pid", PortBindConflict{Kind: "service", Name: "api", Port: port}, configPath, 0},
		{"dependency conflicts are not host services", PortBindConflict{Kind: "dep", Name: "web", Port: port}, configPath, 0},
		{"no config path", PortBindConflict{Kind: "service", Name: "web", Port: port}, "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownHostServicePID(tc.conflict, tc.config); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
