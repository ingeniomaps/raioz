package models

// Runtime represents the detected runtime/tool type of a service.
//
// This type lives in the domain layer so that domain/interfaces ports
// (Orchestrator, DiscoveryManager) can reference it without importing
// infrastructure packages. The internal/detect package re-exports it
// via a type alias for ergonomic use at the scanning site. See ADR-009.
type Runtime string

const (
	RuntimeCompose    Runtime = "compose"
	RuntimeDockerfile Runtime = "dockerfile"
	RuntimeNPM        Runtime = "npm"
	RuntimeGo         Runtime = "go"
	RuntimeMake       Runtime = "make"
	RuntimePython     Runtime = "python"
	RuntimeRust       Runtime = "rust"
	RuntimeJust       Runtime = "just"
	RuntimeTask       Runtime = "task"
	RuntimePHP        Runtime = "php"
	RuntimeJava       Runtime = "java"
	RuntimeDotnet     Runtime = "dotnet"
	RuntimeRuby       Runtime = "ruby"
	RuntimeElixir     Runtime = "elixir"
	RuntimeDart       Runtime = "dart"
	RuntimeSwift      Runtime = "swift"
	RuntimeScala      Runtime = "scala"
	RuntimeClojure    Runtime = "clojure"
	RuntimeZig        Runtime = "zig"
	RuntimeGleam      Runtime = "gleam"
	RuntimeHaskell    Runtime = "haskell"
	RuntimeDeno       Runtime = "deno"
	RuntimeBun        Runtime = "bun"
	RuntimeImage      Runtime = "image"
	RuntimeUnknown    Runtime = "unknown"
)

// AllRuntimes returns every declared runtime EXCEPT RuntimeUnknown
// (which is the "we didn't find anything" sentinel, not a real
// runtime callers ever dispatch to).
//
// Used by the orchestrate registry's exhaustiveness test (issue 039 /
// ADR-019) to verify every runtime has a runner. Adding a new Runtime
// constant above must come with adding it here AND registering a
// runner; the test catches the missing-runner case.
func AllRuntimes() []Runtime {
	return []Runtime{
		RuntimeCompose, RuntimeDockerfile, RuntimeNPM, RuntimeGo,
		RuntimeMake, RuntimePython, RuntimeRust, RuntimeJust,
		RuntimeTask, RuntimePHP, RuntimeJava, RuntimeDotnet,
		RuntimeRuby, RuntimeElixir, RuntimeDart, RuntimeSwift,
		RuntimeScala, RuntimeClojure, RuntimeZig, RuntimeGleam,
		RuntimeHaskell, RuntimeDeno, RuntimeBun, RuntimeImage,
	}
}

// DetectResult holds everything raioz learned about a service's directory.
type DetectResult struct {
	Runtime      Runtime // Primary detected runtime
	ComposeFile  string  // First compose file path (backwards compat). Mirrors ComposeFiles[0].
	ComposeFiles []string
	Dockerfile   string   // Path to Dockerfile if found
	StartCommand string   // Inferred start command (e.g., "npm run dev", "go run .")
	DevCommand   string   // Inferred dev command if different from start
	HasHotReload bool     // True if the runtime has built-in hot-reload
	Port         int      // Inferred port (0 if unknown)
	Files        []string // Notable files found during detection
}

// IsDocker returns true if the service runs in a Docker container.
func (r *DetectResult) IsDocker() bool {
	return r.Runtime == RuntimeCompose || r.Runtime == RuntimeDockerfile || r.Runtime == RuntimeImage
}

// IsHost returns true if the service runs directly on the host.
func (r *DetectResult) IsHost() bool {
	return !r.IsDocker() && r.Runtime != RuntimeUnknown
}
