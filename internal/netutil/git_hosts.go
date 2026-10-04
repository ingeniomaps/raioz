package netutil

import (
	"context"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

// gitHostDialTimeout bounds the reachability probe of one git host.
const gitHostDialTimeout = 3 * time.Second

// dialGitHost opens and closes a TCP connection. Package var so tests
// decide which hosts answer without touching the network.
var dialGitHost = func(ctx context.Context, hostport string) error {
	conn, err := (&net.Dialer{Timeout: gitHostDialTimeout}).DialContext(ctx, "tcp", hostport)
	if err != nil {
		return err //nolint:wrapcheck // the caller only needs to know it failed
	}
	return conn.Close() //nolint:wrapcheck // same
}

// GitRemoteEndpoint returns the host:port a git remote is fetched from:
// 443 or 80 for an http(s) URL, 22 (or the stated port) for ssh in either
// spelling. ok is false for a local path or anything else with no host.
func GitRemoteEndpoint(repo string) (hostport string, ok bool) {
	repo = strings.TrimSpace(repo)
	if strings.Contains(repo, "://") {
		u, err := url.Parse(repo)
		if err != nil || u.Hostname() == "" {
			return "", false
		}
		port := u.Port()
		if port == "" {
			switch u.Scheme {
			case "https":
				port = "443"
			case "http":
				port = "80"
			case "ssh", "git+ssh":
				port = "22"
			case "git":
				port = "9418"
			default:
				return "", false
			}
		}
		return net.JoinHostPort(u.Hostname(), port), true
	}
	// scp-like: [user@]host:path — a colon before any slash.
	colon := strings.IndexByte(repo, ':')
	if colon <= 0 || strings.ContainsAny(repo[:colon], `/\`) {
		return "", false
	}
	host := repo[:colon]
	if at := strings.LastIndexByte(host, '@'); at >= 0 {
		host = host[at+1:]
	}
	if host == "" || len(host) == 1 { // a lone letter is a Windows drive
		return "", false
	}
	return net.JoinHostPort(host, "22"), true
}

// UnreachableGitHosts returns the hosts of the given git remotes that do
// not accept a connection, each once, sorted. Remotes with no host (local
// paths) are skipped. It asks the hosts the project actually clones from:
// an unrelated site being slow says nothing about them, and a project
// that clones nothing has nothing to ask.
func UnreachableGitHosts(ctx context.Context, repos []string) []string {
	seen := map[string]bool{}
	var down []string
	for _, repo := range repos {
		endpoint, ok := GitRemoteEndpoint(repo)
		if !ok || seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		if err := dialGitHost(ctx, endpoint); err != nil {
			host, _, _ := net.SplitHostPort(endpoint)
			down = append(down, host)
		}
	}
	sort.Strings(down)
	return down
}
