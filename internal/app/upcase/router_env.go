package upcase

import (
	"context"
	"os"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/protocol"
)

// routerActiveFromEnv reports whether this sub-up is running under an
// external router project (ADR-037). Truthy: "1", "true", "yes"
// (case-insensitive); anything else, including empty, is false.
func routerActiveFromEnv() bool {
	switch os.Getenv(protocol.RouterActive) {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	}
	return false
}

// shouldSuppressBundledProxy reports whether the gate around
// `startProxy` should short-circuit because an external router owns
// edge routing. True iff `RAIOZ_ROUTER_ACTIVE=1` is in the env AND
// `--router-off` was NOT passed — the flag must override an inherited
// env var so the bundled Caddy can run for debugging.
func shouldSuppressBundledProxy(routerOff bool) bool {
	return !routerOff && routerActiveFromEnv()
}

// maybeStartProxy is the gate around startProxy. It honors `proxy: true`,
// the presence of a ProxyManager, and ADR-037's RAIOZ_ROUTER_ACTIVE
// suppression. When an external router project handles edge routing the
// bundled Caddy is skipped with a single info line. routerOff overrides
// the env-var suppression so `raioz up --router-off` can recover the
// bundled Caddy after a leaked shell env var.
func (uc *UseCase) maybeStartProxy(
	ctx context.Context,
	deps *models.Deps,
	detections DetectionMap,
	serviceNames []string,
	networkName string,
	routerOff bool,
) error {
	if !deps.Proxy || uc.deps.ProxyManager == nil {
		return nil
	}
	if shouldSuppressBundledProxy(routerOff) {
		output.PrintInfo(i18n.T("up.proxy.router_active"))
		return nil
	}
	return uc.startProxy(ctx, deps, detections, serviceNames, networkName)
}
