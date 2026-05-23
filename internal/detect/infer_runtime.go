package detect

import (
	"path/filepath"

	"raioz/internal/domain/models"
)

// InferCommandsForRuntime fills StartCommand/DevCommand/HasHotReload (and
// when discoverable, Port) on result for the given runtime, using path as
// the source-of-truth directory. It does NOT touch result.Runtime — callers
// own that.
//
// When the declared runtime can't be served by the path (e.g.
// `runtime: npm` but no package.json), the helper leaves commands at their
// runtime's literal fallback (e.g. `npm start`). Downstream runners then
// surface a clear "missing manifest" error to the user.
//
// Runtime values not handled here (image, unknown, compose) leave commands
// untouched — compose flows through the explicit `compose:` override
// upstream, image deps don't go through this path, and unknown shouldn't
// be reachable after ValidateServiceRuntime.
func InferCommandsForRuntime(result *models.DetectResult, path string, rt models.Runtime) {
	if result == nil {
		return
	}

	switch rt {
	case models.RuntimeDockerfile:
		inferFromDockerfile(result, path)

	case models.RuntimeNPM:
		inferFromPackageJSON(result, filepath.Join(path, "package.json"))
		applyNodeLockfilePreference(result, path)

	case models.RuntimeGo:
		result.StartCommand = "go run ."
		result.DevCommand = "go run ."
		if fileExists(filepath.Join(path, ".air.toml")) ||
			fileExists(filepath.Join(path, ".air.conf")) {
			result.DevCommand = "air"
			result.HasHotReload = true
		}

	case models.RuntimeMake:
		inferFromMakefile(result, filepath.Join(path, "Makefile"))

	case models.RuntimeJust:
		result.StartCommand = "just dev"
		result.DevCommand = "just dev"

	case models.RuntimeTask:
		result.StartCommand = "task dev"
		result.DevCommand = "task dev"

	case models.RuntimePython:
		result.StartCommand = "python -m flask run"

	case models.RuntimeRust:
		result.StartCommand = "cargo run"

	case models.RuntimePHP:
		result.StartCommand = "php -S 0.0.0.0:8000 -t public"
		if result.Port == 0 {
			result.Port = 8000
		}
		if fileExists(filepath.Join(path, "artisan")) {
			result.StartCommand = "php artisan serve --host=0.0.0.0"
		}

	case models.RuntimeJava:
		result.StartCommand = "./mvnw spring-boot:run"
		if fileExists(filepath.Join(path, "gradlew")) {
			result.StartCommand = "./gradlew bootRun"
		}
		if result.Port == 0 {
			result.Port = 8080
		}

	case models.RuntimeDotnet:
		result.StartCommand = "dotnet watch run"
		result.DevCommand = "dotnet watch run"
		result.HasHotReload = true
		if result.Port == 0 {
			result.Port = 5000
		}

	case models.RuntimeRuby:
		result.StartCommand = "bundle exec ruby app.rb"
		if fileExists(filepath.Join(path, "bin", "rails")) ||
			fileExists(filepath.Join(path, "config", "routes.rb")) {
			result.StartCommand = "bundle exec rails server"
			result.DevCommand = "bundle exec rails server"
		}
		if result.Port == 0 {
			result.Port = 3000
		}

	case models.RuntimeElixir:
		result.StartCommand = "mix phx.server"
		result.DevCommand = "mix phx.server"
		result.HasHotReload = true
		if result.Port == 0 {
			result.Port = 4000
		}

	case models.RuntimeDart:
		result.StartCommand = "dart run"
		if result.Port == 0 {
			result.Port = 8080
		}

	case models.RuntimeSwift:
		result.StartCommand = "swift run"
		if result.Port == 0 {
			result.Port = 8080
		}

	case models.RuntimeScala:
		result.StartCommand = "sbt run"
		if result.Port == 0 {
			result.Port = 9000
		}

	case models.RuntimeClojure:
		result.StartCommand = "clj -M:dev"
		if fileExists(filepath.Join(path, "project.clj")) &&
			!fileExists(filepath.Join(path, "deps.edn")) {
			result.StartCommand = "lein run"
		}
		if result.Port == 0 {
			result.Port = 3000
		}

	case models.RuntimeZig:
		result.StartCommand = "zig build run"
		if result.Port == 0 {
			result.Port = 8080
		}

	case models.RuntimeGleam:
		result.StartCommand = "gleam run"
		if result.Port == 0 {
			result.Port = 8080
		}

	case models.RuntimeHaskell:
		result.StartCommand = "cabal run"
		if fileExists(filepath.Join(path, "stack.yaml")) {
			result.StartCommand = "stack run"
		}
		if result.Port == 0 {
			result.Port = 3000
		}

	case models.RuntimeDeno:
		result.StartCommand = "deno task dev"
		result.DevCommand = "deno task dev"
		result.HasHotReload = true
		if result.Port == 0 {
			result.Port = 8000
		}

	case models.RuntimeBun:
		result.StartCommand = "bun run dev"
		result.DevCommand = "bun run dev"
		result.HasHotReload = true
		if result.Port == 0 {
			result.Port = 3000
		}
	}
}

// applyNodeLockfilePreference swaps the npm command prefix for whichever
// package manager the project actually uses, mirroring the lockfile dispatch
// in Detect(). No-op when StartCommand/DevCommand don't begin with "npm".
func applyNodeLockfilePreference(result *models.DetectResult, path string) {
	switch {
	case fileExists(filepath.Join(path, "bun.lockb")) || fileExists(filepath.Join(path, "bunfig.toml")):
		result.StartCommand = replaceNPM(result.StartCommand, "bun")
		result.DevCommand = replaceNPM(result.DevCommand, "bun")
	case fileExists(filepath.Join(path, "pnpm-lock.yaml")):
		result.StartCommand = replaceNPM(result.StartCommand, "pnpm")
		result.DevCommand = replaceNPM(result.DevCommand, "pnpm")
	case fileExists(filepath.Join(path, "yarn.lock")):
		result.StartCommand = replaceNPM(result.StartCommand, "yarn")
		result.DevCommand = replaceNPM(result.DevCommand, "yarn")
	}
}
