package app

import (
	"context"
	"os/exec"

	"raioz/internal/proxy"
	"raioz/internal/runtime"
)

// checkCaddy verifies that the Caddy Docker image is available.
func (uc *DoctorUseCase) checkCaddy(_ context.Context) DoctorCheck {
	cmd := exec.Command(runtime.Binary(), "image", "inspect", "caddy:latest")
	if err := cmd.Run(); err != nil {
		return DoctorCheck{
			Name:    "Caddy",
			Status:  "warning",
			Message: "caddy:latest image not found (will be pulled on first raioz up with proxy)",
		}
	}
	return DoctorCheck{
		Name:    "Caddy",
		Status:  "ok",
		Message: "caddy:latest image available",
	}
}

// checkMkcert verifies that mkcert is installed for local HTTPS.
func (uc *DoctorUseCase) checkMkcert(_ context.Context) DoctorCheck {
	if !proxy.HasMkcert() {
		return DoctorCheck{
			Name:    "mkcert",
			Status:  "warning",
			Message: "not installed (proxy will work without HTTPS). Install: https://github.com/FiloSottile/mkcert",
		}
	}
	return DoctorCheck{
		Name:    "mkcert",
		Status:  "ok",
		Message: "installed (local HTTPS available)",
	}
}

// checkRuntimes verifies common runtimes are available.
func (uc *DoctorUseCase) checkRuntimes(_ context.Context) DoctorCheck {
	runtimes := map[string]string{
		"node": "Node.js",
		"go":   "Go",
		"make": "Make",
	}

	var available []string
	for cmd, name := range runtimes {
		if _, err := exec.LookPath(cmd); err == nil {
			available = append(available, name)
		}
	}

	if len(available) == 0 {
		return DoctorCheck{
			Name:    "Runtimes",
			Status:  "warning",
			Message: "no host runtimes detected (node, go, make). Host services won't work.",
		}
	}

	msg := ""
	for i, r := range available {
		if i > 0 {
			msg += ", "
		}
		msg += r
	}

	return DoctorCheck{
		Name:    "Runtimes",
		Status:  "ok",
		Message: msg,
	}
}
