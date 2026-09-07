package docker

import (
	"testing"
)

func TestLogsOptions(t *testing.T) {
	// Test default values
	opts := LogsOptions{
		Follow:   false,
		Tail:     0,
		Services: []string{},
	}

	if opts.Follow {
		t.Error("LogsOptions.Follow should be false by default")
	}
	if opts.Tail != 0 {
		t.Errorf("LogsOptions.Tail = %d, want 0", opts.Tail)
	}
	if len(opts.Services) != 0 {
		t.Errorf("LogsOptions.Services = %d, want 0", len(opts.Services))
	}
}
