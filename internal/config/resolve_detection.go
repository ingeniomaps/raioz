package config

import (
	"raioz/internal/detect"
	"raioz/internal/domain/models"
)

// ResolveServiceDetection returns a DetectResult for a service, honoring
// explicit overrides declared in raioz.yaml before falling back to directory
// auto-detection.
//
// Precedence:
//  1. svc.Source.ComposeFiles (yaml: `compose: [...]`) -> RuntimeCompose with
//     exactly those files, in the order given.
//  2. svc.Source.Command      (yaml: `command: ...`)  -> RuntimeMake (generic
//     host exec) with StartCommand/DevCommand set to the user command.
//  3. Fallback: detect.Detect(path) — scan the directory for compose/
//     Dockerfile/etc.
//  4. `svc.Source.Runtime` (yaml: `runtime: <enum>`) overrides
//     the Runtime classification AFTER auto-detection so the user gets
//     correct StartCommand discovery from the dir scan but raioz routes
//     the service through the runner they declared. Useful for repos
//     with multiple manifests (Go service + prod Dockerfile, Python
//     service + docker-compose.yml). Validated against
//     models.AllRuntimes() at config load — invalid values fail before
//     reaching this helper.
//
// Shared between `raioz up`, `raioz check`, `raioz status`, and `raioz down`
// so the runtime classification is consistent across commands.
func ResolveServiceDetection(svc Service, path string) models.DetectResult {
	result := resolveServiceDetectionBase(svc, path)
	// User-declared runtime override. Empty = no override.
	if svc.Source.Runtime != "" {
		declared := models.Runtime(svc.Source.Runtime)
		// When the override switches the runtime AND we fell through to
		// auto-detect (no compose/command override above), StartCommand /
		// DevCommand were inferred for the ORIGINAL runtime — e.g. a
		// `docker build && docker run` from a Dockerfile when the user
		// really meant `runtime: npm`. Clear and re-infer so the override
		// actually honors the user's intent. The compose- and
		// command-override paths already set commands explicitly and stay
		// untouched.
		if declared != result.Runtime && !overrideSuppliedCommands(svc) {
			result.StartCommand = ""
			result.DevCommand = ""
			result.HasHotReload = false
			detect.InferCommandsForRuntime(&result, path, declared)
		}
		result.Runtime = declared
	}
	return result
}

// overrideSuppliedCommands reports whether the user explicitly fixed the
// launch commands via yaml. When true, ResolveServiceDetection must not
// rewrite them on a runtime override.
func overrideSuppliedCommands(svc Service) bool {
	return len(svc.Source.ComposeFiles) > 0 || svc.Source.Command != ""
}

func resolveServiceDetectionBase(svc Service, path string) models.DetectResult {
	// Compose override wins first — user is asking for a specific docker compose setup.
	if len(svc.Source.ComposeFiles) > 0 {
		files := make([]string, len(svc.Source.ComposeFiles))
		copy(files, svc.Source.ComposeFiles)
		return models.DetectResult{
			Runtime:      models.RuntimeCompose,
			ComposeFile:  files[0],
			ComposeFiles: files,
			StartCommand: "docker compose up -d",
			DevCommand:   "docker compose up",
		}
	}

	// Custom command override — route through HostRunner (RuntimeMake is the
	// generic "invoke something on the host" bucket used by the dispatcher).
	if svc.Source.Command != "" {
		return models.DetectResult{
			Runtime:      models.RuntimeMake,
			StartCommand: svc.Source.Command,
			DevCommand:   svc.Source.Command,
		}
	}

	if path == "" {
		return models.DetectResult{Runtime: models.RuntimeUnknown}
	}
	return detect.Detect(path)
}

// ValidateServiceRuntime checks that svc.Source.Runtime (if set)
// names a known runtime. Returns nil for empty (no override) or
// known runtimes. invalid values surface as a load-time
// error instead of silently classifying as `Unknown` at runtime.
func ValidateServiceRuntime(svc Service) error {
	if svc.Source.Runtime == "" {
		return nil
	}
	for _, rt := range models.AllRuntimes() {
		if string(rt) == svc.Source.Runtime {
			return nil
		}
	}
	return &InvalidRuntimeError{Value: svc.Source.Runtime}
}

// InvalidRuntimeError is returned by ValidateServiceRuntime when a
// service declares `runtime: <value>` and the value isn't recognized.
type InvalidRuntimeError struct{ Value string }

func (e *InvalidRuntimeError) Error() string {
	return "unknown runtime '" + e.Value +
		"'; expected one of compose, dockerfile, npm, go, make, python, rust, " +
		"just, task, php, java, dotnet, ruby, elixir, dart, swift, scala, " +
		"clojure, zig, gleam, haskell, deno, bun, image"
}
