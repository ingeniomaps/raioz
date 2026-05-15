package app

import (
	"errors"
	"testing"
)

// TestIsDockerUnreachable pins the heuristic match against the
// docker CLI / engine API signatures that signal "daemon down".
// Issue 071: when this matches, --force-state-cleanup is offered
// in the error suggestion.
func TestIsDockerUnreachable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"daemon down (CLI)", errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"), true},
		{"connection refused", errors.New("dial tcp 127.0.0.1:2375: connect: connection refused"), true},
		{"socket missing", errors.New("dial unix /var/run/docker.sock: connect: no such file or directory"), true},
		{"unrelated", errors.New("project label conflict"), false},
		{"case insensitive", errors.New("CANNOT CONNECT TO THE DOCKER DAEMON"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDockerUnreachable(tt.err); got != tt.want {
				t.Errorf("isDockerUnreachable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
