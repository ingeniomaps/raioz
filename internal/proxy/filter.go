package proxy

import "raioz/internal/netutil"

// IsNonHTTPImage re-exports netutil.IsNonHTTPImage. Canonical home is
// internal/netutil (ADR-029) — the classifier is pure string
// manipulation that app/cli call sites should not need to reach
// through the proxy package.
var IsNonHTTPImage = netutil.IsNonHTTPImage
