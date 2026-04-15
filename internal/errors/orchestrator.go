package errors

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
	ErrCodePreHookFailed  ErrorCode = "PRE_HOOK_FAILED"
	ErrCodePostHookFailed ErrorCode = "POST_HOOK_FAILED"
)

// RuntimeNotDetected creates an error when raioz can't determine how to run a service.
func RuntimeNotDetected(serviceName, path string) *RaiozError {
	return New(ErrCodeRuntimeNotDetected,
		"Cannot detect how to run service '"+serviceName+"'",
	).WithContext("service", serviceName).
		WithContext("path", path).
		WithSuggestion(
			"Raioz looks for: docker-compose.yml, Dockerfile, package.json, go.mod, Makefile, pyproject.toml, or Cargo.toml.\n" +
				"  Add one of these to " + path + ", or check that the path exists and is accessible.",
		)
}

// RuntimeNotInstalled creates an error when a required runtime tool is missing.
func RuntimeNotInstalled(runtime, command string) *RaiozError {
	return New(ErrCodeRuntimeNotInstalled,
		"Runtime '"+runtime+"' requires '"+command+"' which is not installed",
	).WithContext("runtime", runtime).
		WithContext("command", command).
		WithSuggestion(
			"Install " + command + " and make sure it's in your PATH.\n" +
				"  Run 'raioz doctor' to check all requirements.",
		)
}

// ServiceStartFailed creates an error when a service fails to start.
func ServiceStartFailed(serviceName, runtime string, err error) *RaiozError {
	suggestions := map[string]string{
		"compose": "Check the service's docker-compose.yml for errors. " +
			"Try running 'docker compose up' directly in the service directory.",
		"dockerfile": "Check the Dockerfile for build errors. " +
			"Try running 'docker build .' in the service directory.",
		"npm": "Check package.json scripts. " +
			"Try running 'npm run dev' directly in the service directory.",
		"go": "Check for compilation errors. " +
			"Try running 'go run .' directly in the service directory.",
		"make": "Check the Makefile targets. " +
			"Try running 'make dev' directly in the service directory.",
		"python": "Check for missing dependencies. Try running the start command directly in the service directory.",
		"rust":   "Check for compilation errors. Try running 'cargo run' directly in the service directory.",
		"image":  "Check that the Docker image exists and can be pulled. Try 'docker pull <image>' manually.",
	}

	suggestion := suggestions[runtime]
	if suggestion == "" {
		suggestion = "Check the service logs for details. Try starting the service manually."
	}

	return New(ErrCodeServiceStartFailed,
		"Failed to start service '"+serviceName+"' ("+runtime+")",
	).WithContext("service", serviceName).
		WithContext("runtime", runtime).
		WithError(err).
		WithSuggestion(suggestion)
}

// DependencyStartFailed creates an error when a dependency fails to start.
func DependencyStartFailed(name, image string, err error) *RaiozError {
	return New(ErrCodeDepStartFailed,
		"Failed to start dependency '"+name+"' ("+image+")",
	).WithContext("dependency", name).
		WithContext("image", image).
		WithError(err).
		WithSuggestion(
			"Check that Docker is running: docker info\n" +
				"  Check that the image exists: docker pull " + image + "\n" +
				"  Check for port conflicts: raioz ports",
		)
}

// ProxyStartFailed creates an error when Caddy proxy fails to start.
func ProxyStartFailed(err error) *RaiozError {
	return New(ErrCodeProxyStartFailed,
		"Failed to start Caddy proxy",
	).WithError(err).
		WithSuggestion(
			"Check that Docker is running and port 80/443 are free.\n" +
				"  Try: docker pull caddy:latest\n" +
				"  Try: raioz proxy stop && raioz up",
		)
}

// PathNotFound creates an error when a service path doesn't exist.
func PathNotFound(serviceName, path string) *RaiozError {
	return New(ErrCodePathNotFound,
		"Path for service '"+serviceName+"' does not exist: "+path,
	).WithContext("service", serviceName).
		WithContext("path", path).
		WithSuggestion(
			"Check that the path is correct in raioz.yaml.\n" +
				"  Paths are relative to the raioz.yaml directory.",
		)
}

// DevSwapFailed creates an error for raioz dev failures.
func DevSwapFailed(name, action string, err error) *RaiozError {
	return New(ErrCodeDevSwapFailed,
		"Failed to "+action+" '"+name+"'",
	).WithContext("dependency", name).
		WithContext("action", action).
		WithError(err).
		WithSuggestion(
			"Check that the local path exists and has a valid project structure.\n" +
				"  Run 'raioz dev --list' to see active dev overrides.\n" +
				"  Run 'raioz dev --reset " + name + "' to revert to the image.",
		)
}

// PreHookFailed creates an error when a pre-hook command fails.
func PreHookFailed(command string, err error) *RaiozError {
	return New(ErrCodePreHookFailed,
		"Pre-hook failed: "+command,
	).WithError(err).
		WithSuggestion(
			"The 'pre' command in raioz.yaml failed. This usually means:\n" +
				"  - You're not logged in to your secrets manager (try the login command first)\n" +
				"  - The script doesn't exist or isn't executable (check permissions)\n" +
				"  - A required tool is missing (check the command works manually)",
		)
}

// YAMLParseFailed creates an error for YAML parsing failures.
func YAMLParseFailed(path string, err error) *RaiozError {
	return New(ErrCodeYAMLParseFailed,
		"Failed to parse "+path,
	).WithError(err).
		WithSuggestion(
			"Check YAML syntax in " + path + ".\n" +
				"  Common issues: wrong indentation, missing colons, tabs instead of spaces.\n" +
				"  Validate with: cat " + path + " | python3 -c 'import yaml,sys; yaml.safe_load(sys.stdin)'",
		)
}
