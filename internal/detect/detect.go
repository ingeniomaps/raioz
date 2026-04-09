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

	return result
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
