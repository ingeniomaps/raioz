package docker

import (
	"errors"
	"testing"

	"raioz/internal/domain/interfaces"
)

// daemonDownSignatures must wrap to interfaces.ErrDaemonUnreachable.
// Other failures must pass through without the sentinel.
func TestWrapDaemonError(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		err       error
		wantSent  bool
		wantNil   bool
	}{
		{
			name:    "nil err returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:     "daemon down via output",
			output:   "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
			err:      errors.New("exit status 1"),
			wantSent: true,
		},
		{
			name:     "connection refused via output",
			output:   "dial tcp 127.0.0.1:2375: connect: connection refused",
			err:      errors.New("exit status 1"),
			wantSent: true,
		},
		{
			name:     "case insensitive match",
			output:   "CANNOT CONNECT TO THE DOCKER DAEMON",
			err:      errors.New("exit status 1"),
			wantSent: true,
		},
		{
			name:     "signature in err message itself",
			output:   "",
			err:      errors.New("dial unix /var/run/docker.sock: connect: no such file or directory"),
			wantSent: true,
		},
		{
			name:     "unrelated stderr passes through",
			output:   "Error response from daemon: container is not running",
			err:      errors.New("exit status 1"),
			wantSent: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapDaemonError("docker test", []byte(tt.output), tt.err)
			if tt.wantNil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if errors.Is(got, interfaces.ErrDaemonUnreachable) != tt.wantSent {
				t.Errorf("errors.Is(sentinel) = %v, want %v; full err: %v",
					!tt.wantSent, tt.wantSent, got)
			}
		})
	}
}
