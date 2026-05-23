package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/git"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// RemoteRouteWriter abstracts proxy.WriteRemoteProjectRoutes for tests.
// Production callers leave it nil and fall through to the package
// default. Signature mirrors the proxy helper so MetaUpOptions can pass
// it through without rewiring the meta runner.
type RemoteRouteWriter func(workspace, project, domain, tlsMode string, routes []interfaces.ProxyRoute) error

// MetaCloner abstracts git.EnsureRepo so tests can inject a fake without
// reaching for network or process spawn. The signature mirrors
// git.EnsureRepo exactly: callers map MetaProject fields onto the
// SourceConfig and pass the meta BaseDir as the second arg. Production
// callers leave the field nil and fall through to the package default
// (gitEnsureRepoAdapter below), which delegates to git.EnsureRepo.
type MetaCloner func(src models.SourceConfig, baseDir string) error

// bootstrapMeta clones every MetaProject whose Mode is MetaModeClone before
// the consumer / router spawn loop, and materializes route files for
// MetaModeRemote projects (ADR-049). Returns errBootstrapAborted when a
// non-optional sub failed, so the caller short-circuits the meta up. The
// returned MetaSummaryList captures every clone / remote-route attempt
// so the meta lifecycle audit + the user-facing summary line up with the
// regular sub-up entries.
//
// Mode transitions, in-place on cfg.Projects[i]:
//   - Clone OK              → Local
//   - Clone fails + Remote  → Remote (ADR-049 rule 1, clone-fallback path)
//   - Clone fails + Optional → Skip
//   - Clone fails otherwise → returns errBootstrapAborted
//   - Remote OK (route written) → stays Remote (signals runSingle to skip)
//
// Mutating cfg is intentional and scoped: the meta lifecycle owns the
// resolved cfg for the duration of this run, and the mutation makes the
// cascade visible to subsequent phases without threading another state
// parameter through.
func (m *MetaRunner) bootstrapMeta(
	ctx context.Context, cfg *config.MetaConfig,
	cloner MetaCloner, remoteWriter RemoteRouteWriter,
) (MetaSummaryList, error) {
	if cfg == nil {
		return nil, nil
	}
	if cloner == nil {
		cloner = gitEnsureRepoAdapter
	}
	// remoteWriter has no in-package default to keep app/ free of an
	// internal/proxy import (ADR-029). Callers route the default through
	// the cli wiring layer; tests inject a fake. publishRemote surfaces
	// a clear error if it's nil when a remote project is encountered.

	var results MetaSummaryList
	for i := range cfg.Projects {
		p := &cfg.Projects[i]
		switch p.Mode {
		case config.MetaModeClone:
			entry := m.cloneOne(ctx, cfg, *p, cloner)
			results = append(results, entry)
			switch {
			case entry.Err == nil:
				p.Mode = config.MetaModeLocal
			case p.Remote != "":
				// Clone failed but a remote fallback is declared —
				// downgrade Mode and let the Remote branch below
				// publish the route on the same loop iteration. Mark
				// the clone-fail entry as Skipped so MetaSummary.
				// HasFailures doesn't propagate a non-zero exit when
				// the fallback succeeds. The error is still preserved
				// on the entry for log/audit visibility.
				p.Mode = config.MetaModeRemote
				results[len(results)-1].Skipped = true
				output.PrintWarning(
					i18n.T("meta.clone_falls_back_to_remote", p.Name, p.Remote),
				)
			case p.Optional:
				p.Mode = config.MetaModeSkip
			default:
				return results, errBootstrapAborted
			}
		}
		// Remote can be reached two ways: declared directly (load
		// resolved Mode=Remote) or via the clone-fallback branch above
		// (we just mutated Mode to Remote). One write per project.
		if p.Mode == config.MetaModeRemote {
			entry := m.publishRemote(ctx, cfg, p, remoteWriter)
			results = append(results, entry)
			if entry.Err != nil && !p.Optional {
				return results, errBootstrapAborted
			}
			if entry.Err != nil {
				p.Mode = config.MetaModeSkip
			}
		}
	}
	warnIfPureRemote(cfg)
	return results, nil
}

// publishRemote writes the workspace Caddy routes file for a remote-mode
// project. The hostname defaults to filepath.Base(Path) when the user
// didn't override it (ADR-049 rule 3). Domain and TLS mode follow the
// workspace-shared default (`localhost` + mkcert) — these match what
// per-project up flows use, so the union Caddyfile renders consistently.
func (m *MetaRunner) publishRemote(
	ctx context.Context, cfg *config.MetaConfig, p *config.MetaProject,
	writer RemoteRouteWriter,
) MetaSummary {
	start := time.Now()
	stdout := m.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	fmt.Fprintln(stdout, "\n"+i18n.T("meta.banner_remote", p.Name, p.Remote))

	if writer == nil {
		return MetaSummary{
			Project: p.Name, Path: p.Path,
			Err: errors.New("meta: no remote route writer configured"),
		}
	}

	hostname := p.RemoteHostname
	if hostname == "" {
		hostname = filepath.Base(p.Path)
	}
	route := interfaces.ProxyRoute{
		ServiceName: p.Name,
		Hostname:    hostname,
		Target:      p.Remote,
	}
	err := writer(cfg.Workspace, p.Name, "localhost", "mkcert",
		[]interfaces.ProxyRoute{route})
	entry := MetaSummary{Project: p.Name, Path: p.Path, Err: err}
	if err != nil {
		bestEffort := p.Optional
		if bestEffort {
			entry.Skipped = true
			output.PrintWarning(
				i18n.T("meta.remote_optional_failed", p.Name, err),
			)
		}
		_ = audit.LogWithContext(
			ctx,
			audit.EventTypeLifecycle,
			metaRemoteFailureDetails(p, err, bestEffort, time.Since(start)),
			fmt.Sprintf("meta_remote failed: %s", p.Name),
		)
		return entry
	}
	_ = audit.LogWithContext(
		ctx,
		audit.EventTypeLifecycle,
		metaRemoteSuccessDetails(p, hostname, time.Since(start)),
		fmt.Sprintf("meta_remote ok: %s", p.Name),
	)
	return entry
}

// warnIfPureRemote flags the degenerate case where every sub-project is
// in remote or skip mode. Without a single local spawn the workspace
// Caddy never boots, the routes files sit unread, and the remotes don't
// get proxied (ADR-049 rule 4 — "must be brought up by a local sub").
func warnIfPureRemote(cfg *config.MetaConfig) {
	for _, p := range cfg.Projects {
		if p.Mode == config.MetaModeLocal || p.Mode == config.MetaModeClone {
			return
		}
	}
	if cfg.Router != nil {
		return // router project is local-by-construction and boots Caddy
	}
	output.PrintWarning(i18n.T("meta.remote_no_local"))
}

// errBootstrapAborted is the sentinel the Up driver uses to short-circuit
// after a non-optional clone failure. The actual error lives on the last
// entry in the returned summary list; callers surface it from there.
var errBootstrapAborted = errors.New("meta bootstrap aborted")

// cloneOne dispatches a single clone via the configured MetaCloner and
// wraps the result in a MetaSummary + audit event. Banner output mirrors
// printMetaBanner so the user sees the clone phase the same way they see
// `up` / `down` phases.
func (m *MetaRunner) cloneOne(
	ctx context.Context, cfg *config.MetaConfig, p config.MetaProject,
	cloner MetaCloner,
) MetaSummary {
	start := time.Now()
	stdout := m.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	fmt.Fprintln(stdout, "\n"+i18n.T("meta.banner_clone", p.Name, p.Git))

	src := models.SourceConfig{
		Kind:   "git",
		Repo:   p.Git,
		Branch: p.Branch,
		Path:   p.RelPath,
		Auth:   p.Auth,
	}
	err := cloner(src, cfg.BaseDir)
	entry := MetaSummary{Project: p.Name, Path: p.Path, Err: err}

	if err != nil {
		bestEffort := p.Optional
		if bestEffort {
			entry.Skipped = true
			output.PrintWarning(i18n.T("meta.clone_optional_failed", p.Name, err))
		}
		_ = audit.LogWithContext(
			ctx,
			audit.EventTypeLifecycle,
			metaCloneFailureDetails(p, err, bestEffort, time.Since(start)),
			fmt.Sprintf("meta_clone failed: %s", p.Name),
		)
		return entry
	}

	_ = audit.LogWithContext(
		ctx,
		audit.EventTypeLifecycle,
		metaCloneSuccessDetails(p, time.Since(start)),
		fmt.Sprintf("meta_clone ok: %s", p.Name),
	)
	return entry
}

// metaCloneSuccessDetails / metaCloneFailureDetails are the audit-event
// payload shapes for the bootstrap phase. Kept close to the call sites so
// adding a field is one place, not three.
func metaCloneSuccessDetails(p config.MetaProject, dur time.Duration) map[string]any {
	return map[string]any{
		"sub_command": "clone",
		"project":     p.Name,
		"path":        p.Path,
		"git":         p.Git,
		"branch":      p.Branch,
		"duration_ms": dur.Milliseconds(),
	}
}

func metaCloneFailureDetails(
	p config.MetaProject, err error, bestEffort bool, dur time.Duration,
) map[string]any {
	d := metaCloneSuccessDetails(p, dur)
	d["error"] = err.Error()
	if bestEffort {
		d["best_effort"] = true
	}
	return d
}

// applyForceRemote mutates cfg.Projects in-place to set Mode=Remote on
// every entry named in names. Returns an error when a name doesn't
// match any project — better to surface a typo at flag-parse time than
// to silently take the cascade default.
//
// Per ADR-049 rule 2, force-remote only applies to entries that declare
// a `remote:` URL — forcing remote on a project without an upstream
// would have nothing to route to. The error message tells the user
// exactly which entry is missing the URL.
func applyForceRemote(cfg *config.MetaConfig, names []string) error {
	if len(names) == 0 || cfg == nil {
		return nil
	}
	byName := map[string]*config.MetaProject{}
	for i := range cfg.Projects {
		byName[cfg.Projects[i].Name] = &cfg.Projects[i]
	}
	for _, n := range names {
		if n == "" {
			continue
		}
		p, ok := byName[n]
		if !ok {
			return fmt.Errorf("--force-remote: unknown project %q", n)
		}
		if p.Remote == "" {
			return fmt.Errorf(
				"--force-remote: project %q has no remote: URL declared",
				n,
			)
		}
		p.Mode = config.MetaModeRemote
	}
	return nil
}

// gitEnsureRepoAdapter is the default production cloner. Wraps
// git.EnsureRepo so MetaCloner stays decoupled from the git package
// signature (and so tests don't have to import git just to override).
func gitEnsureRepoAdapter(src models.SourceConfig, baseDir string) error {
	return git.EnsureRepo(src, baseDir)
}

// metaRemoteSuccessDetails / metaRemoteFailureDetails are the audit-event
// payload shapes for the remote-route phase (ADR-049). Same shape as
// metaCloneSuccessDetails so downstream tooling can pivot on
// sub_command and treat them uniformly.
func metaRemoteSuccessDetails(p *config.MetaProject, hostname string, dur time.Duration) map[string]any {
	return map[string]any{
		"sub_command": "remote",
		"project":     p.Name,
		"path":        p.Path,
		"remote":      p.Remote,
		"hostname":    hostname,
		"duration_ms": dur.Milliseconds(),
	}
}

func metaRemoteFailureDetails(
	p *config.MetaProject, err error, bestEffort bool, dur time.Duration,
) map[string]any {
	d := metaRemoteSuccessDetails(p, p.RemoteHostname, dur)
	d["error"] = err.Error()
	if bestEffort {
		d["best_effort"] = true
	}
	return d
}
