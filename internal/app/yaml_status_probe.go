package app

import (
	"context"

	"raioz/internal/host"
)

// Status values StatusYAML reports for a host service.
const (
	statusRunning  = "running"
	statusStarting = "starting"
	statusStopped  = "stopped"
)

// hostStatusPortProbe is the probe hostServiceStatus consults. Package var
// so tests can decide the answer without binding a real socket.
var hostStatusPortProbe = host.IsPortListening

// hostServiceStatus reports the status of a host service whose recorded PID
// is still alive.
//
// A live PID only proves the WRAPPER exists. `command: yarn start:dev` (or
// npm / make / any launcher script) keeps the parent alive after the child
// that actually binds the port has died, so a PID-only check paints the
// panel green over a stack that is serving nothing. When the service
// declares `port:`, that socket is the signal that answers the real
// question — is anybody attending?
//
// The `starting` state is deliberate rather than folding into `stopped`: a
// service compiling TypeScript is alive and not yet listening, and calling
// it stopped would be the symmetric lie. Reporting the ambiguity is the
// point — it tells the user to look, which `running` did not.
//
// Services with no `port:` keep the PID answer: raioz allocates their port
// at up-time without recording it, so there is nothing better to probe.
func hostServiceStatus(ctx context.Context, port int) string {
	if port <= 0 {
		return statusRunning
	}
	if hostStatusPortProbe(ctx, port) {
		return statusRunning
	}
	return statusStarting
}
