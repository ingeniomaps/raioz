package detect

// Runtime represents the detected runtime/tool type of a service.
type Runtime string

const (
	RuntimeCompose    Runtime = "compose"
	RuntimeDockerfile Runtime = "dockerfile"
	RuntimeNPM        Runtime = "npm"
	RuntimeGo         Runtime = "go"
	RuntimeMake       Runtime = "make"
	RuntimePython     Runtime = "python"
	RuntimeRust       Runtime = "rust"
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

// DetectResult holds everything Raioz learned about a service's directory.
type DetectResult struct {
	Runtime      Runtime  // Primary detected runtime
	ComposeFile  string   // Path to docker-compose.yml if found
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
