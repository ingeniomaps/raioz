package docker

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestParseExposedPorts(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr string
	}{
		{
			name: "single TCP port",
			raw:  `{"5432/tcp":{}}`,
			want: 5432,
		},
		{
			name: "multiple TCP ports picks lowest",
			raw:  `{"443/tcp":{},"80/tcp":{}}`,
			want: 80,
		},
		{
			name: "mixed TCP and UDP picks TCP",
			raw:  `{"5432/tcp":{},"5432/udp":{}}`,
			want: 5432,
		},
		{
			name:    "UDP only rejected",
			raw:     `{"53/udp":{}}`,
			wantErr: "no TCP port",
		},
		{
			name: "unqualified port treated as TCP",
			raw:  `{"8080":{}}`,
			want: 8080,
		},
		{
			name:    "empty object",
			raw:     `{}`,
			wantErr: "no ExposedPorts declared",
		},
		{
			name:    "null",
			raw:     `null`,
			wantErr: "no ExposedPorts declared",
		},
		{
			name:    "empty string",
			raw:     ``,
			wantErr: "no ExposedPorts declared",
		},
		{
			name:    "malformed JSON",
			raw:     `{oops`,
			wantErr: "unmarshal",
		},
		{
			name:    "non-numeric port ignored",
			raw:     `{"http/tcp":{}}`,
			wantErr: "no TCP port",
		},
		{
			name: "non-numeric + valid picks valid",
			raw:  `{"http/tcp":{},"80/tcp":{}}`,
			want: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExposedPorts(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (port=%d)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("port = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetImageExposedPort_CachesResults(t *testing.T) {
	// Reset cache so prior tests don't shadow this one.
	exposedPortCache = sync.Map{}

	// Seed the cache manually — we can't rely on docker being available
	// in unit tests. This proves the cache short-circuits the subprocess
	// call path.
	exposedPortCache.Store("postgres:16", exposedPortEntry{port: 5432, err: nil})

	port, err := GetImageExposedPort(context.Background(), "postgres:16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 5432 {
		t.Errorf("port = %d, want 5432", port)
	}
}

func TestGetImageExposedPort_CachesErrors(t *testing.T) {
	exposedPortCache = sync.Map{}

	// A cached error must be returned as-is; we don't want every proxy
	// build to re-shell out for an image we already know has no EXPOSE.
	sentinel := context.Canceled
	exposedPortCache.Store("bogus:latest", exposedPortEntry{port: 0, err: sentinel})

	_, err := GetImageExposedPort(context.Background(), "bogus:latest")
	if err != sentinel {
		t.Errorf("expected cached sentinel error, got %v", err)
	}
}
