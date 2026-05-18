package proxy

import "raioz/internal/netutil"

// DefaultProxyIP re-exports netutil.DefaultProxyIP for callers that
// already live inside the proxy package. Canonical home is
// internal/netutil (ADR-029) so app/cli don't need to import proxy for
// a pure subnet calculation.
var DefaultProxyIP = netutil.DefaultProxyIP

// ValidateProxyIP re-exports netutil.ValidateProxyIP. Same rationale.
var ValidateProxyIP = netutil.ValidateProxyIP
