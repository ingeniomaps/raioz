package cli

import (
	"fmt"
	"sort"
	"strings"

	"raioz/internal/app"
	"raioz/internal/config"
	"raioz/internal/naming"
	"raioz/internal/proxy"

	"github.com/spf13/cobra"
)

// hostsCmd implements `raioz hosts` — print the /etc/hosts line that maps
// every proxy hostname for the current project to the proxy container's IP.
//
// Useful in two flows:
//
//  1. proxy.publish: false workflow — user opted out of binding host
//     80/443 and now needs to write /etc/hosts entries by hand. raioz
//     hosts gives them the line ready to copy.
//  2. Multi-workspace setups where each workspace has its own subnet —
//     `raioz hosts` lets the user (or a script) keep /etc/hosts in sync
//     across machines.
//
// Output is intentionally a single, valid /etc/hosts line so it composes
// well with `sudo tee -a /etc/hosts`, sed scripts, etc.
var hostsCmd = &cobra.Command{
	Use:          "hosts",
	Short:        "Print the /etc/hosts line for this project's proxy",
	Long: `Compute the proxy container IP and the list of proxied hostnames for
the current raioz project, and print them as a single /etc/hosts entry.

Requires that raioz.yaml declares either network.subnet (so raioz can derive
<subnet>.1.1) or proxy.ip explicitly. Without one, there's no deterministic
IP to print.

Typical usage:

  # one-time append:
  sudo raioz hosts | sudo tee -a /etc/hosts

  # diff against current /etc/hosts:
  raioz hosts`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		// Reuse the same yaml-or-json discovery as status/up so the command
		// works no matter what the project has on disk.
		proj := app.ResolveYAMLProject(deps, "")
		var cfgDeps *config.Deps
		if proj != nil {
			cfgDeps = proj.Deps
		} else {
			loaded, _, err := deps.ConfigLoader.LoadDeps(":auto:")
			if err != nil {
				return fmt.Errorf("load raioz config: %w", err)
			}
			cfgDeps = loaded
		}
		if cfgDeps == nil {
			return fmt.Errorf("no raioz config found in current directory")
		}

		// Activate workspace prefix so any naming.* helpers (used by the
		// IP derivation below) see the right value.
		naming.SetPrefix(cfgDeps.Workspace)

		ip, err := resolveProxyIPForHosts(cfgDeps)
		if err != nil {
			return err
		}

		hosts := proxiedHostnamesFromConfig(cfgDeps)
		if len(hosts) == 0 {
			return fmt.Errorf("no proxied hostnames declared in raioz.yaml")
		}

		fmt.Printf("%s  %s  # raioz:%s\n",
			ip, strings.Join(hosts, " "), workspaceLabel(cfgDeps))
		return nil
	},
}

// resolveProxyIPForHosts mirrors proxy.Manager's resolution rules without
// requiring a live Manager instance. Precedence: explicit proxy.ip >
// derived <subnet>.1.1. Errors when neither is set since the whole point of
// this command is producing a stable IP.
func resolveProxyIPForHosts(deps *config.Deps) (string, error) {
	if deps.ProxyConfig != nil && deps.ProxyConfig.IP != "" {
		if err := proxy.ValidateProxyIP(deps.ProxyConfig.IP, deps.Network.GetSubnet()); err != nil {
			return "", err
		}
		return deps.ProxyConfig.IP, nil
	}
	if ip := proxy.DefaultProxyIP(deps.Network.GetSubnet()); ip != "" {
		return ip, nil
	}
	return "", fmt.Errorf(
		"cannot derive proxy IP — declare network.subnet (raioz uses <subnet>.1.1) " +
			"or proxy.ip in raioz.yaml")
}

// proxiedHostnamesFromConfig replicates the orchestrator's filter so the
// hosts command shows exactly the entries the proxy will actually serve.
// Services always count; deps only when the image isn't on the binary
// protocol blocklist or the user opted in via routing:.
func proxiedHostnamesFromConfig(deps *config.Deps) []string {
	domain := "localhost"
	if deps.ProxyConfig != nil && deps.ProxyConfig.Domain != "" {
		domain = deps.ProxyConfig.Domain
	}

	var hosts []string
	for name, svc := range deps.Services {
		host := name
		if svc.Hostname != "" {
			host = svc.Hostname
		}
		hosts = append(hosts, host+"."+domain)
	}
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		if entry.Inline.Routing == nil && proxy.IsNonHTTPImage(entry.Inline.Image) {
			continue
		}
		hosts = append(hosts, name+"."+domain)
	}
	sort.Strings(hosts)
	return hosts
}

// workspaceLabel returns a stable identifier for the trailing comment so
// users / scripts can find raioz-managed hosts entries. Falls back to the
// project name when there's no workspace.
func workspaceLabel(deps *config.Deps) string {
	if deps.Workspace != "" {
		return deps.Workspace
	}
	return deps.Project.Name
}
