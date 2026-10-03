package host

import (
	"time"
)

// ProcessInfo contains information about a running host process
type ProcessInfo struct {
	PID         int       `json:"pid"`
	Service     string    `json:"service"`
	Command     string    `json:"command"`
	StopCommand string    `json:"stopCommand,omitempty"` // Optional custom stop command
	ComposePath string    `json:"composePath,omitempty"` // Path to docker-compose.yml if service uses docker-compose
	StartTime   time.Time `json:"startTime"`
}

// startSettleWindow is how long a host runner waits after a successful
// cmd.Start() to make sure the process did not die immediately. If the
// process exits inside this window we treat the start as a failure and
// surface the stderr tail so the user sees why.
//
// Background: cmd.Start() returns nil for any process that fork+exec'd
// successfully — even if it crashes 5 ms later (port already bound,
// missing config, etc). Without this guard `raioz status` then reports
// "running" while the service is already dead.
//
// Exposed as a package var (not a const) so tests can shrink it.
var startSettleWindow = 500 * time.Millisecond
