package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/proxy"
)

// validate handles validate.All, workspace permissions, port validation, and dependency conflicts
func (uc *UseCase) validate(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace, dryRun bool) error {
	// Step 1: Preflight checks (Docker, Git, disk space, network) - as documented
	if err := uc.deps.Validator.PreflightCheckWithContext(ctx); err != nil {
		return errors.New(
			errors.ErrCodeDockerNotInstalled,
			i18n.T("error.preflight_failed_detail"),
		).WithSuggestion(
			i18n.T("error.preflight_suggestion_detail"),
		).WithError(err)
	}

	// Step 2: Perform comprehensive configuration validation
	if err := uc.deps.Validator.All(deps); err != nil {
		return err
	}

	// Step 2b: Proxy requirements. If the yaml declares `proxy:` with (default
	// or explicit) `tls: mkcert`, we must have either the mkcert binary or
	// pre-generated certs in ~/.raioz/certs/. Otherwise Caddy would silently
	// degrade to HTTP, which breaks any service that enforces HTTPS (typical
	// for Keycloak, OIDC providers, apps with HSTS). Fail fast with a clear
	// install pointer rather than landing the user in a half-broken state.
	if err := CheckProxyRequirements(deps); err != nil {
		return err
	}

	// Perform migration of legacy services if needed
	if err := uc.deps.Workspace.MigrateLegacyServices(ws, deps); err != nil {
		// Log but don't fail - migration is best-effort
		output.PrintWarning(i18n.T("up.validate.migration_warning", err.Error()))
	}

	// Detect and warn about shared volumes
	serviceVolumes, err := uc.deps.DockerRunner.BuildServiceVolumesMap(deps)
	if err != nil {
		// Log error but don't fail - volume detection is informational
		logging.Warn("Failed to build service volumes map", "error", err)
	} else {
		sharedVolumes := uc.deps.DockerRunner.DetectSharedVolumes(serviceVolumes)
		if len(sharedVolumes) > 0 {
			warningMsg := uc.deps.DockerRunner.FormatSharedVolumesWarning(sharedVolumes)
			output.PrintWarning(warningMsg)
		}
	}

	// Check for dependency conflicts first (before checking permissions)
	shouldContinue, _, err := uc.handleDependencyConflicts(deps, ws, dryRun)
	if err != nil {
		return errors.New(
			errors.ErrCodeDependencyCycle,
			i18n.T("error.dependency_conflicts_failed"),
		).WithSuggestion(
			i18n.T("error.dependency_conflicts_suggestion"),
		).WithError(err)
	}
	if !shouldContinue {
		if dryRun {
			return nil
			// Dry-run mode: just show conflicts, don't fail
		}
		return errors.New(
			errors.ErrCodeDependencyCycle,
			i18n.T("error.dependency_conflicts_aborted"),
		).WithSuggestion(
			i18n.T("error.dependency_conflicts_aborted_suggestion"),
		)
	}

	// Check for missing dependencies
	shouldContinue, _, err = uc.handleDependencyAssist(deps, ws, dryRun)
	if err != nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.dependency_assist_failed"),
		).WithSuggestion(
			i18n.T("error.dependency_assist_suggestion"),
		).WithError(err)
	}
	if !shouldContinue {
		if dryRun {
			return nil
			// Dry-run mode: just show missing dependencies, don't fail
		}
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.dependency_assist_aborted"),
		).WithSuggestion(
			i18n.T("error.dependency_assist_aborted_suggestion"),
		)
	}

	// Check workspace permissions
	if err := uc.deps.Validator.CheckWorkspacePermissions(ws.Root); err != nil {
		return err
	}

	// Check if workspace was created (implicit check)
	output.PrintWorkspaceCreated()

	return nil
}

// CheckProxyRequirements enforces that the environment can actually serve
// HTTPS when the user declared `proxy:` in raioz.yaml. Without this check,
// raioz would silently fall back to HTTP and the user would end up chasing
// a dead end — services like Keycloak reject non-HTTPS frontends and leave
// no obvious trace of why.
//
// Rules:
//   - proxy disabled in yaml           → no check
//   - tls == "letsencrypt"             → no check (Caddy handles it)
//   - tls == "mkcert" (default)        → need mkcert binary OR pre-existing
//                                        certs in ~/.raioz/certs/; otherwise
//                                        hard fail with install pointer.
//
// Exported so `raioz check` can reuse the same logic and surface the same
// error before `raioz up` ever runs — no more "check said green, up failed".
func CheckProxyRequirements(deps *config.Deps) error {
	if !deps.Proxy {
		return nil
	}

	tlsMode := "mkcert" // default — matches proxy.NewManager
	if deps.ProxyConfig != nil && deps.ProxyConfig.TLS != "" {
		tlsMode = deps.ProxyConfig.TLS
	}

	if tlsMode != "mkcert" {
		return nil
	}

	if proxy.HasMkcert() || proxy.HasExistingCerts() {
		return nil
	}

	return errors.New(
		errors.ErrCodeInvalidConfig,
		i18n.T("error.proxy_mkcert_missing"),
	).WithSuggestion(i18n.T("error.proxy_mkcert_missing_suggestion"))
}
