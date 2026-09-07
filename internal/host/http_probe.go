package host

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// httpProbeTimeout bounds a single request. A service that accepts the
// connection and then hangs must not stall the caller's round: the
// question is whether it answers now, not whether it eventually would.
const httpProbeTimeout = 2 * time.Second

// HealthURL builds the loopback address of a service's declared health
// endpoint. A missing leading slash is added; anything else in the path is
// taken as written, because a typo there is the user's to see.
func HealthURL(port int, endpoint string) string {
	if endpoint != "" && !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint)
}

// ProbeHTTP reports whether the URL answered without a server-side error.
//
// Any 2xx or 3xx counts. A health path that redirects to a dashboard still
// proves the service is serving, and raioz is not the judge of what the
// body should say — the user picked the endpoint precisely because they
// know what "up" means for their service.
func ProbeHTTP(ctx context.Context, url string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, httpProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
