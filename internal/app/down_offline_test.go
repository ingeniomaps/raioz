package app

import (
	"errors"
	"fmt"
	"testing"

	"raioz/internal/domain/interfaces"
)

// Pins the sentinel match. The substring → sentinel translation
// lives in internal/docker; app only knows the typed error.
func TestIsDockerUnreachable(t *testing.T) {
	wrapped := fmt.Errorf("docker ps: %w: exit status 1", interfaces.ErrDaemonUnreachable)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sentinel-wrapped", wrapped, true},
		{"sentinel direct", interfaces.ErrDaemonUnreachable, true},
		{"unrelated", errors.New("project label conflict"), false},
		{"raw CLI prose without sentinel", errors.New("Cannot connect to the Docker daemon"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDockerUnreachable(tt.err); got != tt.want {
				t.Errorf("isDockerUnreachable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
