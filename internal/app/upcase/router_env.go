package upcase

import (
	"context"
	"os"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// envRouterActive is set by the meta runner (ADR-037) when a router
// project is in charge of edge routing for this workspace. The
// consumer-side upcase reads it to suppress the bundled Caddy.
//
// Defined here (not in internal/app) so the upcase package has no
// upward dependency on the meta orchestrator.
const envRouterActive = "RAIOZ_ROUTER_ACTIVE"

// routerActiveFromEnv reports whether this sub-up is running under an
// external router project. Truthy values: "1", "true", "yes" (case-
// insensitive). Anything else, including the empty string, is false.
func routerActiveFromEnv() bool {
	switch os.Getenv(envRouterActive) {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	}
	return false
}

// maybeStartProxy is the gate around startProxy. It honors `proxy: true`,
// the presence of a ProxyManager, and ADR-037's RAIOZ_ROUTER_ACTIVE
// suppression. When an external router project handles edge routing the
// bundled Caddy is skipped with a single info line.
func (uc *UseCase) maybeStartProxy(
	ctx context.Context,
	deps *models.Deps,
	detections DetectionMap,
	serviceNames []string,
	networkName string,
) error {
	if !deps.Proxy || uc.deps.ProxyManager == nil {
		return nil
	}
	if routerActiveFromEnv() {
		output.PrintInfo(i18n.T("up.proxy.router_active"))
		return nil
	}
	return uc.startProxy(ctx, deps, detections, serviceNames, networkName)
}
