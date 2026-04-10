package detect

import (
	"os"
	"path/filepath"
)

// composeNames are the filenames that indicate a Docker Compose project.
var composeNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

// Detect scans a directory and determines the runtime/tool used by the service.
// Priority: docker-compose > Dockerfile > package.json > go.mod > Makefile > pyproject.toml > Cargo.toml
func Detect(path string) DetectResult {
	result := DetectResult{Runtime: RuntimeUnknown}

	if path == "" {
		return result
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return result
	}

	// Check for Docker Compose files (highest priority)
	for _, name := range composeNames {
		composePath := filepath.Join(path, name)
		if fileExists(composePath) {
			result.Runtime = RuntimeCompose
			result.ComposeFile = composePath
			result.Files = append(result.Files, name)
			inferFromCompose(&result, path)
			return result
		}
	}

	// Check for Dockerfile
	dockerfile := filepath.Join(path, "Dockerfile")
	if fileExists(dockerfile) {
		result.Runtime = RuntimeDockerfile
		result.Dockerfile = dockerfile
		result.Files = append(result.Files, "Dockerfile")
		inferFromDockerfile(&result, path)
		return result
	}

	// Check for package.json (Node.js)
	packageJSON := filepath.Join(path, "package.json")
	if fileExists(packageJSON) {
		result.Runtime = RuntimeNPM
		result.Files = append(result.Files, "package.json")
		inferFromPackageJSON(&result, packageJSON)
		return result
	}

	// Check for go.mod (Go)
	goMod := filepath.Join(path, "go.mod")
	if fileExists(goMod) {
		result.Runtime = RuntimeGo
		result.Files = append(result.Files, "go.mod")
		result.StartCommand = "go run ."
		result.DevCommand = "go run ."
		result.HasHotReload = false
		return result
	}

	// Check for Makefile
	makefile := filepath.Join(path, "Makefile")
	if fileExists(makefile) {
		result.Runtime = RuntimeMake
		result.Files = append(result.Files, "Makefile")
		inferFromMakefile(&result, makefile)
		return result
	}

	// Check for Python (pyproject.toml or requirements.txt)
	pyproject := filepath.Join(path, "pyproject.toml")
	requirements := filepath.Join(path, "requirements.txt")
	if fileExists(pyproject) || fileExists(requirements) {
		result.Runtime = RuntimePython
		if fileExists(pyproject) {
			result.Files = append(result.Files, "pyproject.toml")
		}
		if fileExists(requirements) {
			result.Files = append(result.Files, "requirements.txt")
		}
		result.StartCommand = "python -m flask run"
		result.HasHotReload = false
		return result
	}

	// Check for Rust (Cargo.toml)
	cargoToml := filepath.Join(path, "Cargo.toml")
	if fileExists(cargoToml) {
		result.Runtime = RuntimeRust
		result.Files = append(result.Files, "Cargo.toml")
		result.StartCommand = "cargo run"
		result.HasHotReload = false
		return result
	}

	// Check for PHP (composer.json)
	composerJSON := filepath.Join(path, "composer.json")
	if fileExists(composerJSON) {
		result.Runtime = RuntimePHP
		result.Files = append(result.Files, "composer.json")
		result.StartCommand = "php -S 0.0.0.0:8000 -t public"
		result.Port = 8000
		result.HasHotReload = false
		// Check for artisan (Laravel)
		if fileExists(filepath.Join(path, "artisan")) {
			result.StartCommand = "php artisan serve --host=0.0.0.0"
			result.Port = 8000
		}
		return result
	}

	// Check for Java (pom.xml or build.gradle)
	pomXML := filepath.Join(path, "pom.xml")
	buildGradle := filepath.Join(path, "build.gradle")
	buildGradleKts := filepath.Join(path, "build.gradle.kts")
	if fileExists(pomXML) {
		result.Runtime = RuntimeJava
		result.Files = append(result.Files, "pom.xml")
		result.StartCommand = "./mvnw spring-boot:run"
		if fileExists(filepath.Join(path, "gradlew")) {
			result.StartCommand = "./gradlew bootRun"
		}
		result.Port = 8080
		return result
	}
	if fileExists(buildGradle) || fileExists(buildGradleKts) {
		result.Runtime = RuntimeJava
		if fileExists(buildGradle) {
			result.Files = append(result.Files, "build.gradle")
		} else {
			result.Files = append(result.Files, "build.gradle.kts")
		}
		result.StartCommand = "./gradlew bootRun"
		result.Port = 8080
		return result
	}

	// Check for C# / .NET (*.csproj or *.sln)
	if hasGlob(path, "*.csproj") || hasGlob(path, "*.sln") {
		result.Runtime = RuntimeDotnet
		result.StartCommand = "dotnet watch run"
		result.DevCommand = "dotnet watch run"
		result.HasHotReload = true
		result.Port = 5000
		if hasGlob(path, "*.csproj") {
			result.Files = append(result.Files, "*.csproj")
		} else {
			result.Files = append(result.Files, "*.sln")
		}
		return result
	}

	// Check for Ruby (Gemfile)
	gemfile := filepath.Join(path, "Gemfile")
	if fileExists(gemfile) {
		result.Runtime = RuntimeRuby
		result.Files = append(result.Files, "Gemfile")
		result.StartCommand = "bundle exec ruby app.rb"
		result.Port = 3000
		if fileExists(filepath.Join(path, "bin", "rails")) ||
			fileExists(filepath.Join(path, "config", "routes.rb")) {
			result.StartCommand = "bundle exec rails server"
			result.DevCommand = "bundle exec rails server"
		}
		return result
	}

	// Check for Elixir (mix.exs)
	mixExs := filepath.Join(path, "mix.exs")
	if fileExists(mixExs) {
		result.Runtime = RuntimeElixir
		result.Files = append(result.Files, "mix.exs")
		result.StartCommand = "mix phx.server"
		result.DevCommand = "mix phx.server"
		result.HasHotReload = true
		result.Port = 4000
		return result
	}

	// Check for Dart (pubspec.yaml)
	pubspec := filepath.Join(path, "pubspec.yaml")
	if fileExists(pubspec) {
		result.Runtime = RuntimeDart
		result.Files = append(result.Files, "pubspec.yaml")
		result.StartCommand = "dart run"
		result.Port = 8080
		return result
	}

	// Check for Swift (Package.swift)
	packageSwift := filepath.Join(path, "Package.swift")
	if fileExists(packageSwift) {
		result.Runtime = RuntimeSwift
		result.Files = append(result.Files, "Package.swift")
		result.StartCommand = "swift run"
		result.Port = 8080
		return result
	}

	// Check for Scala (build.sbt)
	buildSbt := filepath.Join(path, "build.sbt")
	if fileExists(buildSbt) {
		result.Runtime = RuntimeScala
		result.Files = append(result.Files, "build.sbt")
		result.StartCommand = "sbt run"
		result.Port = 9000
		return result
	}

	// Check for Clojure (deps.edn or project.clj)
	depsEdn := filepath.Join(path, "deps.edn")
	projectClj := filepath.Join(path, "project.clj")
	if fileExists(depsEdn) {
		result.Runtime = RuntimeClojure
		result.Files = append(result.Files, "deps.edn")
		result.StartCommand = "clj -M:dev"
		result.Port = 3000
		return result
	}
	if fileExists(projectClj) {
		result.Runtime = RuntimeClojure
		result.Files = append(result.Files, "project.clj")
		result.StartCommand = "lein run"
		result.Port = 3000
		return result
	}

	// Check for Zig (build.zig)
	buildZig := filepath.Join(path, "build.zig")
	if fileExists(buildZig) {
		result.Runtime = RuntimeZig
		result.Files = append(result.Files, "build.zig")
		result.StartCommand = "zig build run"
		result.Port = 8080
		return result
	}

	// Check for Gleam (gleam.toml)
	gleamToml := filepath.Join(path, "gleam.toml")
	if fileExists(gleamToml) {
		result.Runtime = RuntimeGleam
		result.Files = append(result.Files, "gleam.toml")
		result.StartCommand = "gleam run"
		result.Port = 8080
		return result
	}

	// Check for Haskell (*.cabal or stack.yaml)
	stackYaml := filepath.Join(path, "stack.yaml")
	if fileExists(stackYaml) {
		result.Runtime = RuntimeHaskell
		result.Files = append(result.Files, "stack.yaml")
		result.StartCommand = "stack run"
		result.Port = 3000
		return result
	}
	if hasGlob(path, "*.cabal") {
		result.Runtime = RuntimeHaskell
		result.Files = append(result.Files, "*.cabal")
		result.StartCommand = "cabal run"
		result.Port = 3000
		return result
	}

	// Check for Deno (deno.json or deno.jsonc)
	denoJSON := filepath.Join(path, "deno.json")
	denoJSONC := filepath.Join(path, "deno.jsonc")
	if fileExists(denoJSON) || fileExists(denoJSONC) {
		result.Runtime = RuntimeDeno
		result.Files = append(result.Files, "deno.json")
		result.StartCommand = "deno task dev"
		result.DevCommand = "deno task dev"
		result.HasHotReload = true
		result.Port = 8000
		return result
	}

	// Check for Bun (bunfig.toml or bun.lockb without package.json already matched)
	bunConfig := filepath.Join(path, "bunfig.toml")
	bunLock := filepath.Join(path, "bun.lockb")
	if fileExists(bunConfig) || fileExists(bunLock) {
		result.Runtime = RuntimeBun
		result.Files = append(result.Files, "bunfig.toml")
		result.StartCommand = "bun run dev"
		result.DevCommand = "bun run dev"
		result.HasHotReload = true
		result.Port = 3000
		return result
	}

	return result
}

// hasGlob returns true if any file matches the glob pattern in the directory.
func hasGlob(dir, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	return len(matches) > 0
}

// ForImage returns a DetectResult for a dependency that is a Docker image.
func ForImage(image string) DetectResult {
	return DetectResult{
		Runtime:      RuntimeImage,
		StartCommand: "docker pull " + image,
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
