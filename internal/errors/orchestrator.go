package errors

import "raioz/internal/i18n"

// Error codes for the meta-orchestrator flow.
const (
	// Detection errors
	ErrCodeRuntimeNotDetected  ErrorCode = "RUNTIME_NOT_DETECTED"
	ErrCodeRuntimeNotInstalled ErrorCode = "RUNTIME_NOT_INSTALLED"

	// Orchestration errors
	ErrCodeServiceStartFailed ErrorCode = "SERVICE_START_FAILED"
	ErrCodeServiceStopFailed  ErrorCode = "SERVICE_STOP_FAILED"
	ErrCodeDepStartFailed     ErrorCode = "DEPENDENCY_START_FAILED"

	// Proxy errors
	ErrCodeProxyStartFailed ErrorCode = "PROXY_START_FAILED"
	ErrCodeCertsError       ErrorCode = "CERTS_ERROR"

	// Dev swap errors
	ErrCodeDevSwapFailed  ErrorCode = "DEV_SWAP_FAILED"
	ErrCodeNotADependency ErrorCode = "NOT_A_DEPENDENCY"

	// Config errors for YAML
	ErrCodeYAMLParseFailed ErrorCode = "YAML_PARSE_FAILED"
	ErrCodePathNotFound    ErrorCode = "PATH_NOT_FOUND"

	// Hook errors
	ErrCodePreHookFailed   ErrorCode = "PRE_HOOK_FAILED"
	ErrCodePreUpHookFailed ErrorCode = "PRE_UP_HOOK_FAILED"
	ErrCodePostHookFailed  ErrorCode = "POST_HOOK_FAILED"
)

// RuntimeNotDetected creates an error when raioz can't determine how to run a service.
func RuntimeNotDetected(serviceName, path string) *RaiozError {
	return New(ErrCodeRuntimeNotDetected,
		i18n.T("error.runtime_not_detected", serviceName),
	).WithContext("service", serviceName).
		WithContext("path", path).
		WithSuggestion(i18n.T("error.runtime_not_detected_suggestion", path))
}

// startFailureHints are the runtimes with a hint of their own for a failed
// start; anything else gets the generic one.
var startFailureHints = map[string]bool{
	"compose": true, "dockerfile": true, "npm": true, "go": true, "make": true,
	"command": true, "python": true, "rust": true, "image": true,
}

// ServiceStartFailed creates an error when a service fails to start.
func ServiceStartFailed(serviceName, runtime string, err error) *RaiozError {
	suggestion := i18n.T("error.service_start_hint.default")
	if startFailureHints[runtime] {
		suggestion = i18n.T("error.service_start_hint." + runtime)
	}

	return New(ErrCodeServiceStartFailed,
		i18n.T("error.service_start_failed", serviceName, runtime),
	).WithContext("service", serviceName).
		WithContext("runtime", runtime).
		WithError(err).
		WithSuggestion(suggestion)
}

// DependencyStartFailed creates an error when a dependency fails to start.
// `image` may be empty for deps backed by an external compose file; the
// suggestion shifts accordingly because `docker pull ""` is nonsense in that
// case and the actionable knobs are different (compose file, networks, env).
func DependencyStartFailed(name, image string, err error) *RaiozError {
	title := i18n.T("error.dependency_start_failed", name)
	suggestion := i18n.T("error.dependency_start_hint_compose")
	if image != "" {
		title += " (" + image + ")"
		suggestion = i18n.T("error.dependency_start_hint_image", image)
	}
	e := New(ErrCodeDepStartFailed, title).
		WithContext("dependency", name).
		WithError(err).
		WithSuggestion(suggestion)
	if image != "" {
		e = e.WithContext("image", image)
	}
	return e
}

// PreHookFailed creates an error when a pre-hook command fails.
func PreHookFailed(command string, err error) *RaiozError {
	return New(ErrCodePreHookFailed,
		i18n.T("error.pre_hook_failed", command),
	).WithError(err).
		WithSuggestion(i18n.T("error.pre_hook_failed_suggestion"))
}

// PreUpHookFailed creates an error when the preUp hook fails.
func PreUpHookFailed(command string, err error) *RaiozError {
	return New(ErrCodePreUpHookFailed,
		i18n.T("error.pre_up_hook_failed", command),
	).WithError(err).
		WithSuggestion(i18n.T("error.pre_up_hook_failed_suggestion"))
}
